// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

//! Translates object file (e.g. ELF) symbols into a range symbfile.

use super::{Error, RangeExtractor, RangeVisitor, Result, Stats};
use crate::{demangle, objfile, symbfile};

/// Extracts ranges from object file symbols.
pub struct Extractor<'obj> {
    obj: &'obj objfile::Reader<'obj>,
    source: objfile::SymbolSource,
}

impl<'obj> Extractor<'obj> {
    /// Create a new object file symbol extractor.
    pub fn new(obj: &'obj objfile::Reader<'obj>, source: objfile::SymbolSource) -> Self {
        Self { obj, source }
    }
}

impl<'obj> RangeExtractor for Extractor<'obj> {
    fn extract(&self, visitor: RangeVisitor<'_>) -> Result<Option<Stats>> {
        for sym in self.obj.function_symbols(self.source) {
            let rng = obj_symbol_to_range(&sym);
            visitor(rng).map_err(Error::Obj)?;
        }

        Ok(None)
    }
}

/// Translate an object file symbol to a range.
fn obj_symbol_to_range(sym: &objfile::Symbol<'_>) -> symbfile::Range {
    symbfile::Range {
        elf_va: sym.virt_addr,
        length: sym.length as u32,
        func: demangle::demangle(sym.name).into_owned(),
        file: None,
        call_file: None,
        call_line: None,
        depth: 0,
        line_table: Default::default(),
    }
}
