// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Translates DWARF data into a range symbfile.
//!
//! Input format: DWARF
//!
//! - Inline instance information is stored in a tree structure
//!   - Function name, inline hierarchy and inline call file + line
//! - Every tree node can have 0..n PC ranges
//! - Source lines and files are stored in a separate per-CU structure (line-table)
//!
//! Output format: symbfile
//!
//! - Flat list of records
//! - Depth is defined via an integer depth field
//! - Combines line table with inline tree
//! - Each record has only one range
//! - Each range's line table can only refer to a single source file
//! - If an inline instance contains instructions generated from multiple
//!   source files, it must be split every time the source file changes

mod rangetree;

use self::rangetree::*;
use crate::symbconv::RangeVisitor;
use crate::{debug, demangle, dwarf, range_overlap, symbfile, AnyError, VirtAddr};
use fallible_iterator::FallibleIterator;
use intervaltree::{Element, IntervalTree};
use smallvec::SmallVec;
use std::cell::RefCell;
use std::num::NonZeroU64;
use std::ops::Range;
use std::rc::Rc;

/// Maximum depth of an inline tree.
///
/// Various parts of this implementation use recursion. If we didn't restrict
/// the inline tree depth, we'd run at risk of running into stack overflows.
const MAX_INLINE_TREE_DEPTH: u64 = 256;

