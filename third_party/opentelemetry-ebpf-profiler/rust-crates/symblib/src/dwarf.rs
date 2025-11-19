// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Abstraction for extracting information from object files with DWARF data.
//!
//! The main type here is [`Sections`], created via [`Sections::load`].

// Compiler complains about using the gimli constants in match patterns.
#![allow(non_upper_case_globals)]

use crate::{debug, objfile, AnyError, VirtAddr};
use fallible_iterator::FallibleIterator;
use gimli::{constants::*, AttributeValue as AV};
use lru::LruCache;
use smallvec::{smallvec, SmallVec};
use std::borrow::Cow;
use std::cell::RefCell;
use std::num::NonZeroU64;
use std::ops::Range;
use std::rc::Rc;
use std::{fmt, iter, mem, slice};

/// Shorthand for the [`gimli`] reader type that we use everywhere.
///
/// Until BE binaries come back into favor we simply hard-code LE at
/// compile time, getting rid of a ton of unnecessary branching.
type R<'dwarf> = gimli::EndianSlice<'dwarf, gimli::LittleEndian>;

/// Maximum number of compilation units to process per object file.
const MAX_COMP_UNITS: usize = 256 * 1024;

/// Maximum depth of an inline function tree.
const MAX_INLINE_DEPTH: usize = 64 * 1024;

/// Maximum size of the LRU cache for decoded units.
const UNIT_CACHE_SIZE: usize = 64;

/// Result type shorthand.
pub type Result<T = (), E = Error> = std::result::Result<T, E>;

/// Errors that can occur during DWARF parsing.
#[non_exhaustive]
#[allow(missing_docs)]
#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("Reader currently doesn't support big endian binaries")]
    BigEndian,

    #[error("DWARF references a supplementary file but none was provided")]
    MissingSupplementaryInfo,

    #[error("Reference points to non-existing unit")]
    BadUnitRef,

    #[error("Reference points to invalid offset within a unit")]
    BadUnitOffset,

    #[error("Reference attribute has unexpected type")]
    BadRefAttrType,

    #[error("Language attribute has unexpected type")]
    BadLangAttrType,

    #[error("Found inline subroutine outside of an enclosing function")]
    InlineSubroutineOutsideFunction,

    #[error("DIE reference chain too long")]
    DieReferenceChainTooLong,

    #[error("Encountered an invalid line table reference")]
    BadLineTableReference,

    #[error("The call-file attribute is not a numeric index")]
    CallFileNotNumeric,

    #[error("The inline tree is too deep")]
    InlineTreeTooDeep,

    #[error("The input file has too many translation units")]
    UnitLimitExceeded,

    #[error("The supplementary debug file has another supplementary file (unsupported)")]
    RecursiveSupplementaryFile,

    #[error("File contains an invalid file index value `{}`", .0)]
    InvalidFileIndex(u64),

    #[error("File contains an invalid directory index value `{}`", .0)]
    InvalidDirectoryIndex(u64),

    #[error("Line table doesn't increase monotonically")]
    NonMonotonicLineTable,

    #[error("objfile error")]
    Objfile(#[from] objfile::Error),

    #[error(transparent)]
    Other(AnyError),
}

/// Conversion of [`gimli`] errors into ours.
///
/// We erase the type here to prevent leaking [`gimli`] library types into our
/// public interface. If code needs to special-case based on particular gimli
/// errors, we should instead lift them into custom error variants.
impl From<gimli::Error> for Error {
    fn from(e: gimli::Error) -> Self {
        Self::Other(Box::new(e))
    }
}

/// Collection of DWARF sections of an object file.
///
/// Implements lazy decoding of DWARF information from object files. This is
/// currently a higher-level abstraction over the `gimli` library.
pub struct Sections<'obj> {
    main: gimli::DwarfSections<Option<objfile::Section<'obj>>>,
    sup: Option<gimli::DwarfSections<Option<objfile::Section<'obj>>>>,
}

impl<'obj> Sections<'obj> {
    /// Reads the DWARF sections from the given object file.
    pub fn load(obj: &objfile::Reader<'obj>) -> Result<Self> {
        if !obj.is_little_endian() {
            return Err(Error::BigEndian);
        }

        Ok(Self {
            main: gimli::DwarfSections::load(|id| obj.load_section_reloc(id.name().as_bytes()))?,
            sup: None,
        })
    }

    /// Additionally load data from a supplementary object file.
    pub fn load_sup(&mut self, sup: &objfile::Reader<'obj>) -> Result {
        if !sup.is_little_endian() {
            return Err(Error::BigEndian);
        }

        self.sup = Some(gimli::DwarfSections::load(|id| {
            sup.load_section_reloc(id.name().as_bytes())
        })?);

        Ok(())
    }

    /// Collect a list of all translation units in the DWARF sections.
    pub fn units(&self) -> Result<Units<'_>> {
        // Create a borrowing DWARF instance from our owned one.
        fn borrow<'a>(section: &'a Option<objfile::Section<'a>>) -> R<'a> {
            let data = match section {
                Some(x) => x,
                None => &[][..],
            };

            R::new(data, gimli::LittleEndian)
        }

        let mut dwarf = self.main.borrow(borrow);
        if let Some(sup) = &self.sup {
            dwarf.set_sup(sup.borrow(borrow));
        }

        // Collect all units now. We later need this to quickly seek to
        // different units when we encounter cross-unit references.
        let main = collect_unit_headers(&dwarf)?;

        // Do the same for the supplementary file if present.
        let sup = match dwarf.sup() {
            Some(sup) => collect_unit_headers(sup)?,
            None => vec![],
        };

        let cache_size = UNIT_CACHE_SIZE
            .try_into()
            .expect("UNIT_CACHE_SIZE must be >0");

        let unit_cache = RefCell::new(LruCache::new(cache_size));

        Ok(Units {
            dwarf,
            main,
            sup,
            unit_cache,
        })
    }
}

/// Determines the location of a unit.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
enum UnitLocation {
    /// Unit lives in the main DWARF file.
    Main,

    /// Unit lives in the supplementary DWARF file.
    Sup,
}

/// Cached information about a unit.
struct CachedUnitInfo<'dwarf> {
    gimli_unit: gimli::Unit<R<'dwarf>>,
    loc: UnitLocation,
    producer: Option<R<'dwarf>>,
    language: Option<gimli::DwLang>,
}

