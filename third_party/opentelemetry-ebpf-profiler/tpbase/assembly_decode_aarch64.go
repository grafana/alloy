// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tpbase // import "go.opentelemetry.io/ebpf-profiler/tpbase"

import (
	"errors"
	"fmt"

	ah "go.opentelemetry.io/ebpf-profiler/armhelpers"
	aa "golang.org/x/arch/arm64/arm64asm"
)

func getAnalyzersARM() []Analyzer {
	return []Analyzer{
		{"tls_set", analyzeTLSSetARM},
	}
}

// analyzeTLSSetARM looks at the assembly of the `tls_set` function in the
// kernel in order to compute the offset of `tp_value` into `task_struct`.
func analyzeTLSSetARM(code []byte) (uint32, error) {
	// This tries to extract offset of thread.uw.tp_value relative to
	// struct task_struct. The code analyzed comes from:
	// linux/arch/arm64/kernel/ptrace.c: tls_set(struct task_struct *target, ...) {
	// [...]
	//  unsigned long tls = target->thread.uw.tp_value;
	//
	// Anyalysis is based on the fact that 'target' is in X0 at the start, and early
	// in the assembly there is a direct load via this pointer. Because of reduced
	// instruction set, the pointer often gets moved to another register before the
	// load we are interested, so the arg []bool tracks which register is currently
	// holding the tracked pointer. Once a proper load is matched, the offset is
	// extracted from it.

	// Start tracking of X0
	var arg [32]bool
	arg[0] = true

	for offs := 0; offs < len(code); offs += 4 {
		inst, err := aa.Decode(code[offs:])
		if err != nil {
			break
		}
		if inst.Op == aa.B {
			break
		}

		switch inst.Op {
		case aa.MOV:
			// Track register moves
			destReg, ok := ah.Xreg2num(inst.Args[0])
			if !ok {
				continue
			}
			if srcReg, ok := ah.Xreg2num(inst.Args[1]); ok {
				arg[destReg] = arg[srcReg]
			}
		case aa.LDR:
			// Track loads with offset of the argument pointer we care
			m, ok := inst.Args[1].(aa.MemImmediate)
			if !ok {
				continue
			}
			var srcReg int
			if srcReg, ok = ah.Xreg2num(m.Base); !ok || !arg[srcReg] {
				continue
			}
			// FIXME: m.imm is not public, but should be.
			// https://github.com/golang/go/issues/51517
			imm, ok := ah.DecodeImmediate(m)
			if !ok {
				return 0, err
			}
			// Quick sanity check. Per example, the offset should
			// be under 4k. But allow some leeway.
			if imm < 64 || imm >= 65536 {
				return 0, fmt.Errorf("detected tpbase %#x looks invalid", imm)
			}
			return uint32(imm), nil
		default:
			// Reset register state if something unsupported happens on it
			if destReg, ok := ah.Xreg2num(inst.Args[0]); ok {
				arg[destReg] = false
			}
		}
	}

	return 0, errors.New("tp base not found")
}
