//go:build arm64

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package golabels // import "go.opentelemetry.io/ebpf-profiler/interpreter/golabels"

import (
	"fmt"

	"go.opentelemetry.io/ebpf-profiler/armhelpers"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	"go.opentelemetry.io/ebpf-profiler/libpf/pfelf"
	"go.opentelemetry.io/ebpf-profiler/nativeunwind/elfunwindinfo"
	"golang.org/x/arch/arm64/arm64asm"
)

// https://github.com/golang/go/blob/6885bad7dd86880be/src/runtime/tls_arm64.s#L11
//
//	Get's compiled into:
//	0x000000000007f260 <+0>:     adrp    x27, 0x1c2000 <runtime.mheap_+101440>
//	0x000000000007f264 <+4>:     ldrsb   x0, [x27, #284]
//	0x000000000007f268 <+8>:     cbz     x0, 0x7f278 <runtime.load_g+24>
//	0x000000000007f26c <+12>:    mrs     x0, tpidr_el0
//	0x000000000007f270 <+16>:    mov     x27, #0x30                      // #48
//	0x000000000007f274 <+20>:    ldr     x28, [x0, x27]
//	0x000000000007f278 <+24>:    ret
//
// And, when compiled with -buildmode=pie:
//
//	0x00000000000c2290 <+0>:	adrp	x27, 0x2ca000 <runtime.itabTableInit+3072>
//	0x00000000000c2294 <+4>:	ldrsb	x0, [x27, #1766]
//	0x00000000000c2298 <+8>:	cbz	x0, 0xc22ac <runtime.load_g+28>
//	0x00000000000c229c <+12>:	mrs	x0, tpidr_el0
//	0x00000000000c22a0 <+16>:	movz	x27, #0x0, lsl #16
//	0x00000000000c22a4 <+20>:	movk	x27, #0x10
//	0x00000000000c22a8 <+24>:	ldr	x28, [x0, x27]
//	0x00000000000c22ac <+28>:	ret
func extractTLSGOffset(f *pfelf.File) (int32, error) {
	iscgo, err := f.IsCgoEnabled()
	if err != nil || !iscgo {
		return 0, err
	}

	pclntab, err := elfunwindinfo.NewGopclntab(f)
	if err != nil {
		return 0, err
	}
	defer pclntab.Close()

	symbolName := "runtime.load_g.abi0"
	sym, err := pclntab.LookupSymbol(libpf.SymbolName(symbolName))
	if err != nil {
		// Use runtime.load_g as backup, if we can not identify
		// runtime.load_g.abi0. This can happen, if runtime.load_g.abi0
		// is inlined into runtime.load_g.
		symbolName := "runtime.load_g"
		sym, err = pclntab.LookupSymbol(libpf.SymbolName(symbolName))
		if err != nil {
			return 0, err
		}
	}
	b, err := f.VirtualMemory(int64(sym.Address), 32, 32)
	if err != nil {
		return 0, err
	}
	for ; len(b) > 0; b = b[4:] {
		i, err := arm64asm.Decode(b)
		if err != nil {
			return 0, err
		}
		switch i.Op {
		case arm64asm.MOV:
			imm, ok := i.Args[1].(arm64asm.Imm64)
			if ok {
				return int32(imm.Imm), nil
			}
		case arm64asm.MOVK:
			// when compiled with -buildmode=pie, mov instruction is split into two instructions: movz and movk
			// movz is used to zero the register and set bits 16-31, while movk is used to set the lower 16 bits:
			// movz x27, #0x0, lsl #16
			// movk x27, #0x10
			// For now, we'll just decode the immediate value from the movk instruction since the one from the movz
			// instruction seems to always be 0.
			imm, ok := armhelpers.DecodeImmediate(i.Args[1])
			if ok {
				return int32(imm), nil
			}
		}
	}
	return 0, fmt.Errorf("symbol '%s': %w", symbolName, errDecodeSymbol)
}
