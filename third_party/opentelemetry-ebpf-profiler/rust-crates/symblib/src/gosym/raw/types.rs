// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! New-types for offsets and other integer types.
//!
//! In Go's runtime there are many different offset types that each need to be
//! added to a particular base address to calculate the final pointer. We have
//! a separate offset type for each such base address.

use crate::VirtAddr;

/// Offset within the `.gopclntab` section.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct GopclntabOffset(pub u64);

/// Offset within `runtime.funcnametab`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct FuncNameOffset(pub u32);

/// Offset within `runtime.filetab`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct FileNameOffset(pub u32);

impl FileNameOffset {
    pub const INVALID: Self = FileNameOffset(u32::MAX);
}

/// Offset within `runtime.functab`.
///
/// `u32` in versions >=1.18, `u64` in older ones. We simply widen it to
/// `u64` for all versions.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct FuncTabOffset(pub u64);

/// Offset within `go:func.*`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct GoFuncOffset(pub u32);

impl GoFuncOffset {
    pub const INVALID: Self = GoFuncOffset(u32::MAX);
}

/// Function data reference, either relative or absolute.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum FuncDataRef {
    Addr(VirtAddr),
    Offs(GoFuncOffset),
}

/// Offset within `runtime.pctab`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct PcTabOffset(pub u32);

/// Index within `runtime.cutab`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct CuTabIndex(pub u32);

/// Virtual address offset relative to `text_start`.
///
/// This is usually stored as `u32`, but some code paths accumulate deltas
/// that in sum can then become larger than `u32`.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct TextStartOffset(pub u64);

/// Pointer to code, either relative or absolute.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CodePtr {
    /// Absolute virtual address (Go versions >= 1.18).
    Addr(VirtAddr),

    /// Offset relative to text start (Go versions < 1.18).
    Offs(TextStartOffset),
}

/// Identifier for special internal Go functions.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct FuncId(pub u8);

/// Index within an inline tree.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct InlineTreeIndex(pub u32);
