// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Extract and combine symbols from multiple sources.

use super::{RangeExtractor, RangeVisitor};
use crate::covmap::{CovMap, SegmentedCovMap};
use crate::{objfile, range_overlap, VirtAddr};
use std::num::NonZeroU64;
use std::ops;

/// Result type shorthand.
pub type Result<T = (), E = Error> = std::result::Result<T, E>;

/// Errors that can occurr during extraction from multiple sources.
#[non_exhaustive]
#[allow(missing_docs)]
#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("object file has multiple overlapping code sections")]
    OverlappingCodeSections,

    #[error("all range extractors failed: {0:?}")]
    AllExtractorsFailed(Box<Vec<(String, super::Error)>>),

    #[error("objfile: {0}")]
    Objfile(#[from] objfile::Error),
}

/// Per extractor statistics.
#[derive(Debug)]
pub struct PerExtractorStats {
    /// Name of the extractor.
    pub name: String,

    /// Total number of ranges produced.
    pub ranges_produced: u64,

    /// Number of ranges that made it into the output file.
    pub ranges_accepted: u64,

    /// Number of ranges that were rejected because they were buggy or already covered.
    pub ranges_rejected: u64,

    /// Number of inline children not correctly following their parent root.
    pub unexpected_inline_children: u64,

    /// Extractor specific statistics (on success).
    pub stats: Option<super::Stats>,

    /// Extractor error (in case of failure).
    pub error: Option<super::Error>,
}

/// Combined statistics from all inner extractors.
#[derive(Debug, Default)]
pub struct Stats {
    /// Number of extractors that completed successful.
    pub extractors_succeeded: u64,

    /// Number of extractors that exited prematurely.
    pub extractors_failed: u64,

    /// Detailed per-extractor statistics.
    pub per_extractor: Vec<PerExtractorStats>,
}

impl Stats {
    /// Sum of all ranges accepted from all extractors.
    pub fn total_ranges_accepted(&self) -> u64 {
        self.per_extractor.iter().map(|x| x.ranges_accepted).sum()
    }
}

/// Extractor that combines the outputs from multiple other extractors.
///
/// The output of multiple range extractors are combined by keeping a
/// coverage map of ranges that were already emitted by previous extractors,
/// dropping any duplicate ranges. Extractors added earlier take precedence
/// over extractors added later.
pub struct Extractor<'inner> {
    inner: Vec<(String, Box<dyn RangeExtractor + Send + 'inner>)>,
    cov_map_scale: u64,
    code_sections: Vec<ops::Range<VirtAddr>>,
}

impl<'inner> Extractor<'inner> {
    /// Create a new multi range extractor.
    pub fn new(obj: &objfile::Reader<'_>) -> Result<Self> {
        Ok(Self {
            inner: vec![],
            cov_map_scale: obj.arch().map_or(1, |x| x.min_code_align()),
            code_sections: obj
                .memory_map()?
                .iter()
                .filter(|region| region.protection().map_or(false, |p| p.x))
                .map(objfile::Section::va_range)
                .collect(),
        })
    }

    /// Add a range extractor.
    ///
    /// Earlier entries take precedence over later ones.
    pub fn add(
        &mut self,
        name: impl Into<String>,
        extr: impl RangeExtractor + Send + 'inner,
    ) -> &mut Self {
        self.inner.push((name.into(), Box::new(extr)));
        self
    }
}

impl RangeExtractor for Extractor<'_> {
    fn extract(&self, visitor: RangeVisitor<'_>) -> super::Result<Option<super::Stats>> {
        let mut cov_map = SegmentedCovMap::new();
        let mut stats = Stats::default();
        let scale = NonZeroU64::new(self.cov_map_scale).expect("buggy coverage map scale");

        for sec in &self.code_sections {
            cov_map
                .add_segment(CovMap::with_scale(scale, sec.clone()))
                .map_err(|_| super::Error::Multi(Error::OverlappingCodeSections))?;
        }

        for (name, extractor) in &self.inner {
            let per_extr_stats = run_extractor(&mut cov_map, name.clone(), &**extractor, visitor);

            if per_extr_stats.error.is_some() {
                stats.extractors_failed += 1;
            } else {
                stats.extractors_succeeded += 1;
            }

            stats.per_extractor.push(per_extr_stats);
        }

        if stats.extractors_succeeded == 0 && stats.extractors_failed > 0 {
            let errors: Vec<_> = stats
                .per_extractor
                .into_iter()
                .filter_map(|x| Some((x.name, x.error?)))
                .collect();

            let error = Error::AllExtractorsFailed(Box::new(errors));
            return Err(super::Error::Multi(error));
        }

        Ok(Some(super::Stats::Multi(stats)))
    }
}

