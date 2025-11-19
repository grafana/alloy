// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! High-level abstractions for working with large object files.

use crate::{AnyError, VirtAddr};

use std::io::Read as _;
use std::{fmt, fs, io, ops, path};

use flate2::read::ZlibDecoder;
use memmap2::{Mmap, MmapMut};
use object::{
    CompressionFormat, Object as _, ObjectSection as _, ObjectSegment as _, ObjectSymbol as _,
};
use zstd::stream::read::Decoder as ZstdDecoder;

/// Length of a GNU build ID.
const BUILD_ID_LEN: usize = 20;

/// Maximum size of a GNU debug link.
const MAX_DEBUG_LINK_LENGTH: usize = 4096;

/// Maximum size of an individual object section to keep in memory.
///
/// All sections where the decompressed representation is larger than this
/// constant are instead read into anonymous temporary files and  `mmap`ed.
const SWAP_THRESH: usize = 16 * 1024 * 1024;

/// Result type shorthand.
pub type Result<T = (), E = Error> = std::result::Result<T, E>;

/// Errors that can occur during object file parsing.
#[non_exhaustive]
#[allow(missing_docs)]
#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("GNU alt link section is malformed")]
    MalformedGnuAltLink,

    #[error("Sections are compressed in an unsupported format")]
    UnsupportedCompressionFormat,

    #[error("Section uses an unsupported relocation encoding")]
    UnsupportedRelocEncoding,

    #[error("Section uses an unsupported relocation kind")]
    UnsupportedRelocKind,

    #[error("Section uses an unsupported relocation target")]
    UnsupportedRelocTarget,

    #[error("Section uses an unsupported relocation size")]
    UnsupportedRelocSize,

    #[error("Relocation offset is out of bounds for the section")]
    OutOfBoundsRelocOffset,

    #[error("Relocation contains an invalid symbol index")]
    BadSymbolIndex,

    #[error("Relocation contain an invalid section index")]
    BadSectionIndex,

    #[error("Object file is too big to be loaded")]
    FileTooBig,

    #[error("IO error")]
    IO(#[from] io::Error),

    #[error(transparent)]
    Other(AnyError),
}

/// Conversion of [`object`] errors into ours, with type erasure.
///
/// We erase the type here to prevent leaking [`object`] library types into our
/// public interface. If code needs to special-case based on particular [`object`]
/// errors, we should instead lift them into custom error variants.
impl From<object::Error> for Error {
    fn from(e: object::Error) -> Self {
        Self::Other(Box::new(e))
    }
}

/// Maps an object file or executable into memory.
///
/// This currently supports ELF and mach-O files. The backing file is `mmap`ed
/// to make reading more efficient. This currently uses the [`object`] library
/// to perform the actual heavy lifting, however this should be considered an
/// implementation detail.
pub struct File(Mmap);

impl File {
    /// Map the file at the given path into memory.
    pub fn load(path: &path::Path) -> Result<Self> {
        Self::load_file(&fs::File::open(path)?)
    }

    /// Map the given file into memory.
    pub fn load_file(file: &fs::File) -> Result<Self> {
        Ok(Self(unsafe { Mmap::map(file)? }))
    }

    /// Parse the header and create a reader.
    pub fn parse(&self) -> Result<Reader> {
        Ok(Reader(object::File::parse(&self.0[..])?))
    }
}

/// Provides read access to the data in an object file.
///
/// Created via [`File::parse`].
pub struct Reader<'obj>(object::File<'obj>);

impl<'obj> Reader<'obj> {
    /// Loads the section with the given name into memory.
    ///
    /// Depending on whether the section is compressed in the input file or not,
    /// this can be an expensive operation. Callers should store and retrieve
    /// the returned instance if it is needed more than once.
    pub fn load_section(&self, name: &[u8]) -> Result<Option<Section<'obj>>> {
        let Some(obj_sec) = self.0.section_by_name_bytes(name) else {
            return Ok(None);
        };

