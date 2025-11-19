// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#ifndef SYMBLIB_H
#define SYMBLIB_H

#include <stddef.h>
#include <stdint.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

typedef enum SymblibStatus {
    SYMBLIB_OK = 0,
    SYMBLIB_ERR_IOMISC = 1,
    SYMBLIB_ERR_IOFILENOTFOUND = 2,
    SYMBLIB_ERR_OBJFILE = 3,
    SYMBLIB_ERR_DWARF = 4,
    SYMBLIB_ERR_SYMBCONV = 5,
    SYMBLIB_ERR_RETPAD = 6,
    SYMBLIB_ERR_BADUTF8 = 7,
    SYMBLIB_ERR_ALREADYCLOSED = 8,
} SymblibStatus;

// Opaque handle to a return pad extractor.
typedef struct SymblibRetPadExtractor SymblibRetPadExtractor;

// Rust managed string.
typedef const char* SymblibString;

// Array of objects.
typedef struct {
    // Pointer to the first item in the slice.
    //
    // May or may not be NULL if `len == 0`: don't rely on it.
    const void* data;

    // Number of entries in the slice.
    size_t len;
} SymblibSlice;

// Entry in the return pad inline trace.
//
// See symbfile.proto for details.
typedef struct {
    SymblibString func; // never null
    SymblibString file; // may be null
    uint32_t line;      // 0 = unknown
} SymblibReturnPadEntry;

// Symbol info for a return pad.
//
// See symbfile.proto for details.
typedef struct {
    uint64_t elf_va;
    SymblibSlice/*<SymblibReturnPadEntry>*/ entries;
} SymblibReturnPad;

// Symbol info for a PC range.
//
// See symbfile.proto for details.
typedef struct {
    uint64_t elf_va;
    uint32_t length;
    SymblibString func;
    SymblibString file;
    SymblibString call_file;
    uint32_t call_line;
    uint32_t depth;
    SymblibSlice/*<SymblibLineTableEntry>*/ line_table;
    // Omitted internal Rust-specific field rust_range
} SymblibRange;

// Entry in a range's line table.
//
// See symbfile.proto for details.
typedef struct {
    uint32_t offset;
    uint32_t line_number;
} SymblibLineTableEntry;

// Visitor callback for extracted ranges.
//
// The range is **borrowed** to the callee and the pointer is only valid for
// the duration of the visitor call. Returning an error will abort further
// execution and return early.
typedef SymblibStatus (*SymblibRangeVisitor)(void* user_data, const SymblibRange*);

// Visitor callback for return pads.
//
// The return pad is **borrowed** to the callee and the pointer is only valid
// for the duration of the visitor call. Returning an error will abort further
// execution and return early.
typedef SymblibStatus (*SymblibRetPadVisitor)(void* user_data, const SymblibReturnPad*);

// Extract ranges from an executable.
//
// This creates a range extractor with all supported debug symbol formats. The
// extractor is then run to completion and the visitor is invoked for every
// range that is found in the executable. The user_data pointer is passed to
// the visitor untouched and may be NULL.
extern SymblibStatus symblib_rangeextr(
    const char* executable,
    bool follow_alt_link,
    SymblibRangeVisitor visitor,
    void* user_data
);

// Create a new return pad extractor.
//
// The instance must be freed via a call to `symblib_retpadextr_free`.
extern SymblibStatus symblib_retpadextr_new(
    const char* executable,
    SymblibRetPadExtractor** extr
);

// Submit a new range to the return pad extractor.
//
// The callback may be invoked 0..n times for each range submitted. Processing
// is happening asynchronously in the background: there is no guarantee that
// the return pads passed to the visitor at each call correspond to the range
// that was just submitted.
//
// The user_data pointer is passed to the visitor untouched and may be NULL.
//
// Once all ranges have been submitted, call this function with a `NULL` range
// once to indicate this, forcing all remaining buffered return pads to be
// flushed.
extern SymblibStatus symblib_retpadextr_submit(
    SymblibRetPadExtractor* extr,
    const SymblibRange* range,
    SymblibRetPadVisitor visitor,
    void* user_data
);

// Frees a return pad extractor.
extern void symblib_retpadextr_free(SymblibRetPadExtractor* extr);

// Opaque handle to SymblibPointResolver.
typedef struct SymblibPointResolver SymblibPointResolver;

// Creates a new SymblibPointResolver.
extern SymblibStatus symblib_goruntime_new(
    const char* executable,
    SymblibPointResolver** runtime // out arg
);

// Frees a SymblibPointResolver.
extern void symblib_goruntime_free(SymblibPointResolver* runtime);

// Contains information about a symbol and its origin.
typedef struct SymblibResolvedSymbol {
    uint64_t start_addr;
    SymblibString function_name;
    SymblibString file_name;
    uint32_t line_number;
} SymblibResolvedSymbol;

// Enveloping struct that contains len number of symbols in data.
typedef struct SymblibSlice_SymblibResolvedSymbol {
    const SymblibResolvedSymbol* data;
    size_t len;
} SymblibSlice_SymblibResolvedSymbol;

// Single point lookup for pc using SymblibPointResolver.
SymblibStatus symblib_point_resolver_symbols_for_pc(
    const SymblibPointResolver* resolver,
    uint64_t pc,
    SymblibSlice_SymblibResolvedSymbol** symbols // out arg
);

// Frees a SymblibSlice_SymblibResolvedSymbol.
void symblib_slice_symblibresolved_symbol_free(
    SymblibSlice_SymblibResolvedSymbol* slice
);

#ifdef __cplusplus
}
#endif

#endif // SYMBLIB_H
