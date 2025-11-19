//go:build arm64

// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package maccess // import "go.opentelemetry.io/ebpf-profiler/maccess"

import (
	"errors"

	ah "go.opentelemetry.io/ebpf-profiler/armhelpers"
	aa "golang.org/x/arch/arm64/arm64asm"
)

// Various constants to mark or check for a specific step.
const (
	stepNone = 0         // No instruction marker
	stepMov  = 1 << iota // Marker for the MOV instruction
	stepCmp              // Marker for the CMP instruction
	stepB                // Marker for the B instruction
)

// CopyFromUserNoFaultIsPatched looks for a set of assembly instructions, that indicate
// that copy_from_user_nofault was patched.
// nmi_uaccess_okay, that was added with [0] to check memory access, is a specific function
// for x86 and returns always TRUE [1] on other architectures like arm64. So the compiler
// optimizes this function as the result of the function is known at compile time.
//
// [0] https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=d319f344561de23e810515d109c7278919bff7b0
// [1] https://github.com/torvalds/linux/blob/8bc9e6515183935fa0cccaf67455c439afe4982b/include/asm-generic/tlb.h#L26
//
//nolint:lll
func CopyFromUserNoFaultIsPatched(codeblob []byte, _, _ uint64) (bool, error) {
	if len(codeblob) == 0 {
		return false, errors.New("empty code blob")
	}

	// With the patch [0] of copy_from_user_nofault, access_ok() got replaced with __access_ok() [1].
	// __access_ok() is an inlined function and returns '(size <= limit) && (addr <= (limit - size))' [2].
	// This function tries to identify the following sequence of instructions in the codeblob:
	// MOV X2, #0x1000000000000
	// CMP X19, X2
	// B HI, .+0x14
	// SUB X2, X2, X19
	//
	// [0] https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git/commit/?id=d319f344561de23e810515d109c7278919bff7b0
	// [1] https://github.com/torvalds/linux/blob/1c41041124bd14dd6610da256a3da4e5b74ce6b1/include/asm-generic/access_ok.h#L20-L41
	// [2] https://github.com/torvalds/linux/blob/1c41041124bd14dd6610da256a3da4e5b74ce6b1/include/asm-generic/access_ok.h#L40

	// In the set of expected assembly instructions, one argument register is used by all instructions.
	var trackedReg = -1
	// Statemachine to keep track of the previously encountered and expected instructions.
	var expectedInstructionTracker = stepNone

	for offs := 0; offs < len(codeblob); offs += 4 {
		inst, err := aa.Decode(codeblob[offs:])
		if err != nil {
			break
		}
		switch inst.Op {
		case aa.MOV:
			// Check if an immediate 64-bit value is moved as part of the instruction.
			// From the instruction '(size <= limit) && (addr <= (limit - size))', limit comes
			// down to TASK_SIZE_MAX, which is usually TASK_SIZE, and is known at compile time.
			if v, ok := inst.Args[1].(aa.Imm64); !ok {
				continue
			} else if ok && v.Imm == 0xfffffffffffffff2 {
				// If the immediate value is -EFAULT, ignore this move instruction.
				// -EFAULT is the returned error code by copy_from_user_nofault for
				// error cases.
				continue
			}

			if r, ok := inst.Args[0].(aa.Reg); ok {
				if regN, ok := ah.Xreg2num(r); ok &&
					expectedInstructionTracker == stepNone {
					trackedReg = regN
					expectedInstructionTracker ^= stepMov
					continue
				}
			}
			// Reset trackers as the immediate value is not moved into a register
			// as expected.
			trackedReg = -1
			expectedInstructionTracker = stepNone
		case aa.CMP:
			if regN, ok := ah.Xreg2num(inst.Args[0]); ok &&
				expectedInstructionTracker&stepMov == stepMov {
				if trackedReg == regN {
					expectedInstructionTracker ^= stepCmp
					continue
				}
			}
			if regN, ok := ah.Xreg2num(inst.Args[1]); ok {
				if trackedReg == regN {
					expectedInstructionTracker ^= stepCmp
					continue
				}
			}
			// trackedReg is not used in the CMP instruction.
			trackedReg = -1
			expectedInstructionTracker = stepNone
		case aa.B:
			if cond, ok := inst.Args[0].(aa.Cond); ok &&
				expectedInstructionTracker&(stepMov|stepCmp) == (stepMov|stepCmp) {
				if cond.Value == 8 {
					// Conditional branching with flag check: C = 1 & Z = 0
					// This is expected after a CMP instruction, which sets flags
					// - Z
					//   1 if CMP result is zero, indicating an equal result.
					//   0 otherwise
					// - C
					//   1 if CMP results in carry condition, like unsigned overflow
					//   0 otherwise
					expectedInstructionTracker ^= stepB
					continue
				}
			}
			// trackedReg is not used in the B instruction.
			trackedReg = -1
			expectedInstructionTracker = stepNone
		case aa.SUB:
			// If the minuend of the subtraction is trackedReg, copy_from_user_nofault seems
			// to be patched.
			if regN, ok := ah.Xreg2num(inst.Args[1]); ok {
				if trackedReg == regN && expectedInstructionTracker == (stepMov|stepCmp|stepB) {
					return true, nil
				}
			}
			// trackedReg is not used in the SUB instruction.
			trackedReg = -1
			expectedInstructionTracker = stepNone
		case aa.TBNZ:
			// Safeguard:
			// In unpatched versions of copy_from_user_nofault the 'Test bit and Branch if Nonzero'
			// instruction can be found. This instruction originates from the inlined call of
			// access_ok(). In patched versions of copy_from_user_nofault, access_ok() got replaced
			// with __access_ok().
			return false, nil
		default:
			trackedReg = -1
			expectedInstructionTracker = stepNone
		}
	}

	return false, nil
}
