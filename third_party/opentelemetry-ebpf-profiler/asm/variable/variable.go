// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package variable // import "go.opentelemetry.io/ebpf-profiler/asm/variable"

import (
	"math"
)

var _ Expression = &Variable{}

func Any() *Variable {
	v := Var("any")
	v.isAny = true
	return v
}

func Var(name string) *Variable {
	return &Variable{
		extracted:          nil,
		name:               name,
		maxValueConstraint: math.MaxUint64,
		isAny:              false,
	}
}

type Variable struct {
	name               string
	maxValueConstraint uint64
	// if true - extract any Expression, if false - extract only immediate
	isAny     bool
	extracted Expression
}

func (v *Variable) ExtractedValueImm() uint64 {
	if v.extracted == nil {
		return 0
	}
	imm, ok := v.extracted.(*immediate)
	if ok {
		return imm.Value
	}
	return 0
}

func (v *Variable) MaxValue() uint64 {
	if v.extracted != nil {
		if v.extracted == v {
			return v.maxValueConstraint
		}
		return v.extracted.MaxValue()
	}
	return v.maxValueConstraint
}

func (v *Variable) DebugString() string {
	return "@" + v.name
}

func (v *Variable) Match(pattern Expression) bool {
	switch typedPattern := pattern.(type) {
	case *Variable:
		if typedPattern.isAny {
			typedPattern.extracted = v
			return true
		}
		if v == typedPattern {
			typedPattern.extracted = v
			return true
		}
		return false
	default:
		return false
	}
}

func (v *Variable) SetMaxValue(i uint64) *Variable {
	v.maxValueConstraint = i
	return v
}
