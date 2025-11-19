// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expression // import "go.opentelemetry.io/ebpf-profiler/asm/expression"

import (
	"fmt"

	"golang.org/x/arch/x86/x86asm"
)

var _ Expression = &mem{}

func MemWithSegment8(segment x86asm.Reg, at Expression) Expression {
	return MemWithSegment(segment, at, 8)
}

func MemWithSegment(segment x86asm.Reg, at Expression, sizeBytes int) Expression {
	return &mem{at: at, segment: segment, sizeBytes: sizeBytes}
}

func Mem8(at Expression) Expression {
	return Mem(at, 8)
}

func Mem1(at Expression) Expression {
	return Mem(at, 1)
}

func Mem(at Expression, sizeBytes int) Expression {
	return &mem{at: at, segment: 0, sizeBytes: sizeBytes}
}

type mem struct {
	segment   x86asm.Reg
	at        Expression
	sizeBytes int
}

func (v *mem) DebugString() string {
	if v.segment == 0 {
		return fmt.Sprintf("[%s : %d bits]", v.at.DebugString(), v.sizeBytes*8)
	}
	return fmt.Sprintf("[%s : %s : %d bits]", v.segment, v.at.DebugString(), v.sizeBytes*8)
}

func (v *mem) Match(pattern Expression) bool {
	switch typedPattern := pattern.(type) {
	case *mem:
		if v.segment != typedPattern.segment {
			return false
		}
		return v.at.Match(typedPattern.at)
	default:
		return false
	}
}
