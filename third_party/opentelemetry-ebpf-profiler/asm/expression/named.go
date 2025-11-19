// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expression // import "go.opentelemetry.io/ebpf-profiler/asm/expression"

var _ Expression = &named{}

func Named(name string) Expression {
	return &named{
		name: name,
	}
}

type named struct {
	name string
}

func (v *named) DebugString() string {
	return "@" + v.name
}

func (v *named) Match(pattern Expression) bool {
	return pattern == v
}