        Section::load_from_obj_section(&obj_sec).map(Some)
    }

    /// Like `[Self::load_section]`, but applies relocations if necessary.
    ///
    /// This currently only supports some basic relocation types that we have
    /// seen being applied to DWARF sections in the wild.
    pub fn load_section_reloc(&self, name: &[u8]) -> Result<Option<Section<'obj>>> {
        let Some(obj_sec) = self.0.section_by_name_bytes(name) else {
            return Ok(None);
        };

        let mut section = Section::load_from_obj_section(&obj_sec)?;

        // Don't apply relocations for executables. For ELF files, this
        // corresponds to `ET_EXEC`. We have previously learned the hard
        // way [1] that non-relocatable executables will sometimes come
        // with relocations that, when applied, will essentially relocate
        // the executable twice.
        //
        // [1]: https://go-review.googlesource.com/c/go/+/327009
        if self.0.kind() == object::ObjectKind::Executable {
            return Ok(Some(section));
        }

        // If there are no relocations for this section, we are done here.
        if obj_sec.relocations().next().is_none() {
            return Ok(Some(section));
        }

        // Make section data mutable so we can apply relocations.
        let section_data = section.data.make_mut()?;

        // Apply relocations.
        for (offset, reloc) in obj_sec.relocations() {
            if reloc.encoding() != object::RelocationEncoding::Generic {
                return Err(Error::UnsupportedRelocEncoding);
            }

            // `a` corresponds to `A` in `RelocationKind` documentation.
            let a = reloc.addend();

            // `p` corresponds to `P` in `RelocationKind` documentation.
            let p = match reloc.kind() {
                object::RelocationKind::Absolute => 0,
                object::RelocationKind::Relative => section.virt_addr.wrapping_add(offset),
                _ => return Err(Error::UnsupportedRelocKind),
            };

            // `s` corresponds to `S` in `RelocationKind` documentation.
            let s = match reloc.target() {
                object::RelocationTarget::Absolute => 0,

                object::RelocationTarget::Symbol(sym_idx) => {
                    let Ok(refd_sym) = self.0.symbol_by_index(sym_idx) else {
                        return Err(Error::BadSymbolIndex);
                    };

                    refd_sym.address()
                }

                object::RelocationTarget::Section(sec_idx) => {
                    let Ok(refd_sec) = self.0.section_by_index(sec_idx) else {
                        return Err(Error::BadSectionIndex);
                    };

                    refd_sec.address()
                }

                _ => return Err(Error::UnsupportedRelocTarget),
            };

            // Calculate relocation byte size via ceil division.
            let reloc_byte_size = (usize::from(reloc.size()) + 7) / 8;

            let Ok(offset) = usize::try_from(offset) else {
                return Err(Error::OutOfBoundsRelocOffset);
            };

            if section_data.len().saturating_sub(offset) < reloc_byte_size {
                return Err(Error::OutOfBoundsRelocOffset);
            }

            // Create slice for the data to be updated with the relocation.
            let reloc_buf = &mut section_data[offset..offset + reloc_byte_size];

            // The implicit addend is the original value at the location that
            // we are relocating. In ELF, this is decided from the section name
            // (`rela` => no implicit addend, `rel` => use implicit addend).
            let implicit_addend = match (reloc.has_implicit_addend(), reloc.size()) {
                (true, 32) => u32::from_le_bytes(reloc_buf.try_into().unwrap()) as u64,
                (true, 64) => u64::from_le_bytes(reloc_buf.try_into().unwrap()),
                (true, _) => return Err(Error::UnsupportedRelocSize),
                (false, _) => 0,
            };

            let relocated = implicit_addend
                .wrapping_add(s)
                .wrapping_add_signed(a)
                .wrapping_sub(p);

            match reloc.size() {
                32 => reloc_buf.copy_from_slice(&(relocated as u32).to_le_bytes()),
                64 => reloc_buf.copy_from_slice(&relocated.to_le_bytes()),
                _ => return Err(Error::UnsupportedRelocSize),
            }
        }

        Ok(Some(section))
    }

    /// Checks whether this file has little-endian byte-order.
    pub fn is_little_endian(&self) -> bool {
        self.0.is_little_endian()
    }

    /// Returns the architecture, or [`None`] if unknown.
    pub fn arch(&self) -> Option<Arch> {
        match self.0.architecture() {
            object::Architecture::Aarch64 => Some(Arch::Aarch64),
            object::Architecture::X86_64 => Some(Arch::X86_64),
            _ => None,
        }
    }

    /// Read the contents of the `.gnu_debugaltlink` section.
    pub fn gnu_debug_alt_link(&self) -> Result<Option<GnuDebugAltLink>> {
        GnuDebugAltLink::load_from_obj(self)
    }

    /// Creates a map of all memory mapped regions of the object file.
    pub fn memory_map<'reader>(&'reader self) -> Result<MemoryMap<'obj>> {
        // For ELF files, `.segments()` iterates over PT_LOAD program headers.
        // Load segments cannot be compressed, so we can always borrow them.
        let mut regions = Vec::new();
        for segment in self.0.segments() {
            regions.push(Section {
                prot: Protection::from_segment_flags(segment.flags()),
                virt_addr: segment.address(),
                virt_size: segment.size(),
                data: SectionData::Borrowed(segment.data()?),
            });
        }

        regions.sort_unstable_by_key(|x| x.virt_addr);

        Ok(MemoryMap(regions))
    }

    /// Find a symbol by name.
    ///
    /// Dynamic symbols are preferred over debug symbols. This currently does
    /// a linear search over all symbols.
    pub fn resolve_symbol(&self, name: &str) -> Option<Symbol<'_>> {
        self.0
            .dynamic_symbols()
            .chain(self.0.symbols())
            .find(|sym| sym.name().map_or(false, |x| x == name))
            .map(|sym| Symbol {
                name: sym.name().expect("validated in `find` step"),
                virt_addr: sym.address(),
                length: sym.size(),
            })
    }

    /// Iterate over function symbols in this executable.
    pub fn function_symbols(&self, source: SymbolSource) -> impl Iterator<Item = Symbol<'_>> {
        let iter = match source {
            SymbolSource::Debug => self.0.symbols(),
            SymbolSource::Dynamic => self.0.dynamic_symbols(),
        };

        iter.filter(|x| x.kind() == object::SymbolKind::Text)
            // Dynamic symbols with addr = 0 are imports. Also, compilers
            // often generate bogus debug symbol records at 0.
            .filter(|x| x.address() != 0)
            .filter(|x| x.size() != 0)
            .filter_map(|x| {
                Some(Symbol {
                    name: x.name().ok()?, // just skip non-utf8 symbols
                    virt_addr: x.address(),
                    length: x.size(),
                })
            })
    }
}

