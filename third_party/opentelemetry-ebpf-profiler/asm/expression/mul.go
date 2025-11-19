// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package expression // import "go.opentelemetry.io/ebpf-profiler/asm/expression"

func Multiply(vs ...Expression) Expression {
	oss := make(operands, 0, len(vs)+1)
	v := uint64(1)
	for _, it := range vs {
		if it == zero {
			return zero
		}
		if it == one {
			continue
		}
		if imm, immOk := it.(*immediate); immOk {
			v *= imm.Value
		} else {
			oss = append(oss, it)
		}
	}
	if len(oss) == 0 {
		return Imm(v)
	}
	if v != 1 {
		oss = append(oss, Imm(v))
	}
	if len(oss) == 1 {
		return oss[0]
	}

	if len(oss) == 2 {
		if a, ok := oss[0].(*op); ok && a.typ == opAdd {
			var res []Expression
			for _, ait := range a.operands {
				res = append(res, Multiply(ait, oss[1]))
			}
			return Add(res...)
		}
	}

	return newOp(opMul, oss)
}
