// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#![doc = include_str!("../README.md")]
#![warn(missing_docs)]

pub mod covmap;
pub mod dbglog;
pub mod demangle;
pub mod disas;
pub mod dwarf;
pub mod fileid;
pub mod gosym;
pub mod objfile;
pub mod retpads;
pub mod symbconv;
pub mod symbfile;

/// Type-erased error type.
///
/// We primarily use this to hand out errors from third-party libraries where
/// lifting them into distinct error variants didn't make sense because no
/// consumer cares about differentiating between different error variants.
pub type AnyError = Box<dyn std::error::Error + Send + Sync>;

/// Virtual address in the ELF / mach-O address space.
pub type VirtAddr = u64;

/// Returns the overlap of two given ranges, or `None` if no overlap.
///
/// # Examples
///
/// ```
/// # use symblib::range_overlap;
/// assert_eq!(range_overlap(&(0..5), &(1..3)), Some(1..3));
/// assert_eq!(range_overlap(&(0..5), &(5..10)), None);
/// assert_eq!(range_overlap(&(0..5), &(4..10)), Some(4..5));
/// assert_eq!(range_overlap(&(4..10), &(0..5)), Some(4..5)); // order is irrelevant
/// assert_eq!(range_overlap(&(0..0), &(0..1)), None); // empty ranges can't overlap anything!
/// ```
pub fn range_overlap<T: Ord + Copy>(
    a: &std::ops::Range<T>,
    b: &std::ops::Range<T>,
) -> Option<std::ops::Range<T>> {
    let c = std::ops::Range {
        start: a.start.max(b.start),
        end: a.end.min(b.end),
    };

    if c.is_empty() {
        None
    } else {
        Some(c)
    }
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;

    /// Construct path for test files living in `./testdata`.
    pub fn testdata(name: &str) -> PathBuf {
        PathBuf::from(env!("CARGO_MANIFEST_DIR"))
            .join("testdata")
            .join(name)
    }

    // Get go testdata binaries
    pub fn go_testdata() -> Vec<PathBuf> {
        vec![
            testdata("go-1.20.14"),
            testdata("go-1.22.12"),
            testdata("go-1.24.0"),
        ]
    }
}
