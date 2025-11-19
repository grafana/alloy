// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Minimal coverage bitmap implementation.

use crate::{range_overlap, VirtAddr};
use std::num::NonZeroU64;
use std::{io, ops};

/// Coverage tracker for VA ranges.
#[derive(Debug)]
pub struct CovMap {
    range: ops::Range<VirtAddr>,
    scale: NonZeroU64,
    bit_vec: Vec<u8>,
}

impl CovMap {
    /// Allocate a new coverage map for the given address range.
    pub fn with_buckets(max_buckets: NonZeroU64, range: ops::Range<VirtAddr>) -> Self {
        let len = range.end.saturating_sub(range.start);
        let scale = (len / max_buckets.get()).max(1);
        Self::with_scale(NonZeroU64::new(scale).unwrap(), range)
    }

    /// Allocate a new coverage map with the given granularity (scale) in bytes.
    pub fn with_scale(scale: NonZeroU64, range: ops::Range<VirtAddr>) -> Self {
        let len = range.end.saturating_sub(range.start);
        let bit_scale = scale.get() * 8;
        let vec_len = len.div_ceil(bit_scale);

        Self {
            range,
            scale,
            bit_vec: vec![0; vec_len as usize],
        }
    }

    /// Returns the range covered by this map.
    pub fn map_range(&self) -> ops::Range<VirtAddr> {
        self.range.clone()
    }

    /// Mark the given range as covered.
    ///
    /// Updates to addresses outside the map's range are discarded.
    pub fn add_range(&mut self, rng: ops::Range<VirtAddr>) {
        let Some(bit_indices) = self.bit_indices_for_va_range(rng) else {
            return;
        };

        for offset in bit_indices {
            let byte = offset / 8;
            let bit = offset % 8;
            self.bit_vec[byte] |= 1 << bit;
        }
    }

    fn bit_indices_for_va_range(&self, rng: ops::Range<VirtAddr>) -> Option<ops::Range<usize>> {
        let Some(mut overlap) = range_overlap(&self.range, &rng) else {
            return None;
        };

        // Rebase to coverage map range.
        overlap.start -= self.range.start;
        overlap.end -= self.range.start;

        // Reduce resolution to scale.
        overlap.start /= self.scale.get();
        overlap.end = overlap.end.div_ceil(self.scale.get());

        Some(overlap.start as usize..overlap.end as usize)
    }

    /// Checks whether the given range is at least partially covered.
    pub fn range_partially_covered(&self, rng: ops::Range<VirtAddr>) -> bool {
        let Some(bit_indices) = self.bit_indices_for_va_range(rng) else {
            return false;
        };

        for offset in bit_indices {
            let byte_offs = offset / 8;
            let bit_offs = offset % 8;
            let byte = self.bit_vec[byte_offs];
            if byte & (1 << bit_offs) != 0 {
                return true;
            }
        }

        false
    }

    /// Prints a coverage map to the given output stream.
    ///
    /// Uses unicode braille characters for increased compactness.
    pub fn print_table(&self, mut out: impl io::Write) -> io::Result<()> {
        const CHARS_PER_LINE: usize = 80;

        writeln!(out, "Address    ┃ Coverage")?;
        writeln!(out, "━━━━━━━━━━━╋━{}", "━".repeat(CHARS_PER_LINE))?;

        for (chunk, i) in self.bit_vec.chunks(CHARS_PER_LINE).zip(0u64..) {
            let addr = self.range.start + i * 8 * self.scale.get() * CHARS_PER_LINE as u64;

            write!(out, "0x{:08x} ┃ ", addr)?;

            for block in chunk {
                // Unicode braille characters are constructed by adding an u8
                // where each bit corresponds to one of the 8 braille dots to
                // the char-code of the first braille char ('\u{2800}').
                let char_code = 0x2800u32 + *block as u32;
                write!(out, "{}", char::from_u32(char_code).unwrap())?;
            }

            writeln!(out)?;
        }

        Ok(())
    }
}

/// Error indicating that two segments overlap (not allowed).
#[derive(Debug, thiserror::Error)]
#[error("segments have overlap in range {0:?}")]
pub struct SegmentOverlapError(ops::Range<VirtAddr>);