impl<'dwarf> CachedUnitInfo<'dwarf> {
    fn from_gimli_unit(
        loc: UnitLocation,
        dwarf: &gimli::Dwarf<R<'dwarf>>,
        gimli_unit: gimli::Unit<R<'dwarf>>,
    ) -> Result<Self> {
        let mut die_iter = gimli_unit.entries();

        let mut producer = None;
        let mut language = None;

        if let Some((_, die)) = die_iter.next_dfs()? {
            let mut attrs = die.attrs();
            while let Some(attr) = attrs.next()? {
                match attr.name() {
                    DW_AT_producer => {
                        producer = Some(dwarf.attr_string(&gimli_unit, attr.value())?);
                    }
                    DW_AT_language => {
                        let AV::Language(lang) = attr.value() else {
                            return Err(Error::BadLangAttrType);
                        };

                        language = Some(lang);
                    }
                    _ => {}
                }
            }
        };

        Ok(Self {
            loc,
            gimli_unit,
            producer,
            language,
        })
    }
}

/// List of all translation units in both the main and the supplementary DWARF file.
///
/// Units can contain references to each other and this object serves as an
/// index that permits efficient lookups of other units for these cases.
pub struct Units<'dwarf> {
    /// Borrowed view into the DWARF sections held in the [`Sections`] object.
    dwarf: gimli::Dwarf<R<'dwarf>>,

    /// List of all unit headers in the main DWARF file.
    main: Vec<gimli::UnitHeader<R<'dwarf>>>,

    /// List of all unit headers in the supplementary DWARF file.
    ///
    /// Empty if no supplementary file is present.
    sup: Vec<gimli::UnitHeader<R<'dwarf>>>,

    /// Cache of decoded unit information.
    ///
    /// This significantly reduces the need to constantly re-decode units
    /// when resolving cross-unit references.
    unit_cache: RefCell<
        LruCache<
            /* key:   */ (UnitLocation, gimli::DebugInfoOffset),
            /* value: */ Rc<CachedUnitInfo<'dwarf>>,
        >,
    >,
}

impl<'dwarf> Units<'dwarf> {
    /// Iterate over all units in the main DWARF file.
    pub fn iter<'units>(&'units self) -> UnitIter<'dwarf, 'units> {
        UnitIter {
            all: self,
            iter: self.main.iter(),
        }
    }

    /// Locates the unit that contains the given offset into the `.debug_info` section.
    fn unit_for_offset<'units>(
        &'units self,
        location: UnitLocation,
        offset: gimli::DebugInfoOffset<usize>,
    ) -> Result<Option<Unit<'dwarf, 'units>>> {
        let headers = match location {
            UnitLocation::Main => &self.main,
            UnitLocation::Sup => &self.sup,
        };

        // Use binary search to locate the unit in question.
        let header = match headers.binary_search_by_key(&offset, unit_start) {
            // Exact match.
            Ok(idx) => Some(&headers[idx]),

            // Our unit array is empty.
            Err(0) => None,

            // Either found somewhere within a unit or outside of valid range.
            Err(idx) => {
                let matched = &headers[idx - 1];
                if unit_range(matched).contains(&offset) {
                    Some(matched)
                } else {
                    None
                }
            }
        };

        // Compare with the result of a dumb linear search when compiled in debug mode.
        // Both variants must be equivalent in all cases.
        debug_assert_eq!(
            header.map(|x| x as *const _),
            headers
                .iter()
                .find(|unit| unit_range(unit).contains(&offset))
                .map(|x| x as *const _)
        );

        match header {
            Some(header) => self.unit_for_header(location, header),
            None => Ok(None),
        }
    }

    /// Creates a new `Unit` object for the given unit header.
    fn unit_for_header<'units>(
        &'units self,
        location: UnitLocation,
        header: &'units gimli::UnitHeader<R<'dwarf>>,
    ) -> Result<Option<Unit<'dwarf, 'units>>> {
        let mut cache = self.unit_cache.borrow_mut();
        let cache_key = (location, unit_start(header));

        // Fast path: if we have the decoded unit info cached, just return it.
        if let Some(cached_info) = cache.get(&cache_key) {
            return Ok(Some(Unit {
                all: self,
                unit: cached_info.clone(),
            }));
        }

        // Slow path: decode unit info now and cache it for the next time.
        let dwarf = match location {
            UnitLocation::Main => &self.dwarf,
            UnitLocation::Sup => self.dwarf.sup().ok_or(Error::MissingSupplementaryInfo)?,
        };

        let unit_info = Rc::new(CachedUnitInfo::from_gimli_unit(
            location,
            dwarf,
            dwarf.unit(*header)?,
        )?);

        cache.put(cache_key, unit_info.clone());

        Ok(Some(Unit {
            all: self,
            unit: unit_info,
        }))
    }
}

/// Iterator over the translation units in a DWARF file.
///
/// Created using [`Units::iter`]. Continuing iteration on errors is well-
/// defined and guaranteed not to run into infinite loops: units with bad
/// headers will simply be skipped.
#[derive(Clone)]
pub struct UnitIter<'dwarf, 'units> {
    all: &'units Units<'dwarf>,
    iter: slice::Iter<'units, gimli::UnitHeader<R<'dwarf>>>,
}

impl<'dwarf, 'units> FallibleIterator for UnitIter<'dwarf, 'units> {
    type Item = Unit<'dwarf, 'units>;
    type Error = Error;

    fn next(&mut self) -> Result<Option<Self::Item>> {
        Ok(match self.iter.next() {
            Some(header) => match self.all.unit_for_header(UnitLocation::Main, header) {
                Ok(Some(unit)) => Some(unit),
                Ok(None) => unreachable!(),
                Err(e) => return Err(e),
            },
            None => None,
        })
    }

    fn size_hint(&self) -> (usize, Option<usize>) {
        self.iter.size_hint()
    }
}

/// Programming language this unit was compiled from.
///
/// This currently only maps languages that we need special casing for, mapping
/// all other languages to `[Self::Other]`. The DWARF language attribute also
/// contains the language "version", e.g. C11, but we current simplify this to
/// just the language.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum Lang {
    /// C.
    C,

    /// C++.
    Cxx,

    /// Go.
    Go,

    /// Rust.
    Rust,

    /// Language is known but currently not mapped in this enum type.
    Other,
}

/// References a translation unit in a DWARF section.
#[derive(Clone)]
pub struct Unit<'dwarf, 'units> {
    all: &'units Units<'dwarf>,
    unit: Rc<CachedUnitInfo<'dwarf>>,
}

