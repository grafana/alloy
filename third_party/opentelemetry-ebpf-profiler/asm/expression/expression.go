// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expression // import "go.opentelemetry.io/ebpf-profiler/asm/expression"
import "sort"

// Expression is an interface representing a 64-bit size value. It can be immediate
type Expression interface {
	// Match compares this Expression value against a pattern Expression.
	// The order of the arguments matters: a.Match(b) or b.Match(a) may
	// produce different results. The intended order The pattern should be passed as
	// an argument, not the other way around.
	// It returns true if the values are considered equal or compatible according to
	// the type-specific rules:
	// - For operations (add, mul): checks if operation types and operands match
	// - For immediate: checks if values are equal and extracts value into a ImmediateCapture
	// - For mem references: checks if segments and addresses match
	// - For extend operations: checks if sizes and inner values match
	// - For named: checks if they are pointing to the same object instance.
	// - For ImmediateCapture: matches nothing - see immediate
	Match(pattern Expression) bool
	DebugString() string
}

type operands []Expression

func (os *operands) Match(other operands) bool {
	if len(*os) != len(other) {
		return false
	}
	sort.Sort(sortedOperands(*os))
	sort.Sort(sortedOperands(other))
	for i := 0; i < len(*os); i++ {
		if !(*os)[i].Match(other[i]) {
			return false
		}
	}
	return true
}

type sortedOperands operands

func (s sortedOperands) Len() int {
	return len(s)
}

func (s sortedOperands) Less(i, j int) bool {
	o1 := cmpOrder(s[i])
	o2 := cmpOrder(s[j])
	return o1 < o2
}

func (s sortedOperands) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func cmpOrder(u Expression) int {
	switch u.(type) {
	case *mem:
		return 1
	case *op:
		return 2
	case *named:
		return 3
	case *ImmediateCapture:
		return 4
	case *immediate:
		return 5
	default:
		return 0
	}
}