/// Errors that can occur during symbol translation.
#[non_exhaustive]
#[allow(missing_docs)]
#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("Inline tree depth exceeds the maximum of {}", MAX_INLINE_TREE_DEPTH)]
    InlineTreeTooDeep,

    #[error("DWARF error: {}", .0)]
    Dwarf(#[from] dwarf::Error),

    #[error("symbfile error: {}", .0)]
    Symbfile(#[from] symbfile::Error),

    #[error("visitor error: {}", .0)]
    Visitor(#[source] AnyError),

    #[error(transparent)]
    Other(AnyError),
}

/// Result type shorthand.
type Result<T = (), E = Error> = std::result::Result<T, E>;

/// Maps a VA range to a source file and line.
#[derive(Debug, Clone)]
struct IntermediateLineTableEntry {
    pub rng: Range<VirtAddr>,
    pub file: String,
    pub line: u64,
}

/// Intermediate helper struct storing source lines for a VA range.
///
/// Other than in the final symbfile range struct, the line table in this
/// intermediate format can still contain lines from different source files.
#[derive(Debug)]
struct IntermediateRange<'dwarf, 'units> {
    pub info: Rc<dwarf::SubprogramInfo<'dwarf, 'units>>,
    pub line_table: RefCell<Vec<IntermediateLineTableEntry>>,
}

impl<'dwarf, 'units> IntermediateRange<'dwarf, 'units> {
    pub fn new(info: Rc<dwarf::SubprogramInfo<'dwarf, 'units>>) -> Self {
        Self {
            info,
            line_table: RefCell::new(Vec::new()),
        }
    }
}

/// Node format for our intermediate range tree.
///
/// Since DWARF nodes can have multiple ranges associated with them whereas
/// in our tree structure every node represents a single range, we duplicate
/// the node for each DWARF range. Because the multiple ranges of the DWARF
/// root node need to live somewhere as well, a synthetic root node spanning
/// the entire VA space from [`VirtAddr::MIN`] to [`VirtAddr::MAX`] is used.
#[derive(Debug)]
enum Node<'dwarf, 'units> {
    /// Synthetic root without any actual data.
    SynthRoot,

    /// Range with source-line information.
    Range(IntermediateRange<'dwarf, 'units>),
}

/// Constructs a subroutine tree with empty line tables.
fn collect_subroutine_tree<'dwarf, 'units>(
    sub: dwarf::Subprogram<'dwarf, 'units>,
) -> Result<RangeTree<Node<'dwarf, 'units>>> {
    let mut root = RangeTree {
        range: VirtAddr::MIN..VirtAddr::MAX,
        value: Node::SynthRoot,
        children: Vec::with_capacity(8),
    };

    let mut sub_iter = sub.into_iter();
    while let Some(mut i) = sub_iter.next()? {
        let Some(mut ranges) = i.take_ranges() else {
            continue;
        };
        if i.depth() > MAX_INLINE_TREE_DEPTH {
            return Err(Error::InlineTreeTooDeep);
        }

        // Insert one node for each range.
        let i = Rc::new(i);
        while let Some(range) = ranges.next()? {
            if range.start <= 1 {
                continue;
            }

            let Some(container) = root.find_match_at_depth_mut(i.depth(), range.clone()) else {
                continue;
            };

            container.children.push(RangeTree {
                range,
                value: Node::Range(IntermediateRange::new(i.clone())),
                children: Vec::with_capacity(8),
            });
        }
    }

    root.sort();

    Ok(root)
}

/// Checks whether any child has line info.
fn any_child_has_lines(node: &RangeTree<Node<'_, '_>>) -> bool {
    if let Node::Range(imr) = &node.value {
        if !imr.line_table.borrow().is_empty() {
            return true;
        }
    }

    node.children.iter().any(any_child_has_lines)
}

fn process_subroutine(
    unit_line_table: &IntervalTree<VirtAddr, (dwarf::SourceFile<'_>, Option<NonZeroU64>)>,
    sub: dwarf::Subprogram<'_, '_>,
    mut visitor: impl FnMut(symbfile::Range) -> Result,
) -> Result {
    let tree = collect_subroutine_tree(sub)?;

    if tree.children.is_empty() {
        return Ok(());
    }

    // Use the top-level view of the tree to assign line records.
    for node in tree.collect_top_level_ranges() {
        // Skip synthetic root nodes.
        let RangeTreeRef {
            range,
            value: Node::Range(imr),
            ..
        } = node
        else {
            continue;
        };

        let mut im_linetab = imr.line_table.borrow_mut();
        for line_record in unit_line_table.query(range.clone()) {
            let (ref file, Some(line)) = line_record.value else {
                // Skip records without line/file info.
                continue;
            };

            // Restrict range to the overlapping region with our node.
            let Some(overlap) = range_overlap(&line_record.range, &range) else {
                continue;
            };

            im_linetab.push(IntermediateLineTableEntry {
                rng: overlap,
                file: file.to_string(),
                line: line.get(),
            });
        }

        im_linetab.sort_unstable_by_key(|x| x.rng.start);
        im_linetab.dedup_by(|a, b| {
            let same_range = a.rng.start == b.rng.start;
            let same_line = a.line == b.line && a.file == b.file;
            same_range || same_line
        });
    }

    // With the line numbers assigned, now emit the ranges in symbfile format.
    for node in tree.iter_dfs() {
        let imr = match &node.value {
            Node::Range(imr) => imr,
            Node::SynthRoot => continue,
        };

        // If the function doesn't have a name, we can't really do anything
        // useful with it in symbolization. Skip.
        let Some(name) = imr.info.name()? else {
            continue;
        };

        let mut record = symbfile::Range {
            elf_va: node.range.start,
            length: (node.range.end - node.range.start) as _,
            func: demangle::demangle(&name).into_owned(),
            file: None,
            call_file: imr.info.call_file()?.map(|x| x.to_string()),
            call_line: imr.info.call_line().map(|x| x.get() as u32),
            depth: imr.info.depth() as _,
            line_table: SmallVec::new(),
        };

        let line_table = imr.line_table.borrow();

        for lte in line_table.iter() {
            if let Some(prev_file) = &record.file {
                // We should probably also split for holes in the line table
                // that aren't covered by inline instances, but computing this
                // is unfortunately rather expensive.

                if prev_file != &lte.file {
                    // File changed: split record.
                    let mut clone = record.clone();
                    clone.length = (lte.rng.start - record.elf_va) as u32;
                    visitor(clone)?;

                    record.elf_va = lte.rng.start;
                    record.length = (node.range.end - lte.rng.start) as u32;
                    record.line_table.clear();
                    record.file = Some(lte.file.clone());
                }
            } else {
                record.file = Some(lte.file.clone());
            }

            record.line_table.push(symbfile::LineTableEntry {
                offset: (lte.rng.start - record.elf_va) as _,
                line_number: lte.line as _,
            });
        }

        if !any_child_has_lines(node) {
            continue;
        }

        visitor(record)?;
    }

    Ok(())
}

fn process_unit(
    unit: dwarf::Unit<'_, '_>,
    mut visitor: impl FnMut(symbfile::Range) -> Result,
) -> Result {
    // If the line table is empty, we can't do anything useful with this unit. Skip.
    let Some(line_iter) = unit.line_iter() else {
        return Ok(());
    };

    // Construct an interval tree for fast lookups. We unfortunately have
    // to first collect it into a vector, then move it into the interval
    // tree because `line_iter` is a fallible iterator which cannot be used
    // to construct an interval tree directly.
    let line_table = IntervalTree::from_iter(
        line_iter
            .filter(|x| Ok(!x.rng.is_empty()))
            .map(|x| {
                Ok(Element {
                    range: x.rng,
                    value: (x.file, x.line),
                })
            })
            .collect::<Vec<_>>()?,
    );

    // Process all subroutines in the unit.
    let mut sr_iter = unit.subprograms();
    while let Some(routine) = sr_iter.next()? {
        process_subroutine(&line_table, routine, &mut visitor)?;
    }

    Ok(())
}

/// DWARF translation statistics.
#[derive(Debug, Default, PartialEq, Eq)]
pub struct Stats {
    /// Number of units that were successfully processed.
    pub units_ok: u64,

    /// Number of units that had to be skipped due to parsing issues.
    ///
    /// This includes units that were partially processed but encountered
    /// errors halfway through.
    pub units_broken: u64,
}

/// Extract address ranges and their source-file mapping from the given DWARF
/// sections.
fn extract_ranges(
    dw: &dwarf::Sections<'_>,
    mut visitor: impl FnMut(symbfile::Range) -> Result,
) -> Result<Stats> {
    let units = dw.units()?;
    let mut unit_iter = units.iter();
    let mut stats = Stats::default();

    loop {
        let unit = match unit_iter.next() {
            Ok(Some(unit)) => unit,
            Ok(None) => break,
            Err(e) => {
                debug!("Skipping unit with broken header: {:?}", e);
                stats.units_broken += 1;
                continue;
            }
        };

        debug!("Processing {:?}", &unit);
        match process_unit(unit, &mut visitor) {
            Ok(()) => stats.units_ok += 1,
            Err(e) => {
                debug!("Aborted unit processing due to error: {:?}", e);
                stats.units_broken += 1;
            }
        }
    }

    Ok(stats)
}

/// Extract symbol ranges from DWARF debug info.
pub struct Extractor<'dw, 'obj>(&'dw dwarf::Sections<'obj>);

impl<'dw, 'obj> Extractor<'dw, 'obj> {
    /// Create a new extractor.
    pub fn new(dw: &'dw dwarf::Sections<'obj>) -> Self {
        Self(dw)
    }
}

impl<'dw, 'obj> super::RangeExtractor for Extractor<'dw, 'obj> {
    fn extract(&self, visitor: RangeVisitor<'_>) -> super::Result<Option<super::Stats>> {
        let visitor_adapter = |range| visitor(range).map_err(Error::Visitor);
        extract_ranges(self.0, visitor_adapter)
            .map(|x| Some(super::Stats::Dwarf(x)))
            .map_err(super::Error::Dwarf)
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{
        dwarf, objfile,
        symbconv::{RangeExtractor as _, Stats as SymbconvStats},
        symbfile::{self, LineTableEntry},
        tests::testdata,
    };
    use std::io::{Seek, SeekFrom};

    #[test]
    fn inline() {
        let obj = objfile::File::load(&testdata("inline")).unwrap();
        let obj = obj.parse().unwrap();
        let dwarf = dwarf::Sections::load(&obj).unwrap();
        let mut out_file = tempfile::tempfile().unwrap();
        let extr = Extractor::new(&dwarf);
        let stats = extr.extract_to_symbfile(&mut out_file).unwrap();

        assert!(matches!(
            &stats,
            Some(SymbconvStats::Dwarf(Stats {
                units_ok: 1,
                units_broken: 0,
            })),
        ));

        out_file.seek(SeekFrom::Start(0)).unwrap();
        let mut reader = symbfile::Reader::new(out_file).unwrap();

        let mut rng: symbfile::Range = reader.read().unwrap().unwrap().unwrap_range();
        let src_file = "/media/share/Development/prodfiler/libpf-rs/testdata/inline.c";
        assert_eq!(rng.elf_va, 0x640);
        assert_eq!(rng.length, 0x664 - 0x640);
        assert_eq!(rng.func, "main");
        assert_eq!(rng.file.unwrap(), src_file,);
        assert_eq!(rng.call_line, None);
        assert_eq!(rng.call_file, None);
        assert_eq!(rng.depth, 0);

        // `symbtool dwarf -e inline dump` excerpt:
        //
        // [0x000640..0x000640) /media/share/Development/prodfiler/libpf-rs/testdata/inline.c:38
        // [0x000640..0x000640) /media/share/Development/prodfiler/libpf-rs/testdata/inline.c:39
        // [0x000640..0x000648) /media/share/Development/prodfiler/libpf-rs/testdata/inline.c:38
        // [0x000648..0x00064C) /media/share/Development/prodfiler/libpf-rs/testdata/inline.c:39
        // [...] (covered by inline instances)
        // [0x000658..0x000664) /media/share/Development/prodfiler/libpf-rs/testdata/inline.c:41

        assert_eq!(
            &rng.line_table[..],
            &[
                LineTableEntry {
                    offset: 0x640 - 0x640,
                    line_number: 38,
                },
                LineTableEntry {
                    offset: 0x648 - 0x640,
                    line_number: 39,
                },
                LineTableEntry {
                    offset: 0x658 - 0x640,
                    line_number: 41,
                },
            ]
        );

        rng = reader.read().unwrap().unwrap().unwrap_range();
        assert_eq!(rng.elf_va, 0x64c);
        assert_eq!(rng.length, 0x658 - 0x64c);
        assert_eq!(rng.depth, 1);
        assert_eq!(rng.func, "a_inline");
        assert_eq!(rng.call_file.unwrap(), src_file);
        assert_eq!(rng.call_line.unwrap(), 40);
        assert!(rng.line_table.is_empty());

        rng = reader.read().unwrap().unwrap().unwrap_range();
        assert_eq!(rng.elf_va, 0x64c);
        assert_eq!(rng.length, 0x658 - 0x64c);
        assert_eq!(rng.depth, 2);
        assert_eq!(rng.func, "b_inline");
        assert!(rng.line_table.is_empty());

        rng = reader.read().unwrap().unwrap().unwrap_range();
        assert_eq!(rng.elf_va, 0x64c);
        assert_eq!(rng.length, 0x658 - 0x64c);
        assert_eq!(rng.depth, 3);
        assert_eq!(rng.func, "c_inline");
        assert!(rng.line_table.is_empty());

        rng = reader.read().unwrap().unwrap().unwrap_range();
        assert_eq!(rng.elf_va, 0x64c);
        assert_eq!(rng.length, 0x658 - 0x64c);
        assert_eq!(rng.depth, 4);
        assert_eq!(rng.func, "d_inline");
        assert_eq!(
            &rng.line_table[..],
            &[LineTableEntry {
                offset: 0,
                line_number: 23,
            }]
        );

        rng = reader.read().unwrap().unwrap().unwrap_range();
        assert_eq!(rng.elf_va, 0x7c0);
        assert_eq!(rng.length, 0x7c4 - 0x7c0);
        assert_eq!(rng.depth, 0);
        assert_eq!(rng.func, "a");
        assert_eq!(
            &rng.line_table[..],
            &[LineTableEntry {
                offset: 0,
                line_number: 19,
            }]
        );

        // All same schema as `a` above: no need to repeat everything.
        rng = reader.read().unwrap().unwrap().unwrap_range();
        assert_eq!(rng.func, "b");
        rng = reader.read().unwrap().unwrap().unwrap_range();
        assert_eq!(rng.func, "c");
        rng = reader.read().unwrap().unwrap().unwrap_range();
        assert_eq!(rng.func, "d");

        assert!(reader.read().unwrap().is_none());
    }
}