/// Information and raw data of an object file section.
///
/// This doesn't exactly correspond to an ELF section: we also use it to
/// represent memory regions described in program headers.
#[derive(Debug)]
pub struct Section<'obj> {
    virt_addr: VirtAddr,
    virt_size: u64,
    prot: Option<Protection>,
    data: SectionData<'obj>,
}

impl<'obj> Section<'obj> {
    /// Construction from an [`object::Section`].
    fn load_from_obj_section(obj_sec: &object::Section<'obj, '_>) -> Result<Self> {
        Ok(Section {
            virt_addr: obj_sec.address(),
            virt_size: obj_sec.size(),
            prot: None,
            data: SectionData::load_from_obj_sec(obj_sec)?,
        })
    }

    /// Returns the virtual address range of the section.
    pub fn va_range(&self) -> ops::Range<VirtAddr> {
        self.virt_addr..self.virt_addr + self.virt_size
    }

    /// Returns the virtual address of the first byte of this section.
    pub fn virt_addr(&self) -> VirtAddr {
        self.virt_addr
    }

    /// Returns the virtual size of the section.
    ///
    /// Can be larger than the actual data, padding must be assumed to be zeroed.
    pub fn virt_size(&self) -> u64 {
        self.virt_size
    }

    /// Returns the protection flags for this memory region.
    ///
    /// This is only available for sections from [`MemoryMap::iter`].
    pub fn protection(&self) -> Option<Protection> {
        self.prot
    }

    /// Tries borrowing the section data as a slice with `'obj` lifetime.
    ///
    /// This only works for sections where the data is not owned by the
    /// section thus has the larger `'obj` lifetime (instead of "`'self`").
    pub fn as_obj_slice(&self) -> Option<&'obj [u8]> {
        if let SectionData::Borrowed(slice) = self.data {
            Some(slice)
        } else {
            None
        }
    }
}

/// Allow using section objects where slices are expected.
impl<'obj> ops::Deref for Section<'obj> {
    type Target = [u8];

    fn deref(&self) -> &Self::Target {
        match &self.data {
            SectionData::Borrowed(x) => x,
            SectionData::InMemory(x) => &x[..],
            SectionData::Swapped(x) => &x[..],
        }
    }
}

/// Storage for object file sections.
pub enum SectionData<'obj> {
    /// Section was uncompressed in the input file and we simply kept a ref.
    Borrowed(&'obj [u8]),

    /// Section was originally compressed and we decompressed it into memory.
    InMemory(Vec<u8>),

    /// Section was originally compressed and we decompressed it into a
    /// memory-mapped temporary file.
    Swapped(MmapMut),
}

