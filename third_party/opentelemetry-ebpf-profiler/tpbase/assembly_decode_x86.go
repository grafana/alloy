// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tpbase // import "go.opentelemetry.io/ebpf-profiler/tpbase"

import (
	"bytes"
	"encoding/binary"
	"errors"

	"go.opentelemetry.io/ebpf-profiler/asm/amd"
	e "go.opentelemetry.io/ebpf-profiler/asm/expression"
	"golang.org/x/arch/x86/x86asm"
)

func getAnalyzersX86() []Analyzer {
	return []Analyzer{
		{"x86_fsbase_write_task", analyzefsbaseWriteTaskX86},
		{"aout_dump_debugregs", analyzeAoutDumpDebugregsX86},
	}
}

// analyzeAoutDumpDebugregsX86 looks at the assembly of the `aout_dump_debugregs` function in the
// kernel in order to compute the offset of `fsbase` into `task_struct`.
func analyzeAoutDumpDebugregsX86(code []byte) (uint32, error) {
	if len(code) == 0 {
		return 0, errors.New("empty code blob passed to getFSBaseOffset")
	}
	it := amd.NewInterpreterWithCode(code)
	offset := e.NewImmediateCapture("offset")
	expected := e.Mem8(
		e.Add(
			e.MemWithSegment8(x86asm.GS, e.NewImmediateCapture("")),
			offset,
		),
	)
	for {
		op, err := it.Step()
		if err != nil {
			return 0, err
		}
		if op.Op != x86asm.MOV {
			continue
		}
		dst, ok := op.Args[0].(x86asm.Reg)
		if !ok {
			continue
		}
		actual := it.Regs.GetX86(dst)
		if actual.Match(expected) {
			res := int64(offset.CapturedValue()) - 2*8
			if res < 0 || res > 256*1024 {
				return 0, errors.New("failed to determine offset of fsbase")
			}
			return uint32(res), nil
		}
	}
}

// analyzefsbaseWriteTaskX86 looks at the assembly of the function x86_fsbase_write_task which
// is ideal because it only writes the argument to the fsbase function. We can get the fsbase
// offset directly from the assembly here. Available since kernel version 4.20.
func analyzefsbaseWriteTaskX86(code []byte) (uint32, error) {
	// Supported sequences (might be surrounded be additional code for the WARN_ONCE):
	//
	// 1) Alpine Linux (kernel 5.10+)
	//    48 89 b7 XX XX XX XX 	mov    %rsi,0xXXXXXXXX(%rdi)

	// No need to disassemble here, as it's highly unlikely the below machine code
	// matching approach would fail. Indeed, x86-64 calling conventions ensure that:
	// * %rdi is a pointer to a `task_struct` (first parameter)
	// * %rsi == fsbase value (second parameter)
	// the x86_fsbase_write_task function simply sets that task (from the first parameter) fsbase to
	// be equal to the second parameter.
	// See https://elixir.bootlin.com/linux/latest/source/arch/x86/kernel/process_64.c#L466
	idx := bytes.Index(code, []byte{0x48, 0x89, 0xb7})
	if idx == -1 || idx+7 > len(code) {
		return 0, errors.New("unexpected x86_fsbase_write_task (mov not found)")
	}
	offset := binary.LittleEndian.Uint32(code[idx+3:])
	return offset, nil
}
