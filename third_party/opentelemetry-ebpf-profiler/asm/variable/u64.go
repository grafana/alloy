// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package variable // import "go.opentelemetry.io/ebpf-profiler/asm/variable"
import "sort"

// Expression is an interface representing a 64-bit size value. It can be immediate
type Expression interface {
	// Match compares this Expression value with another Expression for equality or compatibility.
	// It returns true if the values are considered equal or compatible according to
	// the type-specific rules:
	// - For operations (add, mul): checks if operation types and operands match
	// - For immediate: checks if values are equal, or extracts value into a Variable
	// - For memory references: checks if segments and addresses match
	// - For extend operations: checks if sizes and inner values match
	// - For variables: checks if they are the same or if one is marked as "any"
	Match(pattern Expression) bool
	DebugString() string
	MaxValue() uint64
}

type operands []Expression

func (os *operands) Pop() {
	*os = (*os)[:len(*os)-1]
}

func (os *operands) Push(v Expression) {
	*os = append(*os, v)
}

func (os *operands) Eval(other operands) bool {
	if len(*os) != len(other) {
		return false
	}
	if len(*os) == 2 {
		if (*os)[0].Match(other[0]) && (*os)[1].Match(other[1]) {
			return true
		}
		if (*os)[0].Match(other[1]) && (*os)[1].Match(other[0]) {
			return true
		}
		return false
	}
	sort.Sort(sortedOperands(*os))
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
	case *immediate:
		return 4
	case *Variable:
		return 3
	default:
		return 0
	}
}