impl<'obj> SectionData<'obj> {
    /// Create [`Self::InMemory`] variant from a reader.
    fn read_into_memory(final_size: usize, mut reader: impl io::Read) -> Result<Self> {
        let mut mem_buf = Vec::with_capacity(final_size);
        reader.read_to_end(&mut mem_buf)?;
        Ok(SectionData::InMemory(mem_buf))
    }

    /// Create [`Self::Swapped`] variant from a reader.
    fn read_into_swap(mut reader: impl io::Read) -> Result<Self> {
        let mut file = tempfile::tempfile()?;
        io::copy(&mut reader, &mut file)?;
        let mmap = unsafe { MmapMut::map_mut(&file)? };
        Ok(SectionData::Swapped(mmap))
    }

    /// Creates a variant of the [`SectionData`] enum most appropriate for the
    /// given size.
    ///
    /// Uncompressed sections are handed out as a reference whereas compressed
    /// ones are either decoded into memory or into `mmap`ed temporary files
    /// based on their size.
    fn read_smart(final_size: usize, reader: impl io::Read) -> Result<Self> {
        if final_size >= SWAP_THRESH {
            Self::read_into_swap(reader)
        } else {
            Self::read_into_memory(final_size, reader)
        }
    }

    /// Load the data from the given [`object::Section`].
    fn load_from_obj_sec(sec: &object::Section<'obj, '_>) -> Result<Self> {
        let data = sec.compressed_data()?;

        // Ensure that the file fits into memory.
        let final_size: usize = data
            .uncompressed_size
            .try_into()
            .map_err(|_| Error::FileTooBig)?;

        let decoder: Box<dyn io::Read> = match data.format {
            CompressionFormat::Zlib => Box::new(ZlibDecoder::new(data.data)),
            CompressionFormat::Zstandard => Box::new(ZstdDecoder::new(data.data)?),
            CompressionFormat::None => return Ok(SectionData::Borrowed(data.data)),
            _ => return Err(Error::UnsupportedCompressionFormat),
        };

        // Still here? Compressed section: unpack it.
        let decoder = decoder.take(final_size as u64);
        Self::read_smart(final_size, decoder)
    }

    /// Builds a mutable reference to the section's data (CoW semantics).
    ///
    /// If the data was previously borrowed, the first call will force a copy;
    /// all consecutive calls will re-use the same buffer.
    pub fn make_mut(&mut self) -> Result<&mut [u8]> {
        let borrowed = match self {
            // Fast paths: underlying buffer is writable already.
            SectionData::InMemory(x) => return Ok(&mut x[..]),
            SectionData::Swapped(x) => return Ok(&mut x[..]),

            // Expensive case: we need to copy.
            SectionData::Borrowed(x) => x,
        };

        *self = Self::read_smart(borrowed.len(), borrowed)?;

        self.make_mut()
    }
}

impl<'obj> fmt::Debug for SectionData<'obj> {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let (storage, len) = match self {
            Self::Borrowed(x) => ("borrowed", x.len()),
            Self::InMemory(x) => ("in-memory", x.len()),
            Self::Swapped(x) => ("mmapped", x.len()),
        };

        write!(f, "SectionData([{} bytes, {}])", len, storage)
    }
}

/// Represents a GNU build ID.
#[repr(transparent)]
#[derive(Copy, Clone, PartialEq, Eq, Hash)]
pub struct GnuBuildId(pub [u8; BUILD_ID_LEN]);

impl fmt::Debug for GnuBuildId {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let hex: String = self.0.iter().map(|x| format!("{x:02X}")).collect();
        f.debug_tuple("GnuBuildId").field(&hex).finish()
    }
}

/// Parsed contents of the `.gnu_debugaltlink` section.
#[derive(Debug, Clone)]
pub struct GnuDebugAltLink {
    /// Relative or absolute path to the supplementary debug file.
    ///
    /// May contain non UTF-8 characters, hence represented as raw bytes.
    pub path: Vec<u8>,

    /// GNU build ID for the supplementary debug file.
    pub build_id: GnuBuildId,
}

impl GnuDebugAltLink {
    fn load_from_obj(obj: &Reader<'_>) -> Result<Option<Self>> {
        let Some(sec) = obj.load_section(b".gnu_debugaltlink")? else {
            return Ok(None);
        };

        let Some(end_of_path) = sec.iter().position(|&x| x == 0) else {
            return Err(Error::MalformedGnuAltLink);
        };
        if end_of_path > MAX_DEBUG_LINK_LENGTH {
            return Err(Error::MalformedGnuAltLink);
        }

        let path = sec[..end_of_path].to_owned();

        let build_id = GnuBuildId(
            sec[end_of_path + 1..]
                .try_into()
                .map_err(|_| Error::MalformedGnuAltLink)?,
        );

        Ok(Some(GnuDebugAltLink { build_id, path }))
    }
}

