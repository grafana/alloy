// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expression // import "go.opentelemetry.io/ebpf-profiler/asm/expression"
import "fmt"

var zero Expression = &immediate{0}
var one Expression = &immediate{1}

func Imm(v uint64) Expression {
	switch v {
	case 0:
		return zero
	case 1:
		return one
	default:
		return &immediate{v}
	}
}

type immediate struct {
	Value uint64
}

func (v *immediate) MaxValue() uint64 {
	return v.Value
}

func (v *immediate) DebugString() string {
	return fmt.Sprintf("0x%x", v.Value)
}

func (v *immediate) Match(pattern Expression) bool {
	switch typedPattern := pattern.(type) {
	case *immediate:
		return v.Value == typedPattern.Value
	case *ImmediateCapture:
		typedPattern.capturedValue = *v
		return true
	default:
		return false
	}
}