impl fmt::Debug for Unit<'_, '_> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        // We add the header length here to obtain the offset of the first DIE.
        let hdr_len = self.unit.gimli_unit.header.size_of_header();
        let offs = self.unit.gimli_unit.header.offset().as_debug_info_offset();
        let offs = offs.expect("we don't inspect type sections").0 + hdr_len;

        let name = self.name().unwrap_or(Cow::Borrowed("<unnamed>"));
        let is_sup = self.unit.loc == UnitLocation::Sup;
        let loc = if is_sup { "sup::" } else { "" };
        write!(f, "Unit(\"{name}\" @ {loc}{offs:#08x})")
    }
}

impl<'dwarf, 'units> Unit<'dwarf, 'units> {
    /// Gets the correct DWARF object for the location of this unit.
    fn dwarf(&self) -> &'units gimli::Dwarf<R<'dwarf>> {
        match self.unit.loc {
            UnitLocation::Main => &self.all.dwarf,
            UnitLocation::Sup => self.all.dwarf.sup().expect(
                "bug: units with this location should not be constructed if there's no sup",
            ),
        }
    }

    /// Gets the name of the translation unit.
    pub fn name(&self) -> Option<Cow<'dwarf, str>> {
        self.unit.gimli_unit.name.map(|x| x.to_string_lossy())
    }

    /// Gets the producer (compiler) that created this unit.
    pub fn producer(&self) -> Option<Cow<'dwarf, str>> {
        self.unit.producer.map(|x| x.to_string_lossy())
    }

    /// Gets the programming language this unit was compiled from.
    pub fn language(&self) -> Option<Lang> {
        Some(match self.unit.language? {
            DW_LANG_C | DW_LANG_C89 | DW_LANG_C99 | DW_LANG_C11 | DW_LANG_C17 => Lang::C,
            DW_LANG_C_plus_plus
            | DW_LANG_C_plus_plus_03
            | DW_LANG_C_plus_plus_11
            | DW_LANG_C_plus_plus_14
            | DW_LANG_C_plus_plus_17
            | DW_LANG_C_plus_plus_20 => Lang::Cxx,
            DW_LANG_Rust => Lang::Rust,
            DW_LANG_Go => Lang::Go,
            _ => Lang::Other,
        })
    }

    /// Iterate over the PC ranges of this unit.
    pub fn ranges(&self) -> Result<RangeIter<'dwarf>> {
        Ok(RangeIter(self.dwarf().unit_ranges(&self.unit.gimli_unit)?))
    }

    /// Iterate over subprograms in this translation unit.
    pub fn subprograms<'unit>(&'unit self) -> SubprogramIter<'dwarf, 'units, 'unit> {
        SubprogramIter {
            unit: self,
            die_iter: self.unit.gimli_unit.entries(),
            next_mode: NextItemMode::Any,
        }
    }

    /// Construct an iterator over the line table.
    pub fn line_iter(&self) -> Option<LineIter<'dwarf, 'units>> {
        let line_program = self.unit.gimli_unit.line_program.as_ref()?.clone();
        Some(LineIter {
            unit: self.clone(),
            rows: line_program.rows(),
            state: LineTableIterState::Void,
        })
    }

    /// Resolves the given reference value.
    ///
    /// Currently supports the following [`AV`] types:
    /// - [`AV::UnitRef`]
    /// - [`AV::DebugInfoRef`]
    /// - [`AV::DebugInfoRefSup`]
    fn resolve_ref(
        &self,
        reference: AV<R<'dwarf>>,
    ) -> Result<(Unit<'dwarf, 'units>, gimli::UnitOffset<usize>)> {
        use UnitLocation as UL;

        // Determine file and offset from the attribute value type.
        let (location, offs) = match (self.unit.loc, reference) {
            // Reference within same CU. Simple case, do early exit.
            (_, AV::UnitRef(offs)) => return Ok((self.clone(), offs)),

            // Reference into another CU within this file.
            (location, AV::DebugInfoRef(offs)) => (location, offs),

            // Reference from the main DWARF into a CU in the supplementary file.
            (UL::Main, AV::DebugInfoRefSup(offs)) => (UL::Sup, offs),

            // Reference into the supplementary DWARF while already in the supplementary file.
            (UL::Sup, AV::DebugInfoRefSup(_)) => return Err(Error::RecursiveSupplementaryFile),

            // Any other attribute type is a violation of the specification.
            _ => return Err(Error::BadRefAttrType),
        };

        let Some(refd_unit) = self.all.unit_for_offset(location, offs)? else {
            return Err(Error::BadUnitRef);
        };
        let Some(offs) = offs.to_unit_offset(&refd_unit.unit.gimli_unit.header) else {
            return Err(Error::BadUnitOffset);
        };

        Ok((refd_unit, offs))
    }
}

/// Determines how the next item is selected.
#[derive(Debug, Copy, Clone)]
enum NextItemMode {
    /// Selects whatever DIE is next (depth-first search).
    Any,

    /// Selects the next sibling.
    SkipChildren,
}

/// Iterator over the subprograms in a [`Unit`].
///
/// Created via [`Unit::subprograms`].
#[derive(Clone)]
pub struct SubprogramIter<'dwarf, 'units, 'unit: 'units> {
    unit: &'unit Unit<'dwarf, 'units>,
    die_iter: gimli::EntriesCursor<'unit, 'unit, R<'dwarf>>,
    next_mode: NextItemMode,
}

impl<'dwarf, 'units, 'unit: 'units> FallibleIterator for SubprogramIter<'dwarf, 'units, 'unit> {
    type Item = Subprogram<'dwarf, 'units>;
    type Error = Error;

    fn next(&mut self) -> Result<Option<Self::Item>> {
        // Note: this is not particularly efficient if the DWARF file doesn't
        // have sibling links. We might want to give the `Subprogram` instances
        // a link to this instance and have them send the offset that they
        // ended their iteration at, but that's not exactly trivial to do while
        // also not allocating anything (no `Arc<AtomicU64>`) and still
        // implementing `FallibleIterator` (can't return refs to `self`).

        loop {
            let die = 'found_die: {
                // Reset mode and skip children if we were asked to.
                if let NextItemMode::SkipChildren =
                    mem::replace(&mut self.next_mode, NextItemMode::Any)
                {
                    if let Some(sibling) = self.die_iter.next_sibling()? {
                        break 'found_die sibling;
                    }
                    // If no sibling was found, continue normal DFS.
                }

                match self.die_iter.next_dfs()? {
                    Some(x) => x.1,
                    None => return Ok(None),
                }
            };

            // Skip irrelevant records, but not their children: they might
            // contain records that we do care about.
            if !matches!(die.tag(), DW_TAG_subprogram | DW_TAG_entry_point) {
                continue;
            }

            // For the record types selected above, skip child nodes when this
            // iterator is woken up next time: they are either abstract or dealt
            // with by the `Subprogram` object that we yield here.
            self.next_mode = NextItemMode::SkipChildren;

            // Skip over abstract records (and their children).
            if die_is_abstract(die)? {
                continue;
            }

            // Still here? We have a relevant record that we want to yield.
            return Ok(Some(Subprogram {
                unit: self.unit.clone(),
                info: SubprogramInfo::from_die(0, self.unit.clone(), die)?,
                die_iter: self.die_iter.clone(),
            }));
        }
    }
}

