// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Translates Go symbols into symbfile ranges.
//!
//! This is currently still very basic and doesn't support inline functions
//! or constructing line tables.

use crate::{debug, gosym, objfile, symbfile, AnyError};
use fallible_iterator::FallibleIterator as _;

/// Result type shorthand.
pub type Result<T = (), E = Error> = std::result::Result<T, E>;

/// Errors that can occurr during Go symbol extraction.
#[non_exhaustive]
#[allow(missing_docs)]
#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("go metadata parsing error: {0}")]
    Gosym(#[from] gosym::Error),

    #[error("visitor returned an error: {0}")]
    Visitor(#[source] AnyError),

    #[error("line mapping failed with error: {0}")]
    GoSymbolBadLineMapping(#[source] AnyError),
}

/// Go symbol extraction statistics.
#[derive(Debug, Default)]
pub struct Stats {
    /// Functions that we succeeded extracting symbols from.
    pub funcs_ok: u64,

    /// Functions that we had to skip due to parsing errors.
    pub funcs_skipped: u64,

    /// Whether the executable was detected to be a Go executable.
    pub is_go_binary: bool,
}

/// `.gopclntab` symbol extractor.
pub struct Extractor<'obj>(&'obj objfile::Reader<'obj>);

impl<'obj> Extractor<'obj> {
    /// Create a new extractor.
    pub fn new(obj: &'obj objfile::Reader<'obj>) -> Self {
        Extractor(obj)
    }
}

impl<'obj> super::RangeExtractor for Extractor<'obj> {
    fn extract(&self, visitor: super::RangeVisitor<'_>) -> super::Result<Option<super::Stats>> {
        extract_ranges(self.0, visitor)
            .map(|x| Some(super::Stats::Go(x)))
            .map_err(super::Error::Go)
    }
}

fn find_func_end(func: &gosym::Func<'_, '_>) -> Result<Option<u64>> {
    let mut end: Option<u64> = None;
    let mut iter = func.line_mapping()?;
    loop {
        match iter.next()? {
            Some((rng, _)) => {
                if let Some(e) = end {
                    if rng.end < e {
                        continue;
                    }
                }
                end = Some(rng.end)
            }
            None => break,
        }
    }
    Ok(end)
}

fn extract_ranges(obj: &objfile::Reader<'_>, visitor: super::RangeVisitor<'_>) -> Result<Stats> {
    let mut stats = Stats::default();

    let go = match gosym::GoRuntimeInfo::open(obj) {
        Ok(x) => x,
        Err(gosym::Error::GopclntabNotFound) => return Ok(stats),
        Err(other) => return Err(other.into()),
    };

    stats.is_go_binary = true;

    let mut func_iter = go.funcs()?;
    while let Some(func) = func_iter.next()? {
        let end = match find_func_end(&func) {
            Ok(Some(e)) => e,
            Ok(None) => {
                debug!(
                    "WARN: unable to determine end of function ({})",
                    func.name()?
                );
                stats.funcs_skipped += 1;
                continue;
            }
            Err(Error::Gosym(gosym::Error::BadLineNumber)) => {
                // skip function once we hit a BadLineNumber error
                debug!("WARN: bad line number of function ({})", func.name()?);
                stats.funcs_skipped += 1;
                continue;
            }
            Err(e) => {
                return Err(Error::GoSymbolBadLineMapping(e.into()));
            }
        };

        let length = end.saturating_sub(func.start_addr());

        if length == 0 {
            debug!("WARN: zero function length ({})", func.name()?);
            stats.funcs_skipped += 1;
            continue;
        }

        // Pick first file and hope for the best. So far this has worked for
        // all samples that I've looked at. Even in the presence of inline
        // functions there will always be a prologue that has the file of the
        // outer function assigned to it.
        let file = func
            .file_mapping()?
            .find_map(|(_, name)| Ok(name.map(|x| x.to_owned())))?;

        let range = symbfile::Range {
            elf_va: func.start_addr(),
            length: length as _,
            func: func.name()?.to_owned(),
            file,
            call_file: None,
            call_line: None,
            depth: 0,
            line_table: Default::default(),
        };

        visitor(range).map_err(Error::Visitor)?;

        stats.funcs_ok += 1;
    }

    Ok(stats)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::tests::go_testdata;

    #[test]
    fn test_extract_ranges() -> Result<(), AnyError> {
        for test_file in go_testdata() {
            let obj = objfile::File::load(&test_file)?;
            let obj = obj.parse()?;

            let mut ranges = Vec::new();
            let mut visitor = |range: symbfile::Range| -> Result<(), AnyError> {
                ranges.push(range);
                Ok(())
            };
            extract_ranges(&obj, &mut visitor)?;

            // Verify we got some ranges
            assert!(!ranges.is_empty());

            // Verify ranges contains the main function
            assert!(
                ranges.iter().any(|range| range.func == "main.main"
                    && range.file.is_some()
                    && range.file.as_ref().unwrap().ends_with("main.go")),
                "main.main not found in {:?}",
                test_file
            );

            // Verify ranges are valid
            for range in ranges {
                // Basic validity checks
                assert!(range.elf_va > 0);
                assert!(range.length > 0);
                assert!(!range.func.is_empty());
                assert!(range.file.is_some());
            }
        }
        Ok(())
    }
}