/// Provides quick lookups from virtual addresses to the corresponding object file region.
#[derive(Debug)]
pub struct MemoryMap<'obj>(Vec<Section<'obj>>);

impl<'obj> MemoryMap<'obj> {
    /// Finds the section for the given virtual address.
    pub fn section_for_addr(&self, addr: VirtAddr) -> Option<&Section<'obj>> {
        let idx = match self.0.binary_search_by_key(&addr, |x| x.virt_addr) {
            Ok(idx) => idx,
            Err(idx) => idx.checked_sub(1)?,
        };

        let region = self.0.get(idx)?;

        if region.virt_size > addr - region.virt_addr {
            Some(region)
        } else {
            None
        }
    }

    /// Returns a slice for the data at the given address.
    ///
    /// The returned slice might be shorter than the requested length if the
    /// section's virtual size is larger than the data backing it up. In these
    /// cases the caller can assume that the remaining bytes are zero.
    pub fn slice_for_addr(&self, addr: VirtAddr, length: u64) -> Option<&[u8]> {
        let section = self.section_for_addr(addr)?;
        let offset = addr - section.virt_addr();

        if offset.checked_add(length)? > section.virt_size() {
            // Outside of virtual section range: indicate via `None`.
            return None;
        }

        let start = offset as usize;
        let end = (start + length as usize).min(section.len());

        if start >= end {
            // Within virtual section range, but no actual data present:
            // indicate via empty slice.
            return Some(&[]);
        }

        Some(&section[start..end])
    }

    /// Iterate over all memory regions.
    pub fn iter(&self) -> std::slice::Iter<Section<'obj>> {
        self.0.iter()
    }
}

/// Allows iterating the memory map via `&my_memory_map`.
impl<'map, 'obj> IntoIterator for &'map MemoryMap<'obj> {
    type Item = &'map Section<'obj>;
    type IntoIter = std::slice::Iter<'map, Section<'obj>>;

    fn into_iter(self) -> Self::IntoIter {
        self.iter()
    }
}

/// CPU architecture.
#[non_exhaustive]
#[derive(Debug, Copy, Clone, PartialEq, Eq, Hash)]
pub enum Arch {
    /// `aarch64` aka `arm64`.
    Aarch64,
    /// `x86_64` aka `amd64`.
    X86_64,
}

impl Arch {
    /// Minimum instruction alignment required by architecture.
    pub const fn min_code_align(self) -> u64 {
        match self {
            Arch::Aarch64 => 4,
            Arch::X86_64 => 1,
        }
    }
}

/// Specifies an object symbol source.
#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub enum SymbolSource {
    /// Debug symbol table (`.symtab`).
    Debug,

    /// Dynamic symbol table (`.dynsym`).
    Dynamic,
}

/// Memory access protection flags.
#[derive(Debug, Copy, Clone, PartialEq, Eq, Hash)]
pub struct Protection {
    /// Read permissions.
    pub r: bool,
    /// Write permissions.
    pub w: bool,
    /// Execute permissions.
    pub x: bool,
}

impl Protection {
    fn from_segment_flags(flags: object::SegmentFlags) -> Option<Self> {
        match flags {
            object::SegmentFlags::Elf { p_flags, .. } => Some(Self {
                r: p_flags & object::elf::PF_R != 0,
                w: p_flags & object::elf::PF_W != 0,
                x: p_flags & object::elf::PF_X != 0,
            }),
            object::SegmentFlags::MachO { maxprot, .. } => Some(Self {
                r: maxprot & object::macho::VM_PROT_READ != 0,
                w: maxprot & object::macho::VM_PROT_WRITE != 0,
                x: maxprot & object::macho::VM_PROT_EXECUTE != 0,
            }),
            _ => None,
        }
    }
}

/// Basic executable function symbol.
#[derive(Debug, Clone)]
pub struct Symbol<'a> {
    /// Function name. Might be mangled.
    pub name: &'a str,
    /// Start address of the function.
    pub virt_addr: VirtAddr,
    /// Length of the function.
    pub length: u64,
}