/// Describes a top-level (non-inline) subprogram in the application.
pub struct Subprogram<'dwarf, 'units> {
    unit: Unit<'dwarf, 'units>,
    info: SubprogramInfo<'dwarf, 'units>,
    die_iter: gimli::EntriesCursor<'units, 'units, R<'dwarf>>,
}

impl<'dwarf, 'units> Subprogram<'dwarf, 'units> {
    /// Destructively extracts the [`SubprogramInfo`].
    pub fn into_info(self) -> SubprogramInfo<'dwarf, 'units> {
        self.info
    }

    /// Destructively iterate over both this subroutine and and all inline instances.
    ///
    /// TODO: impl IntoFallibleIterator instead?
    pub fn into_iter(
        self,
    ) -> impl FallibleIterator<Item = SubprogramInfo<'dwarf, 'units>, Error = Error> {
        let inline_iter = self.inline_instances();
        let self_iter = iter::once(Ok(self.into_info()));
        let self_iter = fallible_iterator::convert(self_iter);
        self_iter.chain(inline_iter)
    }

    /// Iterate over functions that have been inlined into this subroutine.
    pub fn inline_instances(&self) -> InlineInstanceIter<'dwarf, 'units> {
        InlineInstanceIter {
            unit: self.unit.clone(),
            die_iter: self.die_iter.clone(),
            tag_stack: smallvec![DW_TAG_subprogram],
            fn_tree_depth: 1,
        }
    }
}

/// Iterator over the inline instances in a [`Subprogram`].
///
/// Created via [`Subprogram::inline_instances`].
pub struct InlineInstanceIter<'dwarf, 'units> {
    unit: Unit<'dwarf, 'units>,
    die_iter: gimli::EntriesCursor<'units, 'units, R<'dwarf>>,
    tag_stack: SmallVec<[DwTag; 64]>,
    fn_tree_depth: u64,
}

impl<'dwarf, 'units> FallibleIterator for InlineInstanceIter<'dwarf, 'units> {
    type Item = SubprogramInfo<'dwarf, 'units>;
    type Error = Error;

    fn next(&mut self) -> Result<Option<Self::Item>> {
        fn tag_affects_depth(x: DwTag) -> bool {
            matches!(x, DW_TAG_subprogram | DW_TAG_inlined_subroutine)
        }

        loop {
            let Some((depth_delta, die)) = self.die_iter.next_dfs()? else {
                return Ok(None);
            };

            // Remove as many levels as we have left behind, plus one since we
            // always push the current element even if it doesn't have children.
            self.fn_tree_depth -= (0..1 - depth_delta)
                .flat_map(|_| self.tag_stack.pop())
                .filter(|&x| tag_affects_depth(x))
                .count() as u64;

            if self.tag_stack.is_empty() {
                break Ok(None);
            }

            if self.tag_stack.len() + 1 > MAX_INLINE_DEPTH {
                return Err(Error::InlineTreeTooDeep);
            }

            self.tag_stack.push(die.tag());

            if !tag_affects_depth(die.tag()) {
                continue;
            }

            self.fn_tree_depth += 1;

            // Skip abstract DIEs -- they are instead caught via references
            // in concrete instances and have relative address ranges that
            // only make sense in that concrete context.
            if die_is_abstract(die)? {
                continue;
            }

            break Ok(Some(SubprogramInfo::from_die(
                self.fn_tree_depth - 1,
                self.unit.clone(),
                die,
            )?));
        }
    }
}

/// Common information for both top-level subroutines and inline instances.
pub struct SubprogramInfo<'dwarf, 'units> {
    fn_tree_depth: u64,
    name: Option<UnitAV<'dwarf, 'units>>,
    link_name: Option<UnitAV<'dwarf, 'units>>,
    call_file: Option<UnitAV<'dwarf, 'units>>,
    call_line: Option<NonZeroU64>,
    die_ranges: Option<gimli::RangeIter<R<'dwarf>>>,
}

impl fmt::Debug for SubprogramInfo<'_, '_> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(f, "SubprogramInfo(name = {:?})", self.name())
    }
}

impl<'dwarf, 'units> SubprogramInfo<'dwarf, 'units> {
    /// Returns the depth of this function in the inline tree.
    ///
    /// The outermost, top-level function has a depth of `0`.
    pub fn depth(&self) -> u64 {
        self.fn_tree_depth
    }

    /// Merge this instance with another, preferring entries from `self`.
    fn merge_from(&mut self, other: Self) {
        if self.name.is_none() {
            self.name = other.name;
        }
        if self.link_name.is_none() {
            self.link_name = other.link_name;
        }
        if self.die_ranges.is_none() {
            self.die_ranges = other.die_ranges;
        }
    }

    /// Extract required information from a DIE.
    fn from_die(
        fn_tree_depth: u64,
        unit: Unit<'dwarf, 'units>,
        die: &gimli::DebuggingInformationEntry<'_, '_, R<'dwarf>>,
    ) -> Result<Self> {
        Self::from_die_impl(fn_tree_depth, unit, die, 0)
    }

