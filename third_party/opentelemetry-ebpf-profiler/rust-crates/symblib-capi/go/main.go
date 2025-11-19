// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package main

/*
#cgo LDFLAGS: ${SRCDIR}/../../../target/release/libsymblib_capi.a
#cgo CFLAGS: -g -Wall
#include "../c/symblib.h"
#include <stdlib.h>

// Declare wrapper functions for linking.
SymblibStatus rangeVisitorWrapper(void* user_data, SymblibRange* range);
SymblibStatus retPadVisitorWrapper(void* user_data, SymblibReturnPad* ret_pad);
*/
import "C"
import (
	"fmt"
	"os"
	"unsafe"
)

//export retPadVisitorWrapper
func retPadVisitorWrapper(_ unsafe.Pointer, retPadPtr *C.SymblibReturnPad) C.SymblibStatus {
	// Process the return pad data
	elfVA := uint64(retPadPtr.elf_va)
	entriesCount := int(retPadPtr.entries.len)
	fmt.Printf("Return Pad: ELF VA: 0x%x, Entries: %d\n", elfVA, entriesCount)

	return C.SYMBLIB_OK
}

//export rangeVisitorWrapper
func rangeVisitorWrapper(userData unsafe.Pointer, rangePtr *C.SymblibRange) C.SymblibStatus {
	elfVA := uint64(rangePtr.elf_va)
	length := uint32(rangePtr.length)
	file := C.GoString(rangePtr.file)
	// cgo transforms the field func in SymblibRange to _func
	// as func is a reserved keyword in Go.
	function := C.GoString(rangePtr._func)

	fmt.Printf("Range: ELF VA: 0x%x, Length: %d, Function: %s  File: %s\n",
		elfVA, length, function, file)

	return C.symblib_retpadextr_submit(
		(*C.SymblibRetPadExtractor)(userData),
		rangePtr,
		C.SymblibRetPadVisitor(C.retPadVisitorWrapper),
		nil,
	)
}

func mainWithExitCode() int {
	// For the purpose of demonstration symbolize the executable themselves.
	executablePath := C.CString(os.Args[0])
	defer C.free(unsafe.Pointer(executablePath))

	// Initialize the global return pad extractor.
	// We use it in the range extractor visitor.
	var extractor *C.SymblibRetPadExtractor

	//nolint:gocritic
	status := C.symblib_retpadextr_new(executablePath, &extractor)
	if status != C.SYMBLIB_OK {
		fmt.Fprintf(os.Stderr, "Failed to create return pad extractor: %d\n", status)
		return 1
	}
	defer C.symblib_retpadextr_free(extractor)

	// Call the range extraction function with our visitor.
	status = C.symblib_rangeextr(
		executablePath,
		C.bool(true),
		C.SymblibRangeVisitor(C.rangeVisitorWrapper),
		unsafe.Pointer(extractor),
	)
	if status != C.SYMBLIB_OK {
		fmt.Fprintf(os.Stderr, "Failed to extract ranges: %d\n", status)
		return 1
	}

	// Notify the return pad extractor that we're done.
	status = C.symblib_retpadextr_submit(extractor, nil,
		C.SymblibRetPadVisitor(C.retPadVisitorWrapper), nil)
	if status != C.SYMBLIB_OK {
		fmt.Fprintf(os.Stderr, "Failed to notify retpad extractor: %d\n", status)
		return 1
	}

	fmt.Println("Ranges extracted successfully")
	return 0
}

func main() {
	os.Exit(mainWithExitCode())
}