macro_rules! impl_map_for_addr {
    ( $this:ident, $va:ident $(, $maybe_mut:tt)?  ) => {{
        // Fast path for 0..1 inner maps.
        match $this.maps.len() {
            0 => return None,
            1 if $this.maps[0].map_range().contains(&$va) =>
                return Some(&$($maybe_mut)* $this.maps[0]),
            1 => return None,
            _ => { /* continue below */ }
        }

        // More than one map: bsearch.
        match $this.maps.binary_search_by_key(&$va, |x| x.range.start) {
            // Exact match.
            Ok(idx) => Some(&$($maybe_mut)* $this.maps[idx]),

            // Inner map array is empty.
            Err(0) => None,

            // Either found somewhere within a map or outside valid range.
            Err(idx) => $this.maps[idx - 1]
                .map_range()
                .contains(&$va)
                .then_some(&$($maybe_mut)* $this.maps[idx - 1]),
        }
    }};
}

/// Coverage tracker for multiple non-overlapping VA ranges.
#[derive(Debug, Default)]
pub struct SegmentedCovMap {
    /// Inner maps ordered by start VA. Cannot overlap.
    maps: Vec<CovMap>,
}

impl SegmentedCovMap {
    /// Create an empty segmented coverage map.
    pub fn new() -> Self {
        Self::default()
    }

    /// Adds a new segment to the map.
    pub fn add_segment(&mut self, new: CovMap) -> Result<&mut Self, SegmentOverlapError> {
        for seg in &self.maps {
            if let Some(overlap) = range_overlap(&seg.map_range(), &new.map_range()) {
                return Err(SegmentOverlapError(overlap));
            }
        }

        self.maps.push(new);

        Ok(self)
    }

    /// Mark the given range as covered.
    ///
    /// The range is assigned to the segment containing `rng.start`. If the
    /// range spans more than one segment, the portion of the range that doesn't
    /// overlap with the range of the initial segment is discarded. This
    /// limitation is imposed to simplify the implementation and could be lifted
    /// later if necessary.
    pub fn add_range(&mut self, rng: ops::Range<VirtAddr>) {
        let Some(seg) = self.map_for_addr_mut(rng.start) else {
            return;
        };

        seg.add_range(rng)
    }

    /// Locates the inner map containing the given VA (mutable).
    fn map_for_addr_mut(&mut self, va: VirtAddr) -> Option<&mut CovMap> {
        impl_map_for_addr!(self, va, mut)
    }

    /// Locates the inner map containing the given VA.
    fn map_for_addr(&self, va: VirtAddr) -> Option<&CovMap> {
        impl_map_for_addr!(self, va)
    }

    /// Checks whether the given range is at least partially covered.
    ///
    /// Current implementation requires the range start to be contained in the
    /// map and won't assign coverage to more than one segment. If you submit a
    /// range that starts in segment A and then proceeds into segment B,
    /// coverage will only be assigned to segment A. This limitation is imposed
    /// to simplify the implementation and could be lifted later if necessary.
    pub fn range_partially_covered(&self, rng: ops::Range<VirtAddr>) -> bool {
        let Some(map) = self.map_for_addr(rng.start) else {
            return false;
        };

        map.range_partially_covered(rng)
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn partial_cov_at_scale_1() {
        let scale = NonZeroU64::new(1).unwrap();
        let mut map = CovMap::with_scale(scale, 0x100..0x200);

        map.add_range(0x090..0x110); // A
        map.add_range(0x1A0..0x1E0); // B
        map.add_range(0x1A2..0x1EF); // C
        map.add_range(0x152..0x191); // D

        assert!(
            !map.range_partially_covered(0x92..0x94),
            "outside of map range and should not be included",
        );
        assert!(
            !map.range_partially_covered(0x80..0x100),
            "outside of map range and should not be included",
        );

        assert!(map.range_partially_covered(0x100..0x101), "inside A");
        assert!(map.range_partially_covered(0x90..0x101), "overlaps A");
        assert!(map.range_partially_covered(0x90..0x101), "overlaps A");

        for s in 0x1A0..0x1EF {
            assert!(
                !map.range_partially_covered(s..s),
                "empty range doesn't cover anything",
            );

            for l in 1..10 {
                assert!(
                    map.range_partially_covered(s..s + l),
                    "covered by either B or C",
                );
            }
        }

        assert!(!map.range_partially_covered(0x110..0x111), "just after A");
        assert!(!map.range_partially_covered(0x1EF..0x1F0), "just after C");
        assert!(!map.range_partially_covered(0x191..0x192), "just after D");
    }
}
