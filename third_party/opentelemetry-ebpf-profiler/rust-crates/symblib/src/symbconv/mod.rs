// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Extract symbol info and convert it to [`symbfile`] format.

use crate::{objfile, symbfile, AnyError, VirtAddr};
use std::io;

/// Result type shorthand.
pub type Result<T = (), E = Error> = std::result::Result<T, E>;

/// Errors that can occurr during symbol extraction.
#[non_exhaustive]
#[allow(missing_docs)]
#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("symbfile: {0}")]
    Symbfile(#[from] symbfile::Error),

    #[error("objfile: {0}")]
    Objfile(#[from] objfile::Error),

    #[error("obj sym extraction: {0}")]
    Obj(#[source] AnyError),

    #[error("DWARF: {0}")]
    Dwarf(#[from] dwarf::Error),

    #[error("multi extractor: {0}")]
    Multi(#[from] multi::Error),

    #[error("Go: {0}")]
    Go(#[from] go::Error),
}

/// Callback processing ranges.
pub type RangeVisitor<'a> = &'a mut dyn FnMut(symbfile::Range) -> Result<(), AnyError>;

/// Extractor-specific statistics collected during symbol extraction.
#[derive(Debug)]
pub enum Stats {
    /// Go symbol extractor statistics.
    Go(go::Stats),
    /// DWARF symbol extractor statistics.
    Dwarf(dwarf::Stats),
    /// Multi symbol extractor statistics.
    Multi(multi::Stats),
}

/// Common interface for all range extractor.
///
/// A range extractor is a component that can extract mappings from address
/// ranges to symbols (function name, file name, line number, etc.).
pub trait RangeExtractor {
    /// Extract address ranges and their source-file mappings.
    ///
    /// The `visitor` callback is invoked for every range extracted from the
    /// executable. Returning an error will abort further execution and return
    /// early.
    ///
    /// Implementations that support inline function extraction must make sure
    /// that inline ranges (depth > 0) always immediately follow after the
    /// top-level (depth = 0) range.
    fn extract(&self, visitor: RangeVisitor<'_>) -> Result<Option<Stats>>;

    /// Extract address ranges and their source-file mappings and write them to
    /// an IO writer in range symbfile format.
    ///
    /// The caller should pass a buffered writer for performance reasons.
    fn extract_to_symbfile(&self, out: &mut dyn io::Write) -> Result<Option<Stats>> {
        let mut out = symbfile::Writer::new(out)?;
        let mut visitor = |range| {
            out.write(range)
                .map_err(|x| AnyError::from(Error::Symbfile(x)))
        };
        let stats = self.extract(&mut visitor)?;
        out.finalize()?;
        Ok(stats)
    }
}

/// Hold information about a symbol and its origin.
pub struct ResolvedSymbol {
    /// Start address of a symbol
    pub start_addr: VirtAddr,
    /// Function name associated with an address.
    pub function_name: Option<String>,
    /// File name that hold this function.
    pub file_name: Option<String>,
    /// Line number associcated with this virtual address.
    pub line_number: Option<u32>,
}

/// Common interface to tesolve symbols for a specific program counter address.
pub trait PointResolver {
    /// Returns all symbols that match the given program counter address.
    ///
    /// The returned vector contains all resolved symbols at the given address,
    /// which can include both the direct function and any inline frames
    fn symbols_for_pc(&self, pc: VirtAddr) -> Result<Vec<ResolvedSymbol>>;
}

fn _assert_obj_safe(_: &dyn RangeExtractor) {}

pub mod dwarf;
pub mod go;
pub mod multi;
pub mod obj;