    fn from_die_impl(
        fn_tree_depth: u64,
        unit: Unit<'dwarf, 'units>,
        die: &gimli::DebuggingInformationEntry<'_, '_, R<'dwarf>>,
        recursion_depth: usize,
    ) -> Result<Self> {
        // Protect against theoretically-possible infinite reference loops (from abstract origins & specifications).
        // recursion_depth > 2 is very rare. > 3 is yet to be seen. Using > 4 for good measure.
        if recursion_depth > 4 {
            return Err(Error::DieReferenceChainTooLong);
        }

        // Iterate the attributes and pick what we need. This is faster than
        // calling `attr_value` for each attribute since this would internally
        // loop over all attributes for each call.
        let mut name = None;
        let mut link_name = None;
        let mut abstract_origin = None;
        let mut spec = None;
        let mut call_line = None;
        let mut call_file = None;
        let mut attrs = die.attrs();
        while let Some(attr) = attrs.next()? {
            match attr.name() {
                // Reading is expensive: save unit + attribute value and decode lazily.
                DW_AT_name => name = Some(UnitAV(unit.clone(), attr.value())),
                DW_AT_linkage_name => link_name = Some(UnitAV(unit.clone(), attr.value())),
                DW_AT_call_file => call_file = Some(UnitAV(unit.clone(), attr.value())),

                // Reading is cheap: decode immediately.
                DW_AT_call_line => call_line = attr.value().udata_value(),
                DW_AT_abstract_origin => abstract_origin = Some(attr.value()),
                DW_AT_specification => spec = Some(attr.value()),

                // Ignore all other attribute types.
                _ => (),
            }
        }

        let mut info = SubprogramInfo {
            fn_tree_depth,
            name,
            link_name,
            call_file,
            call_line: call_line.and_then(NonZeroU64::new),
            die_ranges: Some(unit.dwarf().die_ranges(&unit.unit.gimli_unit, die)?),
        };

        // If an abstract origin or a specification are present, also recurse into these.
        // `merge_from` prefers properties from `self`, making sure that we use the most
        // concrete information for our current DIE.
        for ref_attr in abstract_origin.into_iter().chain(spec) {
            let (refd_unit, refd_offs) = unit.resolve_ref(ref_attr)?;
            let refd_die = refd_unit.unit.gimli_unit.entry(refd_offs)?;
            info.merge_from(Self::from_die_impl(
                fn_tree_depth,
                refd_unit.clone(),
                &refd_die,
                recursion_depth + 1,
            )?);
        }

        Ok(info)
    }

    /// Determine the name of this function.
    pub fn name(&self) -> Result<Option<Cow<'dwarf, str>>> {
        // Prefer the linkage name if it is present.
        if let Some(UnitAV(ref unit, av)) = self.link_name {
            let x = unit.dwarf().attr_string(&unit.unit.gimli_unit, av)?;
            return Ok(Some(x.to_string_lossy()));
        };

        // Fallback to regular name.
        if let Some(UnitAV(ref unit, av)) = self.name {
            // TODO: must merge with containing namespaces and modules
            let x = unit.dwarf().attr_string(&unit.unit.gimli_unit, av)?;
            let x = x.to_string_lossy();
            return Ok(Some(x));
        }

        Ok(None)
    }

    /// Destructively retrieve the DIE ranges for this routine.
    ///
    /// This consumes the range iterator on the first call, causing the next
    /// [`Self::take_ranges`] call to return [`None`]. This is a quirk that is
    /// required to work around gimli's DIE range iterator not implementing
    /// [`Clone`].
    pub fn take_ranges(&mut self) -> Option<RangeIter<'dwarf>> {
        self.die_ranges.take().map(RangeIter)
    }

    /// Reads the call file for this function, if present.
    pub fn call_file(&self) -> Result<Option<SourceFile<'dwarf>>> {
        let Some(UnitAV(ref unit, av)) = self.call_file else {
            return Ok(None);
        };
        let Some(ref line_program) = unit.unit.gimli_unit.line_program else {
            return Err(Error::BadLineTableReference);
        };
        let AV::FileIndex(file_idx) = av else {
            return Err(Error::CallFileNotNumeric);
        };

        Ok(Some(SourceFile::read_from_linetab(
            unit.clone(),
            line_program.header(),
            SourceFileId(file_idx),
        )?))
    }

    /// Reads the call line for this function, if present.
    pub fn call_line(&self) -> Option<NonZeroU64> {
        self.call_line
    }
}

/// Iterator yielding the PC ranges of a subroutine or inline instance.
///
/// Thin wrapper around the corresponding gimli type to prevent leaking gimli
/// types into the public interface of this module.
pub struct RangeIter<'dwarf>(gimli::RangeIter<R<'dwarf>>);

impl<'dwarf> FallibleIterator for RangeIter<'dwarf> {
    type Item = Range<VirtAddr>;
    type Error = Error;

    fn next(&mut self) -> Result<Option<Self::Item>> {
        Ok(self.0.next()?.map(|x| x.begin..x.end))
    }
}

/// Opaque ID that uniquely identifies a file within a unit.
///
/// TODO: should probably include unit offset to ensure global uniqueness?
#[derive(Debug, Eq, PartialEq, Ord, PartialOrd, Clone, Copy)]
pub struct SourceFileId(u64);

/// File in the DWARF line table.
#[derive(Debug, Clone, Eq, PartialEq)]
pub struct SourceFile<'dwarf> {
    /// Unique ID within a unit.
    pub id: SourceFileId,
    /// Directory component of the source path, if known.
    pub dir: Option<Cow<'dwarf, str>>,
    /// File name component of the source path.
    pub name: Cow<'dwarf, str>,
}

impl<'dwarf> fmt::Display for SourceFile<'dwarf> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let dir = self.dir.as_deref().unwrap_or("<unknown dir>");
        write!(f, "{}/{}", dir, self.name)
    }
}

impl<'dwarf> SourceFile<'dwarf> {
    fn read_from_linetab<'units>(
        unit: Unit<'dwarf, 'units>,
        header: &gimli::LineProgramHeader<R<'dwarf>>,
        id: SourceFileId,
    ) -> Result<Self> {
        let Some(file_entry) = header.file(id.0) else {
            return Err(Error::InvalidFileIndex(id.0));
        };

        let name_av = file_entry.path_name();
        let name_slice = unit.dwarf().attr_string(&unit.unit.gimli_unit, name_av)?;
        let name = name_slice.to_string_lossy();

        let Some(dir_av) = file_entry.directory(header) else {
            // `0` refers to the `DW_AT_compdir` attribute of the CU: if we
            // ended up here, this means that the CU does not have the compdir
            // attribute. I don't think that the DWARF spec permits that, but
            // we've seen it in mainstream executables, so we allow it anyway.
            if file_entry.directory_index() == 0 {
                return Ok(Self {
                    id,
                    dir: None,
                    name,
                });
            }

            return Err(Error::InvalidDirectoryIndex(file_entry.directory_index()));
        };

        let dir_slice = unit.dwarf().attr_string(&unit.unit.gimli_unit, dir_av)?;
        let dir = Some(dir_slice.to_string_lossy());

        Ok(Self { id, dir, name })
    }
}

