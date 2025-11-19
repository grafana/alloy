// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

use crate::{StatusCode, SymblibSlice, SymblibString};
use symblib::symbconv;
use symblib::symbconv::PointResolver;
use symblib::VirtAddr;

#[repr(C)]
#[derive(Debug)]
pub struct SymblibResolvedSymbol {
    pub start_addr: VirtAddr,
    pub function_name: SymblibString,
    pub file_name: SymblibString, // may be empty
    pub line_number: u32,         // 0 = unknown
}

impl From<symbconv::ResolvedSymbol> for SymblibResolvedSymbol {
    fn from(sym: symbconv::ResolvedSymbol) -> Self {
        Self {
            start_addr: sym.start_addr,
            function_name: sym.function_name.into(),
            file_name: sym.file_name.unwrap_or("".to_string()).into(),
            line_number: sym.line_number.unwrap_or(0),
        }
    }
}

#[repr(C)]
pub struct SymblibPointResolver {
    inner: Box<dyn PointResolver + Send>,
}

impl SymblibPointResolver {
    pub fn new(resolver: Box<dyn PointResolver + Send>) -> Self {
        Self { inner: resolver }
    }
}

#[no_mangle]
pub extern "C" fn symblib_point_resolver_symbols_for_pc(
    resolver: &SymblibPointResolver,
    pc: VirtAddr,
    out_symbols: *mut *mut SymblibSlice<SymblibResolvedSymbol>,
) -> StatusCode {
    let symbols: Vec<_> = match resolver.inner.symbols_for_pc(pc) {
        Ok(syms) => syms.into_iter().map(Into::into).collect(),
        Err(e) => return StatusCode::from(e),
    };

    unsafe {
        *out_symbols = Box::into_raw(Box::new(symbols.into()));
    }
    StatusCode::Ok
}

#[no_mangle]
pub extern "C" fn symblib_slice_symblibresolved_symbol_free(
    slice: *mut SymblibSlice<SymblibResolvedSymbol>,
) {
    if !slice.is_null() {
        unsafe {
            drop(Box::from_raw(slice));
        }
    }
}
