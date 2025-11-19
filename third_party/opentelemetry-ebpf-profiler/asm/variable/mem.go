// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package variable // import "go.opentelemetry.io/ebpf-profiler/asm/variable"

import (
	"fmt"
	"math"

	"golang.org/x/arch/x86/x86asm"
)

var _ Expression = &mem{}

func MemS(segment x86asm.Reg, at Expression, sizeBytes int) Expression {
	return &mem{at: at, segment: segment, sizeBytes: sizeBytes}
}

func Mem(at Expression, sizeBytes int) Expression {
	return &mem{at: at, segment: 0, sizeBytes: sizeBytes}
}

type mem struct {
	segment   x86asm.Reg
	at        Expression
	sizeBytes int
}

func (v *mem) MaxValue() uint64 {
	return math.MaxUint64
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
	case *Variable:
		if typedPattern.isAny {
			typedPattern.extracted = v
			return true
		}
		return false
	default:
		return false
	}
}
