// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package php // import "go.opentelemetry.io/ebpf-profiler/interpreter/php"

import (
	"errors"
	"fmt"

	ah "go.opentelemetry.io/ebpf-profiler/armhelpers"
	"go.opentelemetry.io/ebpf-profiler/libpf"
	aa "golang.org/x/arch/arm64/arm64asm"
)

// retrieveZendVMKindARM reads the code blob and recovers
// the type of the PHP VM that is used by this process.
func retrieveZendVMKindARM(code []byte) (uint, error) {
	// Here we need to decode assembly that looks like this:
	//
	// mov w0, ## constant
	// ret
	//
	// This means all we need to do is look at movs into w0

	// If the implementation isn't as described above, we should bail out. This could happen if the
	// implementation of zend_vm_kind changes. We thus only allow two instructions to be processed.
	maxOffs := min(len(code), 8)

	for offs := 0; offs < maxOffs; offs += 4 {
		inst, err := aa.Decode(code[offs:])
		if err != nil {
			return 0, fmt.Errorf("could not decode instruction at %d"+
				"in the given code blob", offs)
		}

		// We only care about writes into w0
		dest, ok := ah.Xreg2num(inst.Args[0])
		if dest != 0 || !ok {
			continue
		}

		if inst.Op == aa.MOV {
			val, ok := ah.DecodeImmediate(inst.Args[1])
			if !ok {
				break
			}
			if val > ZEND_VM_KIND_HYBRID {
				return 0, fmt.Errorf("zend_vm_kind has an invalid value %d", val)
			}
			return uint(val), nil
		}
	}

	// If we haven't already returned then clearly we're in an error state.
	return 0, errors.New("did not find a mov into w0 in the given code blob")
}

// retrieveExecuteExJumpLabelAddressARM reads the code blob and returns
// the address of the jump label for any JIT code called from execute_ex. Since all JIT
// code is ultimately called from execute_ex, this is the same as returning the return address
// for all JIT code.
func retrieveExecuteExJumpLabelAddressARM(
	code []byte, addrBase libpf.SymbolValue) (libpf.SymbolValue, error) {
	// Here we're looking for the first unrestricted jump in the execute_ex function
	// The reasons for this are given in the php8 unwinding document, but essentially we
	// heuristically found out that the php JIT code gets jumped into using GCC's "labels as
	// values" feature
	//
	// In assembly terms this looks something like this:
	//
	// xxx - 4: ...
	// xxx    : br x0
	// xxx + 4: ...     <---- This is the return address we care about.
	//
	// The heuristic is that the first br we encounter is the jump to the JIT code. This is because
	// (in theory) the first unrestricted goto in the php interpreter loop is the jump to the
	// handler for the particular zend_op.
	//
	// NOTE: we can't strengthen this check by also checking the register: PHP is __meant__ to store
	// the handler pointer in x28, but this is routinely and systematically ignored by compilers.

	for offs := 0; offs < len(code); offs += 4 {
		inst, err := aa.Decode(code[offs:])
		if err != nil {
			return libpf.SymbolValueInvalid,
				fmt.Errorf("could not decode the instruction at %d"+
					"in the given code blob", offs)
		}

		// We only care about br instructions
		if inst.Op == aa.BR && offs+4 < len(code) {
			// The length check is enough to make sure this is
			// a valid address.
			return libpf.SymbolValue(offs+4) + addrBase, nil
		}
	}
	return libpf.SymbolValueInvalid, errors.New("did not find a BR in the given code blob")
}

// retrieveJITBufferPtrARM reads the code blob and returns a pointer to the JIT buffer used by
// PHP (called "dasm_buf" in the PHP source).
func retrieveJITBufferPtrARM(code []byte, addrBase libpf.SymbolValue) (
	dasmBuf libpf.SymbolValue, dasmSize libpf.SymbolValue, err error) {
	// The code for recovering the JIT buffer is a little bit more involved on ARM than on x86.
	//
	// The idea is still the same: we're looking for a ldr into x0 in preparation for a function
	// call. Unfortunately, the Go disassembler makes it hard to do this sort of thing, so we need
	// to track the offsets by hand so that we can recover the address.
	//
	// For example, this is a likely assembly snippet:
	//
	// adrp x1, 0xfffff55aa000
	// add  x1, x1, #0xf8
	// ...
	// ldr x0, [x1, #840]
	// ldr x1, [x1, #850]
	//
	// Given that x0 depends on x1 we need to track the instructions
	// that are issued and then produce the correct offset at the end.
	//
	// We also assume that the first BL we encounter is the one we care about.
	// This is because the first call inside zend_jit_protect is a call to mprotect.
	var regOffset [32]int64

	bufRetVal := libpf.SymbolValueInvalid
	sizeRetVal := libpf.SymbolValueInvalid

	for offs := 0; offs < len(code); offs += 4 {
		inst, err := aa.Decode(code[offs:])
		if err != nil {
			return libpf.SymbolValueInvalid,
				libpf.SymbolValueInvalid,
				fmt.Errorf("could not decode instruction at %d"+
					"in the given code blob", offs)
		}

		if inst.Op == aa.BL && bufRetVal != libpf.SymbolValueInvalid &&
			sizeRetVal != libpf.SymbolValueInvalid {
			return bufRetVal, sizeRetVal, nil
		}

		// We only care about writes into xn/wn registers.
		dest, ok := ah.Xreg2num(inst.Args[0])
		if !ok {
			continue
		}

		switch inst.Op {
		case aa.ADD:
			a2, ok := ah.DecodeImmediate(inst.Args[2])
			if !ok {
				break
			}

			regOffset[dest] += a2
		case aa.ADRP:
			// The offset here is a PCRel, so we
			// can just recover it directly
			// Note that GDB lies to you here: it will give you
			// a different value in the ASM listing compared to the
			// disassembler
			a2, ok := ah.DecodeImmediate(inst.Args[1])
			if !ok {
				break
			}

			// The instruction specifies that this value needs to
			// shifted about before being added to the PC.
			pc := int64(addrBase) + int64(offs)
			regOffset[dest] = ((pc + a2) >> 12) << 12
		case aa.LDR:
			m, ok := inst.Args[1].(aa.MemImmediate)
			if !ok {
				break
			}

			val, ok := ah.DecodeImmediate(m)
			if !ok {
				break
			}

			src, ok := ah.Xreg2num(m.Base)
			if !ok {
				break
			}

			// If we're writing to x0/x1 then these are potentially interesting
			// values for us, so we'll recover them
			switch dest {
			case 0:
				bufRetVal = libpf.SymbolValue(regOffset[src] + val)
			case 1:
				sizeRetVal = libpf.SymbolValue(regOffset[src] + val)
			}
		}
	}

	return libpf.SymbolValueInvalid, libpf.SymbolValueInvalid,
		errors.New("did not find a BL instruction in the given code blob")
}
