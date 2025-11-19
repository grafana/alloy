// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package tpbase // import "go.opentelemetry.io/ebpf-profiler/tpbase"
import (
	"errors"

	"go.opentelemetry.io/ebpf-profiler/asm/amd"
	e "go.opentelemetry.io/ebpf-profiler/asm/expression"
	"golang.org/x/arch/x86/x86asm"
)

func extractTSDInfoX86(code []byte) (TSDInfo, error) {
	it := amd.NewInterpreterWithCode(code)
	key := it.Regs.Get(amd.RDI)
	_, err := it.LoopWithBreak(func(op x86asm.Inst) bool {
		return op.Op == x86asm.RET
	})
	if err != nil {
		return TSDInfo{}, err
	}
	res := it.Regs.Get(amd.RAX)
	var (
		multiplier  = e.NewImmediateCapture("multiplier")
		multiplier2 = e.NewImmediateCapture("multiplier2")
		offset      = e.NewImmediateCapture("offset")
	)

	expected := e.Mem8(
		e.Add(
			e.Mem8(
				e.Add(
					e.MemWithSegment8(x86asm.FS, e.Imm(0)),
					offset,
				),
			),
			e.Multiply(
				e.ZeroExtend32(key),
				multiplier),
		),
	)
	if res.Match(expected) {
		return TSDInfo{
			Offset:     int16(offset.CapturedValue()),
			Multiplier: uint8(multiplier.CapturedValue()),
			Indirect:   1,
		}, nil
	}
	expected = e.Mem8(
		e.Add(
			e.MemWithSegment8(x86asm.FS, e.Imm(0x10)),
			e.Multiply(e.ZeroExtend32(key), multiplier),
			offset,
		),
	)
	if res.Match(expected) {
		return TSDInfo{
			Offset:     int16(offset.CapturedValue()),
			Multiplier: uint8(multiplier.CapturedValue()),
			Indirect:   0,
		}, nil
	}
	expected = e.Mem8(
		e.Add(
			e.MemWithSegment8(x86asm.FS, e.Imm(0x10)),
			e.Multiply(
				e.ZeroExtend32(e.Add(key, multiplier2)),
				multiplier,
			),
			offset,
		),
	)
	if res.Match(expected) {
		return TSDInfo{
			Offset: int16(multiplier.CapturedValue()*multiplier2.CapturedValue() +
				offset.CapturedValue()),
			Multiplier: uint8(multiplier.CapturedValue()),
			Indirect:   0,
		}, nil
	}
	return TSDInfo{}, errors.New("could not extract tsdInfo amd")
}
