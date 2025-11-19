// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expression // import "go.opentelemetry.io/ebpf-profiler/asm/expression"

import (
	"fmt"
	"strings"
)

type opType int

const (
	_ opType = iota
	opAdd
	opMul
)

type op struct {
	typ      opType
	operands operands
}

func newOp(typ opType, operands operands) Expression {
	res := &op{typ: typ, operands: operands}

	return res
}

func (o *op) Match(pattern Expression) bool {
	switch typedPattern := pattern.(type) {
	case *op:
		if o.typ != typedPattern.typ ||
			len(o.operands) != len(typedPattern.operands) {
			return false
		}
		return o.operands.Match(typedPattern.operands)
	default:
		return false
	}
}

func (o *op) DebugString() string {
	ss := make([]string, len(o.operands))
	for i := range o.operands {
		ss[i] = o.operands[i].DebugString()
	}
	sep := ""
	switch o.typ {
	case opAdd:
		sep = "+"
	case opMul:
		sep = "*"
	}
	return fmt.Sprintf("( %s )", strings.Join(ss, sep))
}