fn run_extractor(
    cov_map: &mut SegmentedCovMap,
    name: String,
    extractor: &dyn RangeExtractor,
    visitor: RangeVisitor<'_>,
) -> PerExtractorStats {
    let mut ranges_produced = 0;
    let mut ranges_rejected = 0;
    let mut ranges_accepted = 0;
    let mut unexpected_inline_children = 0;

    // Tracks the last accepted top-level (depth = 0) range. Used to also accept
    // the inline records following after the root even if the top-level
    // function already marked the whole range as covered.
    let mut accept_inline_for: Option<ops::Range<VirtAddr>> = None;

    let extr_result = extractor.extract(&mut |rng| {
        ranges_produced += 1;

        // Start of new top-level function? Consult coverage map.
        if rng.depth == 0 {
            if cov_map.range_partially_covered(rng.va_range()) {
                ranges_rejected += 1;
                accept_inline_for = None;
            } else {
                ranges_accepted += 1;
                accept_inline_for = Some(rng.va_range());
                cov_map.add_range(rng.va_range());
                visitor(rng)?;
            }

            return Ok(());
        }

        // Inline children: accept if we previously accepted the corresponding
        // top-level function range from this extractor.
        if let Some(accept_range) = &accept_inline_for {
            if range_overlap(accept_range, &rng.va_range()) != Some(rng.va_range()) {
                ranges_rejected += 1;
                unexpected_inline_children += 1;
                return Ok(());
            }

            ranges_accepted += 1;
            visitor(rng)?;
        } else {
            ranges_rejected += 1;
        }

        Ok(())
    });

    let (stats, error) = match extr_result {
        Ok(x) => (x, None),
        Err(e) => (None, Some(e)),
    };

    PerExtractorStats {
        name: name.clone(),
        ranges_produced,
        ranges_accepted,
        ranges_rejected,
        unexpected_inline_children,
        stats,
        error,
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::tests::testdata;
    use crate::{symbconv, symbfile};
    use std::ops;

    #[test]
    fn empty() {
        let obj = objfile::File::load(&testdata("inline")).unwrap();
        let obj = obj.parse().unwrap();
        let multi = Extractor::new(&obj).unwrap();
        let stats = multi.extract(&mut |_| Ok(())).unwrap().unwrap();

        let symbconv::Stats::Multi(stats) = stats else {
            panic!("unexpected stats type produced");
        };

        assert!(stats.per_extractor.is_empty());
        assert_eq!(stats.extractors_failed, 0);
        assert_eq!(stats.extractors_succeeded, 0);
    }

    struct MockExtractor(Vec<(/* depth */ u32, ops::Range<VirtAddr>)>);

    impl RangeExtractor for MockExtractor {
        fn extract(&self, visitor: RangeVisitor<'_>) -> symbconv::Result<Option<symbconv::Stats>> {
            for (depth, va_range) in &self.0 {
                visitor(symbfile::Range {
                    elf_va: va_range.start,
                    length: (va_range.end - va_range.start).try_into().unwrap(),
                    func: "some_func".to_string(),
                    file: Some("some_file".to_string()),
                    call_file: None,
                    call_line: None,
                    depth: *depth,
                    line_table: Default::default(),
                })
                .unwrap();
            }

            Ok(None)
        }
    }

    #[test]
    fn multi() {
        let obj = objfile::File::load(&testdata("inline")).unwrap();
        let obj = obj.parse().unwrap();
        let code_sec = obj.load_section(b".text").unwrap().unwrap();
        let code_va = code_sec.virt_addr();

        let extr1 = MockExtractor(vec![
            (0, code_va + 0x10..code_va + 0x20), // A
            (1, code_va + 0x1A..code_va + 0x1F), // A1
            (2, code_va + 0x1C..code_va + 0x1F), // A2
            (2, code_va + 0xCC..code_va + 0xDD), // A3 (buggy inline range covered by root)
            (0, code_va + 0x30..code_va + 0x40), // B
            (0, code_va + 0x3A..code_va + 0x60), // C (partial overlap with B)
            (1, code_va + 0x40..code_va + 0x41), // C1
        ]);

        let extr2 = MockExtractor(vec![
            (0, code_va + 0x10..code_va + 0x20), // D (full overlaps A)
            (0, code_va + 0x50..code_va + 0x70), // E
            (0, code_va + 0x32..code_va + 0x3B), // F (partial overlap with B)
        ]);

        let mut multi = Extractor::new(&obj).unwrap();
        multi.add("extr1", extr1);
        multi.add("extr2", extr2);

        let mut emitted_ranges = Vec::new();
        let stats = multi
            .extract(&mut |rng| {
                emitted_ranges.push(rng);
                Ok(())
            })
            .unwrap()
            .unwrap();

        let symbconv::Stats::Multi(stats) = stats else {
            panic!("unexpected stats type produced");
        };

        assert_eq!(stats.extractors_succeeded, 2);
        assert_eq!(stats.extractors_failed, 0);

        let stats1 = &stats.per_extractor[0];
        assert_eq!(stats1.name, "extr1");
        assert_eq!(stats1.ranges_accepted, 4);
        assert_eq!(stats1.ranges_produced, 7);
        assert_eq!(stats1.ranges_rejected, 3);
        assert_eq!(stats1.unexpected_inline_children, 1);

        let stats2 = &stats.per_extractor[1];
        assert_eq!(stats2.name, "extr2");
        assert_eq!(stats2.ranges_accepted, 1);
        assert_eq!(stats2.ranges_produced, 3);
        assert_eq!(stats2.ranges_rejected, 2);
        assert_eq!(stats2.unexpected_inline_children, 0);

        assert_eq!(
            emitted_ranges.len() as u64,
            stats1.ranges_accepted + stats2.ranges_accepted
        );
    }
}
