//go:build amd64

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package golabels // import "go.opentelemetry.io/ebpf-profiler/interpreter/golabels"

import (
	"fmt"

	"go.opentelemetry.io/ebpf-profiler/asm/amd"
	e "go.opentelemetry.io/ebpf-profiler/asm/expression"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/nativeunwind/elfunwindinfo"
	"golang.org/x/arch/x86/x86asm"
)

// Most normal amd64 Go binaries use -8 as offset into TLS space for
// storing the current g but "static" binaries it ends up as -80. There
// may be dynamic relocating going on so just read it from a known
// symbol if possible.
func extractTLSGOffset(f *pfelf.File) (int32, error) {
	pclntab, err := elfunwindinfo.NewGopclntab(f)
	if err != nil {
		return 0, err
	}
	defer pclntab.Close()

	symbolName := "runtime.stackcheck"

	// Dump of assembler code for function runtime.stackcheck:
	// 0x0000000000470080 <+0>:     mov    %fs:0xfffffffffffffff8,%rax
	// Binaries built with -buildmode=pie have a different assembly code for stackcheck with 2 movs:
	//  0x00000000007ec320 <+0>:	mov    $0xfffffffffffffff8,%rcx
	//  0x00000000007ec327 <+7>:	mov    %fs:(%rcx),%rax
	sym, err := pclntab.LookupSymbol(libpf.SymbolName(symbolName))
	if err != nil {
		return 0, err
	}

	sz := int(min(sym.Size, 128))
	code, err := f.VirtualMemory(int64(sym.Address), sz, sz)
	if err != nil {
		return 0, err
	}

	offset := e.NewImmediateCapture("offset")
	it := amd.NewInterpreterWithCode(code)
	for {
		op, err := it.Step()
		if err != nil {
			break
		}
		if op.Op != x86asm.MOV {
			continue
		}
		mem, ok := op.Args[1].(x86asm.Mem)
		if !ok || mem.Segment != x86asm.FS {
			continue
		}
		// If the base is 0, it means the offset is directly in the register:
		// 0x0000000000470080 <+0>:     mov    %fs:0xfffffffffffffff8,%rax
		if mem.Base == 0 {
			return int32(mem.Disp), nil
		}
		// Otherwise, the offset is in the register:
		// 0x00000000007ec320 <+0>:	mov    $0xfffffffffffffff8,%rcx
		// 0x00000000007ec327 <+7>:	mov    %fs:(%rcx),%rax
		// Check if the register value was set with an immediate value in a previous instruction
		// and if so, use that value as the offset.
		actual := it.Regs.GetX86(mem.Base)
		if actual.Match(offset) {
			return int32(offset.CapturedValue()), nil
		}
	}
	return -8, fmt.Errorf("symbol '%s': %w", symbolName, errDecodeSymbol)
}