impl Symbol<'_> {
    /// Constructs the address range for the symbol.
    pub fn range(&self) -> ops::Range<VirtAddr> {
        self.virt_addr..self.virt_addr.saturating_add(self.length)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::tests::testdata;

    #[test]
    fn arch() {
        let obj = File::load(&testdata("inline")).unwrap();
        let reader = obj.parse().unwrap();
        assert_eq!(reader.arch(), Some(Arch::Aarch64));
    }

    #[test]
    fn uncompressed_section() {
        let obj = File::load(&testdata("inline")).unwrap();
        let reader = obj.parse().unwrap();
        let section = reader.load_section(b".debug_info").unwrap().unwrap();
        assert!(matches!(section.data, SectionData::Borrowed(_)));
        assert_eq!(section.virt_addr(), 0);
        assert_eq!(section.len(), 0x22c);

        let section = reader.load_section(b".text").unwrap().unwrap();
        assert_eq!(
            &section[..8],
            [0xFD, 0x7B, 0xBF, 0xA9, 0xFD, 0x03, 0x00, 0x91]
        );
    }

    #[test]
    fn compressed_section() {
        for file in ["inline-compressed-dwarf", "inline-compressed-dwarf-zstd"] {
            let obj = File::load(&testdata(file)).unwrap();
            let reader = obj.parse().unwrap();
            let section = reader.load_section(b".debug_info").unwrap().unwrap();
            assert!(matches!(section.data, SectionData::InMemory(_)));
            assert_eq!(section.virt_addr(), 0);
            assert_eq!(section.len(), 0x22c);
        }

        {
            let obj = File::load(&testdata("inline-big-fake-compressed-dwarf")).unwrap();
            let reader = obj.parse().unwrap();
            let section = reader.load_section(b".debug_info").unwrap().unwrap();
            assert!(matches!(section.data, SectionData::Swapped(_)));
            assert_eq!(section.virt_addr(), 0);
            assert_eq!(section.len(), 16 * 4 * 1024 * 1024);
            assert!(section.iter().all(|x| *x == 0x00));
        }
    }

    #[test]
    fn memory_map() {
        let obj = File::load(&testdata("inline")).unwrap();
        let reader = obj.parse().unwrap();
        let mem = reader.memory_map().unwrap();

        for addr in [0, 0x640, 0x650, 0x944 - 1] {
            let load_seg_1 = mem.section_for_addr(addr).unwrap();
            assert_eq!(load_seg_1.virt_addr(), 0);
            assert_eq!(load_seg_1.virt_size(), 0x944);
            assert_eq!(load_seg_1.len(), 0x944);
            assert_eq!(&load_seg_1[0x640..0x644], b"\xFD\x7B\xBF\xA9");
            assert_eq!(mem.slice_for_addr(0x640, 4).unwrap(), b"\xFD\x7B\xBF\xA9");
        }

        assert!(mem.section_for_addr(0x944).is_none());
        assert!(mem.section_for_addr(0x1fdc8 - 1).is_none());

        for addr in [0x1fdc8, 0x1fdc8 + 0x270, 0x1fdc8 + 0x278 - 1] {
            let load_seg_2 = mem.section_for_addr(addr).unwrap();
            assert_eq!(load_seg_2.virt_addr(), 0x1fdc8);
            assert_eq!(load_seg_2.virt_size(), 0x278);
            assert_eq!(load_seg_2.len(), 0x270);
        }

        // check truncation
        assert_eq!(mem.slice_for_addr(0x1fdc8 + 0x26c, 0x8).unwrap().len(), 4);
        assert_eq!(mem.slice_for_addr(0x1fdc8 + 0x270, 0x8).unwrap().len(), 0);
        assert!(mem.slice_for_addr(0x1fdc8 + 0x278, 0x8).is_none());
    }

    #[test]
    fn alt_link() {
        let obj = File::load(&testdata("inline-split-dwarf")).unwrap();
        let reader = obj.parse().unwrap();
        let alt_link = reader.gnu_debug_alt_link().unwrap().unwrap();

        #[rustfmt::skip]
        assert_eq!(
            alt_link.build_id,
            GnuBuildId([
                0x83, 0xFF, 0xD1, 0xE5, 0x5E, 0xB9, 0x9F, 0x9A, 0x41, 0xA0, 
                0x77, 0xAD, 0xBC, 0x95, 0x09, 0x96, 0xBF, 0xB7, 0x93, 0x7F,
            ]),
        );

        assert_eq!(alt_link.path, b"meow");
    }
}
