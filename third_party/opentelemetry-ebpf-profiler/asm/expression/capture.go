// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expression // import "go.opentelemetry.io/ebpf-profiler/asm/expression"

var _ Expression = &ImmediateCapture{}

func NewImmediateCapture(name string) *ImmediateCapture {
	return &ImmediateCapture{
		name: name,
	}
}

type ImmediateCapture struct {
	name          string
	capturedValue immediate
}

func (v *ImmediateCapture) CapturedValue() uint64 {
	return v.capturedValue.Value
}

func (v *ImmediateCapture) DebugString() string {
	return "@" + v.name
}

func (v *ImmediateCapture) Match(_ Expression) bool {
	return false
}
