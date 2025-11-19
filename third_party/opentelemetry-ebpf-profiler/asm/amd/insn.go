// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package amd // import "go.opentelemetry.io/ebpf-profiler/asm/amd"
import (
	"bytes"

	"go.opentelemetry.io/ebpf-profiler/libpf"
	"golang.org/x/arch/x86/x86asm"
)

// https://www.felixcloutier.com/x86/endbr64
var opcodeEndBr64 = []byte{0xf3, 0x0f, 0x1e, 0xfa}

// DecodeSkippable decodes an instruction that we don't care much about and are going to skip,
// as golang.org/x/arch/x86/x86asm fails to decode it.
// The second returned argument is the size of the decoded instruction to skip.
func DecodeSkippable(code []byte) (ok bool, size int) {
	switch {
	case bytes.HasPrefix(code, opcodeEndBr64):
		return true, len(opcodeEndBr64)
	default:
		return false, 0
	}
}

// FindExternalJump decodes every instruction in the sym function and searches for
// a relative jump outside itself - to an address not covered by the sym.
// FindExternalJump returns the destination address of the relative jump outside the function or 0.
func FindExternalJump(code []byte, f *libpf.Symbol) (libpf.Address, error) {
	var (
		err  error
		inst x86asm.Inst
		rip  = int64(f.Address)
	)
	for len(code) > 0 {
		if ok, l := DecodeSkippable(code); ok {
			inst = x86asm.Inst{Op: x86asm.NOP, Len: l}
		} else {
			inst, err = x86asm.Decode(code, 64)
			if err != nil {
				return 0, err
			}
		}
		rip += int64(inst.Len)
		code = code[inst.Len:]
		if !isJump(inst.Op) {
			continue
		}
		if rel, ok := inst.Args[0].(x86asm.Rel); !ok {
			continue
		} else {
			dst := rip + int64(rel)
			if dst >= int64(f.Address) && dst < int64(f.Address)+int64(f.Size) {
				continue
			}
			return libpf.Address(dst), nil
		}
	}
	return 0, nil
}

func isJump(op x86asm.Op) bool {
	switch op {
	case x86asm.JA,
		x86asm.JAE,
		x86asm.JB,
		x86asm.JBE,
		x86asm.JCXZ,
		x86asm.JE,
		x86asm.JECXZ,
		x86asm.JG,
		x86asm.JGE,
		x86asm.JL,
		x86asm.JLE,
		x86asm.JMP,
		x86asm.JNE,
		x86asm.JNO,
		x86asm.JNP,
		x86asm.JNS,
		x86asm.JO,
		x86asm.JP,
		x86asm.JRCXZ,
		x86asm.JS:
		return true
	default:
		return false
	}
}