/// Associates a PC range with a source file and line number.
#[derive(Debug, Clone, Eq, PartialEq)]
pub struct LineTableEntry<'dwarf> {
    /// PC range being described by this line table entry.
    pub rng: Range<VirtAddr>,
    /// Source file that corresponds to this range.
    pub file: SourceFile<'dwarf>,
    /// Line number within the source file, starting at `1`.
    pub line: Option<NonZeroU64>,
}

/// Internal state of [`LineIter`].
#[derive(Debug, Clone, Eq, PartialEq)]
enum LineTableIterState<'dwarf> {
    /// We are in the void between ranges.
    Void,
    /// We are within a line table range.
    InRange(LineTableEntry<'dwarf>),
}

impl<'dwarf> LineTableIterState<'dwarf> {
    /// Constructs a [`Self::InRange`] variant of this enum from a gimli row.
    pub fn from_row<'units>(
        unit: Unit<'dwarf, 'units>,
        header: &gimli::LineProgramHeader<R<'dwarf>>,
        row: &gimli::LineRow,
    ) -> Result<LineTableIterState<'dwarf>> {
        Ok(Self::InRange(LineTableEntry {
            file: SourceFile::read_from_linetab(unit, header, SourceFileId(row.file_index()))?,
            rng: row.address()..row.address(),
            line: row.line(),
        }))
    }

    /// Consume this instance, extracting the current entry.
    ///
    /// # Panics
    ///
    /// If currently in [`Self::Void`] state.
    pub fn unwrap_entry(self) -> LineTableEntry<'dwarf> {
        match self {
            Self::Void => panic!("attempted unwrapping void state as range"),
            Self::InRange(entry) => entry,
        }
    }
}

/// Iterator yielding all line table entries in a unit.
///
/// Constructed via [`Unit::line_iter`].
pub struct LineIter<'dwarf, 'units> {
    unit: Unit<'dwarf, 'units>,
    rows: gimli::LineRows<R<'dwarf>, gimli::IncompleteLineProgram<R<'dwarf>>>,
    state: LineTableIterState<'dwarf>,
}

impl<'dwarf, 'units> FallibleIterator for LineIter<'dwarf, 'units> {
    type Item = LineTableEntry<'dwarf>;
    type Error = Error;

    fn next(&mut self) -> Result<Option<Self::Item>> {
        use LineTableIterState::*;

        loop {
            let Some((header, row)) = self.rows.next_row()? else {
                // Line table exhausted: yield final record if we still have one stashed.
                return Ok(match mem::replace(&mut self.state, Void) {
                    Void => None,
                    InRange(entry) => Some(entry),
                });
            };

            let active = match (&mut self.state, row.end_sequence()) {
                // Sequence ends but we didn't even know that we are in one.
                (Void, true) => continue,

                // New sequence starts here: update state but don't yield anything.
                (Void, false) => {
                    self.state = LineTableIterState::from_row(self.unit.clone(), header, row)?;
                    continue;
                }

                // Sequence is ending and we're moving into the void.
                (state @ InRange { .. }, true) => {
                    let mut old_state = mem::replace(state, Void).unwrap_entry();
                    old_state.rng.end = row.address();
                    return Ok(Some(old_state));
                }

                // Sequence is ongoing: handle outside this match.
                (InRange(entry), false) => entry,
            };

            // DWARF5 [6.2.5]:
            // > Within a sequence, addresses and operation pointers may only increase.
            //
            // While this is clearly not permitted per specification, it is unfortunately
            // quite common in practice, so we have to handle it as graceful as possible.
            if active.rng.end > row.address() {
                debug!(
                    "Non-monotonic line table sequence (jumping from {:#08x} -> {:#08x})",
                    active.rng.end,
                    row.address()
                );

                let new = LineTableIterState::from_row(self.unit.clone(), header, row)?;
                let mut old = mem::replace(&mut self.state, new).unwrap_entry();

                // Since we have no idea where this would actually end we just
                // arbitrarily assume it to be 1 byte long.
                old.rng.end = old.rng.start + 1;

                return Ok(Some(old));
            }

            // Extend range.
            active.rng.end = row.address();

            // Neither line number nor the file changed: done here.
            if active.file.id == SourceFileId(row.file_index()) && active.line == row.line() {
                continue;
            }

            // Sequence is ongoing and something changed: create new record.
            let new_state = LineTableIterState::from_row(self.unit.clone(), header, row)?;
            debug_assert_ne!(&new_state, &self.state);
            let prev_state = mem::replace(&mut self.state, new_state);
            return Ok(Some(prev_state.unwrap_entry()));
        }
    }
}

