// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tpbase // import "go.opentelemetry.io/ebpf-profiler/tpbase"

import (
	"errors"
	"fmt"

	ah "go.opentelemetry.io/ebpf-profiler/armhelpers"
	"go.opentelemetry.io/ebpf-profiler/stringutil"
	aa "golang.org/x/arch/arm64/arm64asm"
)

const (
	Unspec int = iota
	TSDBase
	TSDElementBase
	TSDIndex
	TSDValue
	TSDConstant
)

type regState struct {
	status     int
	offset     int64
	multiplier int
	indirect   bool
}

func extractTSDInfoARM(code []byte) (TSDInfo, error) {
	// This tries to extract offsetof(struct pthread, tsd).
	// The analyzed code is pthread_getspecific, and should work on glibc and musl.
	// See test cases for example assembly. The strategy is to find "MRS xx, tpidr_el0"
	// instruction as loading something relative to "struct pthread". It is
	// then tracked against first argument to find the exact offset and multiplier
	// to address the TSD array.

	// Start tracking of X0
	var regs [32]regState

	regs[0].status = TSDIndex
	regs[0].multiplier = 1
	resetReg := int(-1)

	for offs := 0; offs < len(code); offs += 4 {
		if resetReg >= 0 {
			// Reset register state if something unsupported happens on it
			regs[resetReg] = regState{status: Unspec}
		}

		inst, err := aa.Decode(code[offs:])
		if err != nil {
			continue
		}
		if inst.Op == aa.RET {
			break
		}

		destReg, ok := ah.Xreg2num(inst.Args[0])
		if !ok {
			continue
		}

		resetReg = destReg
		switch inst.Op {
		case aa.MOV:
			switch val := inst.Args[1].(type) {
			case aa.Imm64:
				regs[destReg] = regState{
					status:     TSDConstant,
					offset:     int64(val.Imm),
					multiplier: 1,
				}
			case aa.Imm:
				regs[destReg] = regState{
					status:     TSDConstant,
					offset:     int64(val.Imm),
					multiplier: 1,
				}
			default:
				// Track register moves
				srcReg, ok := ah.Xreg2num(inst.Args[1])
				if !ok {
					continue
				}
				regs[destReg] = regs[srcReg]
			}
		case aa.MRS:
			// MRS X1, S3_3_C13_C0_2
			if inst.Args[1].String() == "S3_3_C13_C0_2" {
				regs[destReg] = regState{
					status:     TSDBase,
					multiplier: 1,
				}
			}
		case aa.LDUR:
			// LDUR X1, [X1,#-88]
			m, ok := inst.Args[1].(aa.MemImmediate)
			if !ok {
				continue
			}
			srcReg, ok := ah.Xreg2num(m.Base)
			if !ok {
				continue
			}
			if regs[srcReg].status == TSDBase {
				imm, ok := ah.DecodeImmediate(m)
				if !ok {
					continue
				}
				regs[destReg] = regState{
					status:     TSDBase,
					offset:     regs[srcReg].offset + imm,
					multiplier: regs[srcReg].multiplier,
					indirect:   true,
				}
			} else {
				continue
			}
		case aa.LDR:
			switch m := inst.Args[1].(type) {
			case aa.MemExtend:
				// LDR X0, [X1,W0,UXTW #3]
				srcReg, ok := ah.Xreg2num(m.Base)
				if !ok {
					continue
				}
				srcIndex, ok := ah.Xreg2num(m.Index)
				if !ok {
					continue
				}
				if regs[srcReg].status == TSDBase && regs[srcIndex].status == TSDIndex {
					regs[destReg] = regState{
						status:     TSDValue,
						offset:     regs[srcReg].offset + (regs[srcIndex].offset << m.Amount),
						multiplier: regs[srcReg].multiplier << m.Amount,
						indirect:   regs[srcReg].indirect,
					}
				} else {
					continue
				}
			case aa.MemImmediate:
				// ldr x0, [x2, #8]
				srcReg, ok := ah.Xreg2num(m.Base)
				if !ok {
					continue
				}
				if regs[srcReg].status == TSDElementBase {
					i, ok := ah.DecodeImmediate(m)
					if !ok {
						continue
					}
					regs[destReg] = regState{
						status:     TSDValue,
						offset:     regs[srcReg].offset + i,
						multiplier: regs[srcReg].multiplier,
						indirect:   regs[srcReg].indirect,
					}
				} else {
					continue
				}
			}
		case aa.UBFIZ:
			// UBFIZ X0, X1, #4, #32
			srcReg, ok := ah.Xreg2num(inst.Args[1])
			if !ok {
				continue
			}
			if regs[srcReg].status == TSDIndex {
				i, ok := inst.Args[2].(aa.Imm)
				if !ok {
					continue
				}
				regs[destReg] = regState{
					status:     TSDIndex,
					offset:     regs[srcReg].offset << i.Imm,
					multiplier: regs[srcReg].multiplier << i.Imm,
				}
			}
		case aa.ADD:
			srcReg, ok := ah.Xreg2num(inst.Args[1])
			if !ok {
				continue
			}
			switch a2 := inst.Args[2].(type) {
			case aa.ImmShift:
				i, ok := ah.DecodeImmediate(a2)
				if !ok {
					continue
				}
				regs[destReg] = regs[srcReg]
				regs[destReg].offset += i
			case aa.RegExtshiftAmount:
				regStr := inst.Args[2].String()
				shift := int(0)
				var fields [2]string
				if stringutil.SplitN(regStr, ",", fields[:]) == 2 {
					regStr = fields[0]
					n, err := fmt.Sscanf(fields[1], " LSL #%v", &shift)
					if n != 1 || err != nil {
						n, err := fmt.Sscanf(fields[1], " UXTW #%v", &shift)
						if n != 1 || err != nil {
							continue
						}
					}
				}
				reg, ok := ah.DecodeRegister(regStr)
				if !ok {
					continue
				}
				srcReg2, ok := ah.Xreg2num(reg)
				if !ok {
					continue
				}
				if regs[srcReg].status == TSDBase && regs[srcReg2].status == TSDIndex {
					regs[destReg] = regState{
						status:     TSDElementBase,
						offset:     regs[srcReg].offset + regs[srcReg2].offset<<shift,
						multiplier: regs[srcReg2].multiplier << shift,
						indirect:   regs[srcReg].indirect,
					}
				} else if regs[srcReg].status == TSDConstant && regs[srcReg2].status == TSDIndex {
					regs[destReg] = regState{
						status:     TSDIndex,
						offset:     regs[srcReg].offset + regs[srcReg2].offset<<shift,
						multiplier: regs[srcReg2].multiplier << shift,
					}
				} else {
					continue
				}
			}
		case aa.SUB:
			srcReg, ok := ah.Xreg2num(inst.Args[1])
			if !ok {
				continue
			}
			if regs[srcReg].status != Unspec {
				i, ok := ah.DecodeImmediate(inst.Args[2])
				if !ok {
					continue
				}
				regs[destReg] = regs[srcReg]
				regs[destReg].offset -= i
			} else {
				continue
			}
		case aa.CMP, aa.CBZ:
			// Opcode with no affect on first argument.
			// Noop to exit switch without default continue.
		default:
			continue
		}
		resetReg = -1
	}

	if regs[0].status != TSDValue {
		return TSDInfo{}, errors.New("libc data not found")
	}

	indirect := uint8(0)
	if regs[0].indirect {
		indirect = 1
	}
	return TSDInfo{
		Offset:     int16(regs[0].offset),
		Multiplier: uint8(regs[0].multiplier),
		Indirect:   indirect,
	}, nil
}
