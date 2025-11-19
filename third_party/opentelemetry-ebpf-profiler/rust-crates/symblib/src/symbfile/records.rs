// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Rust representation of the payload data in `symbfile` files.
//!
//! This is a higher-level, more idiomatic representation of the protobuf
//! messages, hiding away implementation details like strings getting replaced
//! with references into the line table, relative integer encodings and columnar
//! representations.

use crate::VirtAddr;

use smallvec::SmallVec;

/// [`Range`] or [`ReturnPad`] symbfile record.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Record {
    /// Range symbol information.
    Range(Range),
    /// Point symbol information.
    ReturnPad(ReturnPad),
}

impl Record {
    /// Assume that the record is a range and unwrap it.
    ///
    /// # Panics
    ///
    /// If the record is not in fact a range.
    pub fn unwrap_range(self) -> Range {
        match self {
            Record::Range(range) => range,
            _ => panic!("tried to unwrap a non-range as a range"),
        }
    }
}

/// Create a [`Record`] from a [`Range`].
impl From<Range> for Record {
    fn from(x: Range) -> Self {
        Self::Range(x)
    }
}

/// Create a [`Record`] from a [`ReturnPad`].
impl From<ReturnPad> for Record {
    fn from(x: ReturnPad) -> Self {
        Self::ReturnPad(x)
    }
}

/// High-level representation of the [`RangeV1`] protobuf struct.
///
/// Please refer to [raw protobuf message][`RangeV1`] for more details.
///
/// [`RangeV1`]: super::proto::RangeV1
#[allow(missing_docs)]
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Range {
    /// Start address of the instruction range, in ELF virtual address space.
    pub elf_va: VirtAddr,
    /// Length of the instruction sequence.
    pub length: u32,
    /// Demangled name of the function.
    pub func: String,
    /// Source file that these instructions were generated from.
    pub file: Option<String>,
    /// The file that issued the call to the inline function. `None` if depth = 0
    /// or if the call file is equal to the file of the parent record record
    /// (depth - 1).
    pub call_file: Option<String>,
    /// Absolute line number of the call to the inline function. 0 if depth is 0.
    pub call_line: Option<u32>,
    /// Depth in the inline function tree, starting at 0 for the top-level function.
    pub depth: u32,
    /// Line table for this executable range.
    pub line_table: SmallVec<[LineTableEntry; 8]>,
}

impl Range {
    /// Construct a range from the `elf_va` and `length` fields.
    pub fn va_range(&self) -> std::ops::Range<VirtAddr> {
        self.elf_va..(self.elf_va.saturating_add(u64::from(self.length)))
    }

    /// Looks up the line number for the given virtual address.
    ///
    /// Note that the result of this method is only valid if you made sure that
    /// this range is the most concrete (highest depth) instance covering this
    /// range. For ranges that are covered by other inline instances, please
    /// refer to the `call_line` field in the `depth + 1` range instead.
    pub fn line_number_for_va(&self, va: VirtAddr) -> Option<u32> {
        let Some(max_offs) = va.checked_sub(self.elf_va) else {
            return None;
        };

        let mut line = None;
        for lte in &self.line_table {
            if lte.offset as VirtAddr > max_offs {
                break;
            }
            line = Some(lte.line_number);
        }

        line
    }
}

/// High-level representation of the [`LineTableEntry`] protobuf struct.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct LineTableEntry {
    /// Offset relative to [`Range::elf_va`].
    pub offset: u32,
    /// Line number in the source file.
    pub line_number: u32,
}

/// High-level representation of the [`ReturnPadV1`] protobuf struct.
///
/// Please refer to [raw protobuf message][`ReturnPadV1`] for more details.
///
/// [`ReturnPadV1`]: super::proto::ReturnPadV1
#[allow(missing_docs)]
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ReturnPad {
    /// Address of the return pad, in ELF virtual address space.
    pub elf_va: VirtAddr,

    /// Inline stack trace for the address.
    pub entries: SmallVec<[ReturnPadEntry; 4]>,
}

/// AoS representation of the [`ReturnPadV1`] columnar stack trace.
///
/// Please refer to the [raw protobuf message][`ReturnPadV1`] for details.
///
/// [`ReturnPadV1`]: super::proto::ReturnPadV1
#[allow(missing_docs)]
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ReturnPadEntry {
    /// Name of the function.
    pub func: String,
    /// Source file that these instructions were generated from.
    pub file: Option<String>,
    /// Absolute source line number.
    pub line: Option<u32>,
}

#[cfg(test)]
mod tests {
    use super::*;
    use smallvec::smallvec;

    #[test]
    fn line_number_lookup() {
        let range = Range {
            elf_va: 0x123,
            depth: 0,
            length: 0,
            func: "".into(),
            file: None,
            call_line: None,
            call_file: None,
            line_table: smallvec![
                LineTableEntry {
                    offset: 2,
                    line_number: 2,
                },
                LineTableEntry {
                    offset: 4,
                    line_number: 5,
                },
                LineTableEntry {
                    offset: 100,
                    line_number: 99,
                },
            ],
        };

        assert_eq!(range.line_number_for_va(0x123 - 1), None);
        assert_eq!(range.line_number_for_va(0x123 + 0), None);
        assert_eq!(range.line_number_for_va(0x123 + 1), None);
        assert_eq!(range.line_number_for_va(0x123 + 2), Some(2));
        assert_eq!(range.line_number_for_va(0x123 + 3), Some(2));
        assert_eq!(range.line_number_for_va(0x123 + 4), Some(5));
        assert_eq!(range.line_number_for_va(0x123 + 99), Some(5));
        assert_eq!(range.line_number_for_va(0x123 + 100), Some(99));
        assert_eq!(range.line_number_for_va(0x123 + 10000), Some(99));
    }
}
