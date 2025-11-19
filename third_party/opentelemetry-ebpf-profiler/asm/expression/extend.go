// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expression // import "go.opentelemetry.io/ebpf-profiler/asm/expression"
import (
	"fmt"
	"math"
)

var _ Expression = &extend{}

func SignExtend(v Expression, bits int) Expression {
	return &extend{v, bits, true}
}

func ZeroExtend32(v Expression) Expression {
	return ZeroExtend(v, 32)
}

func ZeroExtend8(v Expression) Expression {
	return ZeroExtend(v, 8)
}

func ZeroExtend(v Expression, bits int) Expression {
	if bits >= 64 {
		bits = 64
	}
	c := &extend{
		v:    v,
		bits: bits,
	}
	if c.bits == 0 {
		return Imm(0)
	}
	if c.bits == 64 {
		return c.v
	}
	switch typed := c.v.(type) {
	case *immediate:
		return Imm(typed.Value & c.MaxValue())
	case *extend:
		if typed.sign {
			return c
		}
		if typed.bits <= c.bits {
			return typed
		}
		return &extend{typed.v, c.bits, false}
	default:
		return c
	}
}

type extend struct {
	v    Expression
	bits int
	sign bool
}

func (c *extend) MaxValue() uint64 {
	if c.bits >= 64 || c.sign {
		return math.MaxUint64
	}
	return 1<<c.bits - 1
}

func (c *extend) Match(pattern Expression) bool {
	switch typedPattern := pattern.(type) {
	case *extend:
		return typedPattern.bits == c.bits &&
			typedPattern.sign == c.sign &&
			c.v.Match(typedPattern.v)
	default:
		return false
	}
}

func (c *extend) DebugString() string {
	s := "zero"
	if c.sign {
		s = "sign"
	}
	return fmt.Sprintf("%s-extend(%s, %d bits)", s, c.v.DebugString(), c.bits)
}