/// Pair of an attribute value and the corresponding unit.
struct UnitAV<'dwarf, 'units>(Unit<'dwarf, 'units>, AV<R<'dwarf>>);

/// Unwraps the start offset of a unit into a generic [`usize`].
fn unit_start(unit: &gimli::UnitHeader<R<'_>>) -> gimli::DebugInfoOffset {
    unit.offset()
        .as_debug_info_offset()
        .expect("we only collect non-type units")
}

/// Constructs the offset [`Range`] for a unit.
fn unit_range(unit: &gimli::UnitHeader<R<'_>>) -> Range<gimli::DebugInfoOffset> {
    let start = unit_start(unit);
    let end = gimli::DebugInfoOffset(start.0 + unit.length_including_self());
    start..end
}

/// Inspect the given DIE and determine whether it is an abstract record
/// that doesn't actually describe a location in the executable by itself.
fn die_is_abstract(die: &gimli::DebuggingInformationEntry<'_, '_, R<'_>>) -> Result<bool> {
    let mut attrs = die.attrs();
    while let Some(attr) = attrs.next()? {
        match attr.name() {
            // DWARF 5 [3.3.8.1]:
            // > Any subroutine entry that contains a DW_AT_inline attribute
            // > whose value is other than DW_INL_not_inlined is known as an
            // > abstract instance root.
            DW_AT_inline => match attr.value() {
                AV::Inline(DW_INL_not_inlined) => (),
                AV::Inline(_) => return Ok(true),
                _ => (),
            },

            // DWARF 5 [2.13.1]:
            // > A debugging information entry that represents a non-defining or
            // > otherwise incomplete declaration of a program entity has a
            // > DW_AT_declaration attribute, which is a flag.
            DW_AT_declaration => {
                if let AV::Flag(true) = attr.value() {
                    return Ok(true);
                }
            }

            _ => (),
        }
    }

    Ok(false)
}

/// Collect list of all unit headers in a DWARF file.
fn collect_unit_headers<'obj>(
    dwarf: &gimli::Dwarf<R<'obj>>,
) -> Result<Vec<gimli::UnitHeader<R<'obj>>>> {
    let mut unit_iter = dwarf.units().enumerate();
    let mut units = Vec::with_capacity(unit_iter.size_hint().0);

    while let Some((i, unit)) = unit_iter.next()? {
        if i >= MAX_COMP_UNITS {
            return Err(Error::UnitLimitExceeded);
        }

        units.push(unit);
    }

    Ok(units)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{objfile, tests::testdata};

    #[test]
    fn inline() {
        let obj = objfile::File::load(&testdata("inline")).unwrap();
        let obj = obj.parse().unwrap();

        let dwarf = Sections::load(&obj).unwrap();
        let units = dwarf.units().unwrap();

        let mut unit_iter = units.iter();
        let inline_c = unit_iter.next().unwrap().unwrap();
        assert!(unit_iter.next().unwrap().is_none());

        assert_eq!(inline_c.name().unwrap(), "inline.c");
        assert_eq!(
            inline_c.producer().unwrap(),
            "GNU C17 12.2.0 -mlittle-endian -mabi=lp64 -g -O2 -fasynchronous-unwind-tables"
        );
        assert_eq!(inline_c.language().unwrap(), Lang::C);

        // Output `llvm-dwarfdump --debug-line`
        // ====================================
        //
        // include_directories[  0] = "/media/share/Development/prodfiler/libpf-rs/testdata"
        // file_names[  0]:
        //  name: "inline.c"
        //  dir_index: 0
        // file_names[  1]:
        //  name: "inline.c"
        //  dir_index: 0
        // file_names[  2]:
        //  name: "<built-in>"
        //  dir_index: 0
        //
        // Address            Line   Column File   ISA Discriminator Flags
        // ------------------ ------ ------ ------ --- ------------- -------------
        // 0x00000000000007a0      6     18      1   0             0  is_stmt
        // 0x00000000000007a0      7      3      1   0             0  is_stmt
        // 0x00000000000007b0     10     18      1   0             0  is_stmt
        // 0x00000000000007b0     11      3      1   0             0  is_stmt
        // 0x00000000000007b4     14     18      1   0             0  is_stmt
        // 0x00000000000007b4     15      3      1   0             0  is_stmt
        // 0x00000000000007c0     18     18      1   0             0  is_stmt
        // 0x00000000000007c0     19      3      1   0             0  is_stmt
        // 0x00000000000007c4     19      3      1   0             0  is_stmt end_sequence
        // 0x0000000000000640     38     21      1   0             0  is_stmt
        // 0x0000000000000640     39      3      1   0             0  is_stmt
        // 0x0000000000000640     38     21      1   0             0
        // 0x0000000000000648     39      3      1   0             0
        // 0x000000000000064c     40      3      1   0             0  is_stmt
        // 0x000000000000064c     34     12      1   0             0  is_stmt
        // 0x000000000000064c     35      3      1   0             0  is_stmt
        // 0x000000000000064c     30     12      1   0             0  is_stmt
        // 0x000000000000064c     31      3      1   0             0  is_stmt
        // 0x000000000000064c     26     12      1   0             0  is_stmt
        // 0x000000000000064c     27      3      1   0             0  is_stmt
        // 0x000000000000064c     22     12      1   0             0  is_stmt
        // 0x000000000000064c     23      3      1   0             0  is_stmt
        // 0x0000000000000658     41      1      1   0             0
        // 0x0000000000000664     41      1      1   0             0  end_sequence

        let actual_items: Vec<_> = inline_c.line_iter().unwrap().collect().unwrap();

        let expected_items = [
            (0x7a0..0x7a0, 6),
            (0x7a0..0x7b0, 7),
            (0x7b0..0x7b0, 10),
            (0x7b0..0x7b4, 11),
            (0x7b4..0x7b4, 14),
            (0x7b4..0x7c0, 15),
            (0x7c0..0x7c0, 18),
            (0x7c0..0x7c4, 19),
            // end_sequence
            (0x640..0x640, 38),
            (0x640..0x640, 39),
            (0x640..0x648, 38),
            (0x648..0x64c, 39),
            (0x64c..0x64c, 40),
            (0x64c..0x64c, 34),
            (0x64c..0x64c, 35),
            (0x64c..0x64c, 30),
            (0x64c..0x64c, 31),
            (0x64c..0x64c, 26),
            (0x64c..0x64c, 27),
            (0x64c..0x64c, 22),
            (0x64c..0x658, 23),
            (0x658..0x664, 41),
            // end_sequence
        ];

        assert_eq!(actual_items.len(), expected_items.len());

        let inline_c_path = "/media/share/Development/prodfiler/libpf-rs/testdata/inline.c";
        for (actual, expected) in iter::zip(actual_items, expected_items) {
            assert_eq!(actual.rng, expected.0, "range mismatch");
            assert_eq!(actual.line, NonZeroU64::new(expected.1), "line mismatch");
            assert_eq!(actual.file.id, SourceFileId(1), "file ID mismatch");
            assert_eq!(actual.file.to_string(), inline_c_path, "file path mismatch");
        }

        // Output `llvm-dwarfdump --debug-info`
        // ====================================
        //
        // NOTE: output manually filtered and re-indented to aid readability
        //
        // 0x000c: DW_TAG_compile_unit
        //           DW_AT_name                         ("inline.c")
        //           DW_AT_low_pc                       (0x0000000000000000)
        //            DW_AT_ranges	(0x0000000c
        //              [0x00000000000007a0, 0x00000000000007c4)
        //              [0x0000000000000640, 0x0000000000000664))
        // 0x0069:   DW_TAG_subprogram
        //             DW_AT_name                       ("main")
        //             DW_AT_low_pc                     (0x0000000000000640)
        //             DW_AT_high_pc                    (0x0000000000000664)
        // 0x008b:     DW_TAG_inlined_subroutine
        //               DW_AT_abstract_origin          (0x0000013a "a_inline")
        //               DW_AT_low_pc                   (0x000000000000064c)
        //               DW_AT_high_pc                  (0x0000000000000658)
        //               DW_AT_call_file                ("[...]/testdata/inline.c")
        //               DW_AT_call_line                (40)
        // 0x00b0:       DW_TAG_inlined_subroutine
        //                 DW_AT_abstract_origin        (0x00000144 "b_inline")
        //                 DW_AT_low_pc                 (0x000000000000064c)
        //                 DW_AT_high_pc                (0x0000000000000658)
        //                 DW_AT_call_file              ("[...]/testdata/inline.c")
        //                 DW_AT_call_line              (35)
        // 0x00cf:         DW_TAG_inlined_subroutine
        //                   DW_AT_abstract_origin      (0x0000014e "c_inline")
        //                   DW_AT_low_pc               (0x000000000000064c)
        //                   DW_AT_high_pc              (0x0000000000000658)
        //                   DW_AT_call_file            ("[...]/testdata/inline.c")
        //                   DW_AT_call_line            (31)
        // 0x00ee:           DW_TAG_inlined_subroutine
        //                     DW_AT_abstract_origin    (0x00000158 "d_inline")
        //                     DW_AT_low_pc             (0x000000000000064c)
        //                     DW_AT_high_pc            (0x0000000000000658)
        //                     DW_AT_call_file          ("[...]/testdata/inline.c")
        //                     DW_AT_call_line          (27)
        // 0x013a:   DW_TAG_subprogram
        //             DW_AT_name                       ("a_inline")
        //             DW_AT_inline                     (DW_INL_declared_inlined)
        // 0x0144:   DW_TAG_subprogram
        //             DW_AT_name                       ("b_inline")
        //             DW_AT_inline                     (DW_INL_declared_inlined)
        // 0x014e:   DW_TAG_subprogram
        //             DW_AT_name                       ("c_inline")
        //             DW_AT_inline                     (DW_INL_declared_inlined)
        // 0x0158:   DW_TAG_subprogram
        //             DW_AT_name                       ("d_inline")
        //             DW_AT_inline                     (DW_INL_declared_inlined)
        // 0x0162:   DW_TAG_subprogram
        //             DW_AT_name                       ("a")
        //             DW_AT_low_pc                     (0x00000000000007c0)
        //             DW_AT_high_pc                    (0x00000000000007c4)
        // 0x018e:   DW_TAG_subprogram
        //             DW_AT_name                       ("b")
        //             DW_AT_decl_file                  ("[...]/testdata/inline.c")
        //             DW_AT_low_pc                     (0x00000000000007b4)
        //             DW_AT_high_pc                    (0x00000000000007b8)
        // 0x01ba:   DW_TAG_subprogram
        //             DW_AT_name                       ("c")
        //             DW_AT_decl_file                  ("[...]/testdata/inline.c")
        //             DW_AT_low_pc                     (0x00000000000007b0)
        //             DW_AT_high_pc                    (0x00000000000007b4)
        // 0x01e6:   DW_TAG_subprogram
        //             DW_AT_name                       ("d")
        //             DW_AT_decl_file                  ("[...]/testdata/inline.c")
        //             DW_AT_low_pc                     (0x00000000000007a0)
        //             DW_AT_high_pc                    (0x00000000000007ac)
        // 0x0220:   DW_TAG_subprogram
        //             DW_AT_name                       ("__builtin_puts")
        //             DW_AT_declaration                (true)

        assert_eq!(inline_c.name().unwrap(), "inline.c");

        let unit_ranges: Vec<_> = inline_c.ranges().unwrap().collect().unwrap();
        assert_eq!(unit_ranges, [0x7a0..0x7c4, 0x640..0x664]);

        let mut sp_iter = inline_c.subprograms();

        // 0x0069
        let mut main = sp_iter.next().unwrap().unwrap();
        assert_eq!(main.info.depth(), 0);
        assert_eq!(main.info.name().unwrap().unwrap(), "main");
        assert!(main.info.call_line().is_none());
        assert!(main.info.call_file().unwrap().is_none());
        let rng: Vec<_> = main.info.take_ranges().unwrap().collect().unwrap();
        assert_eq!(rng, [0x640..0x664]);

        // 0x008b
        let mut main_ii = main.inline_instances();
        let mut a_inline = main_ii.next().unwrap().unwrap();
        assert_eq!(a_inline.depth(), 1);
        assert_eq!(a_inline.name().unwrap().unwrap(), "a_inline");
        assert_eq!(a_inline.call_line().unwrap().get(), 40);
        assert_eq!(a_inline.call_file().unwrap().unwrap().id, SourceFileId(1));
        assert_eq!(
            a_inline.call_file().unwrap().unwrap().to_string(),
            inline_c_path
        );
        let rng: Vec<_> = a_inline.take_ranges().unwrap().collect().unwrap();
        assert_eq!(rng, [0x64c..0x658]);

        // 0x00b0
        let b_inline = main_ii.next().unwrap().unwrap();
        assert_eq!(b_inline.name().unwrap().unwrap(), "b_inline");
        assert_eq!(b_inline.depth(), 2);

        // 0x00cf
        let b_inline = main_ii.next().unwrap().unwrap();
        assert_eq!(b_inline.name().unwrap().unwrap(), "c_inline");
        assert_eq!(b_inline.depth(), 3);

        // 0x00ee
        let b_inline = main_ii.next().unwrap().unwrap();
        assert_eq!(b_inline.name().unwrap().unwrap(), "d_inline");
        assert_eq!(b_inline.depth(), 4);

        assert!(main_ii.next().unwrap().is_none());

        // 0x013a..=0x0158 should be skipped due to being abstract (`DW_AT_inline`)

        // 0x0162
        let mut a = sp_iter.next().unwrap().unwrap();
        assert_eq!(a.info.depth(), 0);
        assert_eq!(a.info.name().unwrap().unwrap(), "a");
        assert!(a.info.call_line().is_none());
        assert!(a.info.call_file().unwrap().is_none());
        let rng: Vec<_> = a.info.take_ranges().unwrap().collect().unwrap();
        assert_eq!(rng, [0x7c0..0x7c4]);

        // 0x018e
        let b = sp_iter.next().unwrap().unwrap();
        assert_eq!(b.info.depth(), 0);
        assert_eq!(b.info.name().unwrap().unwrap(), "b");

        // 0x01ba
        let c = sp_iter.next().unwrap().unwrap();
        assert_eq!(c.info.depth(), 0);
        assert_eq!(c.info.name().unwrap().unwrap(), "c");

        // 0x01e6
        let d = sp_iter.next().unwrap().unwrap();
        assert_eq!(d.info.depth(), 0);
        assert_eq!(d.info.name().unwrap().unwrap(), "d");

        // 0x0220 should be skipped due to `DW_AT_declaration`

        assert!(sp_iter.next().unwrap().is_none());
    }
}
