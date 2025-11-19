// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elfunwindinfo // import "go.opentelemetry.io/ebpf-profiler/nativeunwind/elfunwindinfo"

// ARM64 specific code for handling DWARF / stack delta extraction.
// The filename ends with `_aarch64` instead of `_arm64`, so that the code
// can be taken into account regardless of the target build platform.

import (
	"bytes"
	"debug/elf"
	"fmt"

	sdtypes "go.opentelemetry.io/ebpf-profiler/nativeunwind/stackdeltatypes"
	"go.opentelemetry.io/ebpf-profiler/support"
	"golang.org/x/arch/arm64/arm64asm"
)

const (
	// Aarch64 ABI
	armRegX0  uleb128 = 0
	armRegX1  uleb128 = 1
	armRegX2  uleb128 = 2
	armRegX3  uleb128 = 3
	armRegX4  uleb128 = 4
	armRegX5  uleb128 = 5
	armRegX6  uleb128 = 6
	armRegX7  uleb128 = 7
	armRegX8  uleb128 = 8
	armRegX9  uleb128 = 9
	armRegX10 uleb128 = 10
	armRegX11 uleb128 = 11
	armRegX12 uleb128 = 12
	armRegX13 uleb128 = 13
	armRegX14 uleb128 = 14
	armRegX15 uleb128 = 15
	armRegX16 uleb128 = 16
	armRegX17 uleb128 = 17
	armRegX18 uleb128 = 18
	armRegX19 uleb128 = 19
	armRegX20 uleb128 = 20
	armRegX21 uleb128 = 21
	armRegX22 uleb128 = 22
	armRegX23 uleb128 = 23
	armRegX24 uleb128 = 24
	armRegX25 uleb128 = 25
	armRegX26 uleb128 = 26
	armRegX27 uleb128 = 27
	armRegX28 uleb128 = 28
	armRegFP  uleb128 = 29
	armRegLR  uleb128 = 30
	armRegSP  uleb128 = 31
	armRegPC  uleb128 = 32

	armLastReg uleb128 = iota
)

// newVMRegsARM initializes the vmRegs structure for aarch64.
func newVMRegsARM() vmRegs {
	return vmRegs{
		arch: elf.EM_AARCH64,
		cfa:  vmReg{arch: elf.EM_AARCH64, reg: regUndefined},
		fp:   vmReg{arch: elf.EM_AARCH64, reg: regSame},
		ra:   vmReg{arch: elf.EM_AARCH64, reg: regSame},
	}
}

// getRegNameARM converts register index to a string describing the register
func getRegNameARM(reg uleb128) string {
	switch reg {
	case armRegFP:
		return "fp"
	case armRegLR:
		return "lr"
	case armRegSP:
		return "sp"
	case armRegPC:
		return "pc"
	default:
		if reg < armLastReg {
			return fmt.Sprintf("x%d", reg)
		}
		return fmt.Sprintf("?%d", reg)
	}
}

// regARM returns the address to ARM specific register in vmRegs
func (regs *vmRegs) regARM(ndx uleb128) *vmReg {
	switch ndx {
	case armRegFP:
		return &regs.fp
	case armRegLR:
		return &regs.ra
	default:
		return nil
	}
}

// getUnwindInfo ARM specific part
func (regs *vmRegs) getUnwindInfoARM() sdtypes.UnwindInfo {
	// Is CFA valid?
	// Not sure if this ever occurs on ARM64, it's been observed that the
	// initial CFA opcodes setup CFA to SP.
	if regs.cfa.reg == regUndefined {
		return sdtypes.UnwindInfoStop
	}

	// Undefined RA (aka X30/LR) marks entry point / end-of-stack functions.
	if regs.ra.reg == regUndefined {
		return sdtypes.UnwindInfoStop
	}

	var info sdtypes.UnwindInfo

	// Determine unwind info for stack pointer (CFA)
	// For ARM64, the Analyser output indicated only simple (no deref) register
	// (usually FP and SP, but sometimes x12 as in qpdldecode) based expressions
	// are used for CFA.
	switch regs.cfa.reg {
	case armRegFP:
		info.Opcode = support.UnwindOpcodeBaseFP
		info.Param = int32(regs.cfa.off)
	case armRegSP:
		info.Opcode = support.UnwindOpcodeBaseSP
		info.Param = int32(regs.cfa.off)
	}

	// Determine unwind info for return address
	// In order to not increase the EBPF stack deltas map, FP opcode is used
	// to hold RA unwinding information.
	switch regs.ra.reg {
	case regSame:
		// for ARM64:
		// 1) the link register is loaded with RA prior to the call (one can assume
		//    it is valid for a sequence of prolog instructions, prior to its value
		//    being stored into the stack)
		// 2) the link register is restored from the stack (one can assume it is
		//    valid for a sequence of instructions in the function prolog - prior to
		//    the ret instruction itself)
		// thus, the assumption - use UnwindOpcodeBaseLR to instruct native stack
		// unwinder to load RA from link register
		// This is either prolog or epilog sequence, read RA from link register.
		info.FPOpcode = support.UnwindOpcodeBaseLR
		info.FPParam = 0
	case regCFA:
		if regs.cfa.off != 0 {
			// In ARM64, nothing can be assumed regarding RA location, it is
			// simply somewhere on the stack, its detailed location needs to
			// be extracted from FDE record.
			// In our approach, RA offset part of stack delta always points
			// to RA location  no matter whether CFA is evaluated with respect
			// to SP or FP.
			// Use same opcode as for CFA:
			info.FPOpcode = info.Opcode
			// Convert CFA base to SP / FP base in order to keep
			// offset to RA from frame bottom (FP based heuristic).
			// CFA offset needs to be added to the one denoting RA location.
			info.FPParam = int32(regs.cfa.off) + int32(regs.ra.off)
		}
	}

	return info
}

func detectEntryARM(code []byte) int {
	// Refer to test cases for the seen assembly dumps.
	// Both, on GLIBC and MUSL there is no FDE for the entry code. This code tries
	// to match both. The main difference is that glibc uses BL (Branch with Link)
	// or a proper function call to maintain frame, and musl uses B (Branch) or
	// a jump so the entry is not seen on traces.

	// Match the prolog for clearing LR/FP
	if len(code) < 32 ||
		!bytes.Equal(code[:8], []byte{0x1d, 0x00, 0x80, 0xd2, 0x1e, 0x00, 0x80, 0xd2}) {
		return 0
	}

	// Search for the second B or BL
	numBranch := 0
	for pos := 8; pos < len(code); pos += 4 {
		inst, err := arm64asm.Decode(code[pos:])
		if err != nil {
			return 0
		}
		switch inst.Op {
		case arm64asm.ADD, arm64asm.ADRP, arm64asm.AND, arm64asm.LDR,
			arm64asm.MOV, arm64asm.MOVK, arm64asm.MOVZ:
			// nop, allowed instruction
		case arm64asm.B, arm64asm.BL:
			numBranch++
			if numBranch == 2 {
				return pos + 4
			}
		default:
			return 0
		}
	}
	return 0
}
