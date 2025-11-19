// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package amd // import "go.opentelemetry.io/ebpf-profiler/asm/amd"

import (
	"fmt"

	"go.opentelemetry.io/ebpf-profiler/asm/expression"
	"golang.org/x/arch/x86/x86asm"
)

type Registers struct {
	regs [int(registersCount)]expression.Expression
}

type regEntry struct {
	idx  Reg
	bits int
}
type Reg uint8

const (
	_ Reg = iota
	RAX
	RCX
	RDX
	RBX
	RSP
	RBP
	RSI
	RDI
	R8
	R9
	R10
	R11
	R12
	R13
	R14
	R15
	RIP
	registersCount
)

var regNames = [...]string{
	RAX: "RAX",
	RCX: "RCX",
	RDX: "RDX",
	RBX: "RBX",
	RSP: "RSP",
	RBP: "RBP",
	RSI: "RSI",
	RDI: "RDI",
	R8:  "R8",
	R9:  "R9",
	R10: "R10",
	R11: "R11",
	R12: "R12",
	R13: "R13",
	R14: "R14",
	R15: "R15",
	RIP: "RIP",
}

func (r Reg) String() string {
	i := int(r)
	if r == 0 || i >= len(regNames) || regNames[i] == "" {
		return fmt.Sprintf("Reg(%d)", i)
	}
	return regNames[i]
}

func regMappingFor(reg x86asm.Reg) regEntry {
	switch reg {
	case x86asm.AL:
		return regEntry{idx: RAX, bits: 8}
	case x86asm.CL:
		return regEntry{idx: RCX, bits: 8}
	case x86asm.DL:
		return regEntry{idx: RDX, bits: 8}
	case x86asm.BL:
		return regEntry{idx: RBX, bits: 8}
	case x86asm.SPB:
		return regEntry{idx: RSP, bits: 8}
	case x86asm.BPB:
		return regEntry{idx: RBP, bits: 8}
	case x86asm.SIB:
		return regEntry{idx: RSI, bits: 8}
	case x86asm.DIB:
		return regEntry{idx: RDI, bits: 8}
	case x86asm.R8B:
		return regEntry{idx: R8, bits: 8}
	case x86asm.R9B:
		return regEntry{idx: R9, bits: 8}
	case x86asm.R10B:
		return regEntry{idx: R10, bits: 8}
	case x86asm.R11B:
		return regEntry{idx: R11, bits: 8}
	case x86asm.R12B:
		return regEntry{idx: R12, bits: 8}
	case x86asm.R13B:
		return regEntry{idx: R13, bits: 8}
	case x86asm.R14B:
		return regEntry{idx: R14, bits: 8}
	case x86asm.R15B:
		return regEntry{idx: R15, bits: 8}
	case x86asm.AX:
		return regEntry{idx: RAX, bits: 16}
	case x86asm.CX:
		return regEntry{idx: RCX, bits: 16}
	case x86asm.DX:
		return regEntry{idx: RDX, bits: 16}
	case x86asm.BX:
		return regEntry{idx: RBX, bits: 16}
	case x86asm.SP:
		return regEntry{idx: RSP, bits: 16}
	case x86asm.BP:
		return regEntry{idx: RBP, bits: 16}
	case x86asm.SI:
		return regEntry{idx: RSI, bits: 16}
	case x86asm.DI:
		return regEntry{idx: RDI, bits: 16}
	case x86asm.R8W:
		return regEntry{idx: R8, bits: 16}
	case x86asm.R9W:
		return regEntry{idx: R9, bits: 16}
	case x86asm.R10W:
		return regEntry{idx: R10, bits: 16}
	case x86asm.R11W:
		return regEntry{idx: R11, bits: 16}
	case x86asm.R12W:
		return regEntry{idx: R12, bits: 16}
	case x86asm.R13W:
		return regEntry{idx: R13, bits: 16}
	case x86asm.R14W:
		return regEntry{idx: R14, bits: 16}
	case x86asm.R15W:
		return regEntry{idx: R15, bits: 16}
	case x86asm.EAX:
		return regEntry{idx: RAX, bits: 32}
	case x86asm.ECX:
		return regEntry{idx: RCX, bits: 32}
	case x86asm.EDX:
		return regEntry{idx: RDX, bits: 32}
	case x86asm.EBX:
		return regEntry{idx: RBX, bits: 32}
	case x86asm.ESP:
		return regEntry{idx: RSP, bits: 32}
	case x86asm.EBP:
		return regEntry{idx: RBP, bits: 32}
	case x86asm.ESI:
		return regEntry{idx: RSI, bits: 32}
	case x86asm.EDI:
		return regEntry{idx: RDI, bits: 32}
	case x86asm.R8L:
		return regEntry{idx: R8, bits: 32}
	case x86asm.R9L:
		return regEntry{idx: R9, bits: 32}
	case x86asm.R10L:
		return regEntry{idx: R10, bits: 32}
	case x86asm.R11L:
		return regEntry{idx: R11, bits: 32}
	case x86asm.R12L:
		return regEntry{idx: R12, bits: 32}
	case x86asm.R13L:
		return regEntry{idx: R13, bits: 32}
	case x86asm.R14L:
		return regEntry{idx: R14, bits: 32}
	case x86asm.R15L:
		return regEntry{idx: R15, bits: 32}
	case x86asm.RAX:
		return regEntry{idx: RAX, bits: 64}
	case x86asm.RCX:
		return regEntry{idx: RCX, bits: 64}
	case x86asm.RDX:
		return regEntry{idx: RDX, bits: 64}
	case x86asm.RBX:
		return regEntry{idx: RBX, bits: 64}
	case x86asm.RSP:
		return regEntry{idx: RSP, bits: 64}
	case x86asm.RBP:
		return regEntry{idx: RBP, bits: 64}
	case x86asm.RSI:
		return regEntry{idx: RSI, bits: 64}
	case x86asm.RDI:
		return regEntry{idx: RDI, bits: 64}
	case x86asm.R8:
		return regEntry{idx: R8, bits: 64}
	case x86asm.R9:
		return regEntry{idx: R9, bits: 64}
	case x86asm.R10:
		return regEntry{idx: R10, bits: 64}
	case x86asm.R11:
		return regEntry{idx: R11, bits: 64}
	case x86asm.R12:
		return regEntry{idx: R12, bits: 64}
	case x86asm.R13:
		return regEntry{idx: R13, bits: 64}
	case x86asm.R14:
		return regEntry{idx: R14, bits: 64}
	case x86asm.R15:
		return regEntry{idx: R15, bits: 64}
	case x86asm.RIP:
		return regEntry{idx: RIP, bits: 64}
	default:
		return regEntry{idx: 0, bits: 64}
	}
}

func (r *Registers) setX86asm(reg x86asm.Reg, v expression.Expression) {
	e := regMappingFor(reg)
	if e.bits != 64 {
		v = expression.ZeroExtend(v, e.bits)
	}
	r.regs[e.idx] = v
}

// GetX86 returns the expression.Expression value associated with the given x86asm.Reg, with
// appropriate zero-extension if necessary.
func (r *Registers) GetX86(reg x86asm.Reg) expression.Expression {
	e := regMappingFor(reg)
	res := r.regs[e.idx]
	if e.bits != 64 {
		res = expression.ZeroExtend(res, e.bits)
	}
	return res
}

// Get returns the expression.Expression value associated with the given Reg register
func (r *Registers) Get(reg Reg) expression.Expression {
	if int(reg) >= len(r.regs) {
		return r.regs[0]
	}
	return r.regs[int(reg)]
}
