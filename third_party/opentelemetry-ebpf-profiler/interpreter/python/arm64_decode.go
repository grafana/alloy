// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package python // import "go.opentelemetry.io/ebpf-profiler/interpreter/python"

import (
	ah "go.opentelemetry.io/ebpf-profiler/armhelpers"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	aa "golang.org/x/arch/arm64/arm64asm"
)

// decodeStubArgumentARM64 disassembles arm64 code and decodes the assumed value
// of requested argument.
func decodeStubArgumentARM64(code []byte,
	addrBase libpf.SymbolValue) libpf.SymbolValue {
	const argNumber uint8 = 0
	// The concept is to track the latest load offset for all X0..X30 registers.
	// These registers are used as the function arguments. Once the first branch
	// instruction (function call/tail jump) is found, the state of the requested
	// argument register's offset is inspected and returned if found.
	// It is seen often that the load with offset happens to intermediate register
	// first, and is later moved to the argument register. Because of this, the
	// tracking requires extra effort between register moves etc.

	// PyEval_ReleaseLock (Amazon Linux /usr/lib64/libpython3.7m.so.1.0):
	// ADRP X0, .+0x148000
	// LDR X1, [X0,#1960]
	// ADD X2, X1, #0x5d8		1. X2's regOffset is 0x5d8 (the value we want)
	// LDR X0, [X2]			2. The argument register is loaded via X2
	// B .+0xfffffffffffffe88

	// PyGILState_GetThisThreadState (Amazon Linux /usr/lib64/libpython3.7m.so.1.0):
	// ADRP X0, .+0x251000
	// LDR X2, [X0,#1960]
	// LDR X1, [X2,#1512]
	// CBZ X1, .+0xc
	// ADD X0, X2, #0x5f0		1. X0's regOffset gets 0x5f0
	// B .+0xfffffffffffb92b4

	// PyGILState_GetThisThreadState (Debian 11 /usr/bin/python3):
	// ADRP X0, #0x907000
	// ADD  X2, X0, #0x880
	// ADD  X3, X2, #0x10
	// LDR  X1, [X2,#0x260]
	// CBZ  X1, loc_4740BC
	// LDR  W0, [X3,#0x25C] ; key
	// B    .pthread_getspecific

	// Storage for load offsets for each Xn register
	var regOffset [32]int64
	retValue := libpf.SymbolValueInvalid

	for offs := 0; offs < len(code); offs += 4 {
		inst, err := aa.Decode(code[offs:])
		if err != nil {
			return libpf.SymbolValueInvalid
		}
		if inst.Op == aa.B {
			return retValue
		}

		// Interested only on commands modifying Xn
		dest, ok := ah.Xreg2num(inst.Args[0])
		if !ok {
			continue
		}

		instOffset := int64(0)
		instRetval := libpf.SymbolValueInvalid
		switch inst.Op {
		case aa.ADD:
			a2, ok := ah.DecodeImmediate(inst.Args[2])
			if !ok {
				break
			}
			instOffset = a2
			instRetval = addrBase + libpf.SymbolValue(a2)
		case aa.LDR:
			m, ok := inst.Args[1].(aa.MemImmediate)
			if !ok {
				break
			}
			src, ok := ah.Xreg2num(m.Base)
			if !ok {
				break
			}
			imm, ok := ah.DecodeImmediate(inst.Args[1])
			if !ok {
				break
			}
			// FIXME: addressing mode not taken into account
			// because m.imm is not public, but needed.
			instRetval = addrBase + libpf.SymbolValue(regOffset[src]+imm)
		}
		regOffset[dest] = instOffset
		if dest == int(argNumber) {
			retValue = instRetval
		}
	}

	return libpf.SymbolValueInvalid
}
