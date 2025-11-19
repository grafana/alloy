// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package amd // import "go.opentelemetry.io/ebpf-profiler/asm/amd"

import (
	"fmt"
	"io"
	"math"

	"go.opentelemetry.io/ebpf-profiler/asm/expression"
	"golang.org/x/arch/x86/x86asm"
)

type CodeBlock struct {
	Address expression.Expression
	Code    []byte
}

type Interpreter struct {
	Regs        Registers
	code        []byte
	CodeAddress expression.Expression
	pc          int
}

func NewInterpreter() *Interpreter {
	it := &Interpreter{}
	it.initRegs()
	return it
}

func NewInterpreterWithCode(code []byte) *Interpreter {
	it := &Interpreter{code: code, CodeAddress: expression.Named("code address")}
	it.initRegs()
	return it
}

func (i *Interpreter) ResetCode(code []byte, address expression.Expression) {
	i.code = code
	i.CodeAddress = address
	i.pc = 0
}

func (i *Interpreter) Loop() (x86asm.Inst, error) {
	return i.LoopWithBreak(func(x86asm.Inst) bool { return false })
}

func (i *Interpreter) LoopWithBreak(breakLoop func(op x86asm.Inst) bool) (x86asm.Inst, error) {
	prev := x86asm.Inst{}
	for {
		op, err := i.Step()
		if err != nil {
			return prev, err
		}
		if breakLoop(op) {
			return op, nil
		}
		prev = op
	}
}

func (i *Interpreter) Step() (x86asm.Inst, error) {
	if len(i.code) == 0 {
		return x86asm.Inst{}, io.EOF
	}
	var inst x86asm.Inst
	var err error
	if ok, instLen := DecodeSkippable(i.code); ok {
		inst = x86asm.Inst{Op: x86asm.NOP, Len: instLen}
	} else {
		inst, err = x86asm.Decode(i.code, 64)
		if err != nil {
			return inst, fmt.Errorf("at 0x%x : %v", i.pc, err)
		}
	}
	i.pc += inst.Len
	i.code = i.code[inst.Len:]
	i.Regs.setX86asm(x86asm.RIP, expression.Add(i.CodeAddress, expression.Imm(uint64(i.pc))))
	switch inst.Op {
	case x86asm.ADD:
		if dst, ok := inst.Args[0].(x86asm.Reg); ok {
			left := i.Regs.GetX86(dst)
			switch src := inst.Args[1].(type) {
			case x86asm.Imm:
				right := expression.Imm(uint64(src))
				i.Regs.setX86asm(dst, expression.Add(left, right))
			case x86asm.Reg:
				right := i.Regs.GetX86(src)
				i.Regs.setX86asm(dst, expression.Add(left, right))
			case x86asm.Mem:
				right := i.MemArg(src)
				right = expression.MemWithSegment(src.Segment, right, inst.MemBytes)
				i.Regs.setX86asm(dst, expression.Add(left, right))
			}
		}
	case x86asm.SHL:
		if dst, ok := inst.Args[0].(x86asm.Reg); ok {
			if src, imm := inst.Args[1].(x86asm.Imm); imm {
				v := expression.Multiply(
					i.Regs.GetX86(dst),
					expression.Imm(uint64(math.Pow(2, float64(src)))),
				)
				i.Regs.setX86asm(dst, v)
			}
		}
	case x86asm.MOV, x86asm.MOVZX, x86asm.MOVSXD, x86asm.MOVSX:
		if dst, ok := inst.Args[0].(x86asm.Reg); ok {
			switch src := inst.Args[1].(type) {
			case x86asm.Imm:
				i.Regs.setX86asm(dst, expression.Imm(uint64(src)))
			case x86asm.Reg:
				i.Regs.setX86asm(dst, i.Regs.GetX86(src))
			case x86asm.Mem:
				v := i.MemArg(src)

				dataSizeBits := inst.DataSize

				v = expression.MemWithSegment(src.Segment, v, inst.MemBytes)
				if inst.Op == x86asm.MOVSXD || inst.Op == x86asm.MOVSX {
					v = expression.SignExtend(v, dataSizeBits)
				} else {
					v = expression.ZeroExtend(v, dataSizeBits)
				}
				i.Regs.setX86asm(dst, v)
			}
		}
	case x86asm.XOR:
		if dst, ok := inst.Args[0].(x86asm.Reg); ok {
			if src, reg := inst.Args[1].(x86asm.Reg); reg {
				if src == dst {
					i.Regs.setX86asm(dst, expression.Imm(0))
				}
			}
		}
	case x86asm.AND:
		if dst, ok := inst.Args[0].(x86asm.Reg); ok {
			if src, imm := inst.Args[1].(x86asm.Imm); imm {
				if src == 3 { // todo other cases
					i.Regs.setX86asm(dst, expression.ZeroExtend(i.Regs.GetX86(dst), 2))
				}
			}
		}
	case x86asm.LEA:
		if dst, ok := inst.Args[0].(x86asm.Reg); ok {
			if src, mem := inst.Args[1].(x86asm.Mem); mem {
				v := i.MemArg(src)
				i.Regs.setX86asm(dst, v)
			}
		}
	default:
	}
	return inst, nil
}

func (i *Interpreter) MemArg(src x86asm.Mem) expression.Expression {
	vs := make([]expression.Expression, 0, 3)
	if src.Disp != 0 {
		vs = append(vs, expression.Imm(uint64(src.Disp)))
	}
	if src.Base != 0 {
		vs = append(vs, i.Regs.GetX86(src.Base))
	}
	if src.Index != 0 {
		v := expression.Multiply(
			i.Regs.GetX86(src.Index),
			expression.Imm(uint64(src.Scale)),
		)
		vs = append(vs, v)
	}
	v := expression.Add(vs...)
	return v
}

func (i *Interpreter) initRegs() {
	for j := range len(i.Regs.regs) {
		i.Regs.regs[j] = expression.Named(Reg(j).String())
	}
}
