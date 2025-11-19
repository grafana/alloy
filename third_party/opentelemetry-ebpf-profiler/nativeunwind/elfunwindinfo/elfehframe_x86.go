// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package elfunwindinfo // import "go.opentelemetry.io/ebpf-profiler/nativeunwind/elfunwindinfo"

// x86-64 specific code for handling DWARF / stack delta extraction.
// The filename ends with `_x86` instead of `_amd64`, so that the code
// can be taken into account regardless of the target build platform.

import (
	"bytes"
	"debug/elf"
	"fmt"

	sdtypes "go.opentelemetry.io/ebpf-profiler/nativeunwind/stackdeltatypes"
	"go.opentelemetry.io/ebpf-profiler/support"
	"golang.org/x/arch/x86/x86asm"
)

const (
	// x86_64 abi (https://refspecs.linuxbase.org/elf/x86_64-abi-0.99.pdf, page 57)
	x86RegRAX uleb128 = 0
	x86RegRDX uleb128 = 1
	x86RegRCX uleb128 = 2
	x86RegRBX uleb128 = 3
	x86RegRSI uleb128 = 4
	x86RegRDI uleb128 = 5
	x86RegRBP uleb128 = 6
	x86RegRSP uleb128 = 7
	x86RegR8  uleb128 = 8
	x86RegR9  uleb128 = 9
	x86RegR10 uleb128 = 10
	x86RegR11 uleb128 = 11
	x86RegR12 uleb128 = 12
	x86RegR13 uleb128 = 13
	x86RegR14 uleb128 = 14
	x86RegR15 uleb128 = 15
	x86RegRIP uleb128 = 16

	x86LastReg uleb128 = iota
)

// newVMRegsX86 initializes the vmRegs structure for X86_64.
func newVMRegsX86() vmRegs {
	return vmRegs{
		arch: elf.EM_X86_64,
		cfa:  vmReg{arch: elf.EM_X86_64, reg: regUndefined},
		fp:   vmReg{arch: elf.EM_X86_64, reg: regUndefined},
		ra:   vmReg{arch: elf.EM_X86_64, reg: regUndefined},
	}
}

// getRegNameX86 converts register index to a string describing x86 register
func getRegNameX86(reg uleb128) string {
	switch reg {
	case x86RegRAX:
		return "rax"
	case x86RegRDX:
		return "rdx"
	case x86RegRCX:
		return "rcx"
	case x86RegRBX:
		return "rbx"
	case x86RegRSI:
		return "rsi"
	case x86RegRDI:
		return "rdi"
	case x86RegRBP:
		return "rbp"
	case x86RegRSP:
		return "rsp"
	default:
		if reg < x86LastReg {
			return fmt.Sprintf("r%d", reg)
		}
		return fmt.Sprintf("?%d", reg)
	}
}

// regX86 returns the address to x86 specific register in vmRegs
func (regs *vmRegs) regX86(ndx uleb128) *vmReg {
	switch ndx {
	case x86RegRBP:
		return &regs.fp
	case x86RegRIP:
		return &regs.ra
	default:
		return nil
	}
}

// getUnwindInfo x86 specific part
func (regs *vmRegs) getUnwindInfoX86() sdtypes.UnwindInfo {
	// Is CFA and RIP (return address) valid?
	if regs.cfa.reg == regUndefined || regs.ra.reg == regUndefined {
		return sdtypes.UnwindInfoStop
	}

	// Is RA popped out from stack?
	if regs.ra.reg == regCFA && regs.cfa.reg == x86RegRSP && regs.cfa.off+regs.ra.off < 0 {
		// It depends on context if this is INVALID or STOP. As this catch the musl
		// thread start __clone function, treat this as STOP. Seeing the INVALID
		// condition in samples is statistically unlikely.
		return sdtypes.UnwindInfoStop
	}

	// The CFI allows having Return Address (RA) be recoverable via an expression,
	// but the eBPF currently supports the ABI standard RA=CFA-8 only. Verify that
	// we are not in any weird hand woven assembly which is not supported.
	if regs.ra.reg != regCFA || regs.ra.off != -8 {
		return sdtypes.UnwindInfoInvalid
	}

	info := sdtypes.UnwindInfo{}

	// Determine unwind info for frame pointer
	switch regs.fp.reg {
	case regCFA:
		// Check that RBP is between CFA and stack top
		if regs.cfa.reg != x86RegRSP || (regs.fp.off < 0 && regs.fp.off >= -regs.cfa.off) {
			info.FPOpcode = support.UnwindOpcodeBaseCFA
			info.FPParam = int32(regs.fp.off)
		}
	case regExprReg:
		// expression: RBP+offrbp
		if r, _, offrbp, _ := splitOff(regs.fp.off); uleb128(r) == x86RegRBP {
			info.FPOpcode = support.UnwindOpcodeBaseFP
			info.FPParam = int32(offrbp)
		}
	}

	// Determine unwind info for stack pointer
	switch regs.cfa.reg {
	case x86RegRBP:
		info.Opcode = support.UnwindOpcodeBaseFP
		info.Param = int32(regs.cfa.off)
	case x86RegRSP:
		if regs.cfa.off != 0 {
			info.Opcode = support.UnwindOpcodeBaseSP
			info.Param = int32(regs.cfa.off)
		}
	case x86RegRAX, x86RegR9, x86RegR11, x86RegR15:
		// openssl libcrypto has handwritten assembly that use these registers
		// as the CFA directly. These function do not call other code that would
		// trash the register, so allow these for libcrypto.
		if regs.cfa.off%8 == 0 {
			info.Opcode = support.UnwindOpcodeBaseReg
			info.Param = int32(regs.cfa.reg) + int32(regs.cfa.off)<<1
		}
	case regExprPLT:
		info.Opcode = support.UnwindOpcodeCommand
		info.Param = support.UnwindCommandPLT
	case regExprRegDeref:
		reg, _, off, off2 := splitOff(regs.cfa.off)
		if param, ok := sdtypes.PackDerefParam(int32(off), int32(off2)); ok {
			switch uleb128(reg) {
			case x86RegRBP:
				// GCC SSE vectorized functions
				info.Opcode = support.UnwindOpcodeBaseFP | support.UnwindOpcodeFlagDeref
				info.Param = param
			case x86RegRSP:
				// OpenSSL assembly using SSE/AVX
				info.Opcode = support.UnwindOpcodeBaseSP | support.UnwindOpcodeFlagDeref
				info.Param = param
			}
		}
	}
	return info
}

func detectEntryX86(code []byte) int {
	// Refer to test cases for the actual assembly code seen.
	// On glibc, the entry has FDE. No fixup is needed.
	// On musl, the entry has no FDE, or possibly has an FDE covering part of it.
	// Detect the musl case and return entry.

	// Match the assembly exactly except the LEA call offset
	if len(code) < 32 ||
		!bytes.Equal(code[:9], []byte{0x48, 0x31, 0xed, 0x48, 0x89, 0xe7, 0x48, 0x8d, 0x35}) ||
		!bytes.Equal(code[13:22], []byte{0x48, 0x83, 0xe4, 0xf0, 0xe8, 0x00, 0x00, 0x00, 0x00}) {
		return 0
	}

	// Decode the second portion and allow whitelisted opcodes finding the JMP
	for pos := 22; pos < len(code); {
		inst, err := x86asm.Decode(code[pos:], 64)
		if err != nil {
			return 0
		}
		switch inst.Op {
		case x86asm.MOV, x86asm.LEA, x86asm.XOR:
			pos += inst.Len
		case x86asm.JMP:
			return pos + inst.Len
		default:
			return 0
		}
	}
	return 0
}
