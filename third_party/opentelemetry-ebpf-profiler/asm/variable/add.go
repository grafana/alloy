// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package variable // import "go.opentelemetry.io/ebpf-profiler/asm/variable"
import (
	"math"
	"math/bits"
)

func Add(vs ...Expression) Expression {
	oss := make(operands, 0, len(vs)+1)
	v := uint64(0)
	for _, it := range vs {
		if o, ok := it.(*op); ok && o.typ == opAdd {
			for _, jit := range o.operands {
				if imm, immOk := jit.(*immediate); immOk {
					v += imm.Value
				} else {
					oss = append(oss, jit)
				}
			}
		} else {
			if imm, immOk := it.(*immediate); immOk {
				v += imm.Value
			} else {
				oss = append(oss, it)
			}
		}
	}
	if len(oss) == 0 {
		return Imm(v)
	}
	if v != 0 {
		oss = append(oss, Imm(v))
	}
	if len(oss) == 1 {
		return oss[0]
	}
	return newOp(opAdd, oss)
}

func (o *op) maxAddValue() uint64 {
	v := uint64(0)
	c := uint64(0)
	for i := range o.operands {
		v, c = bits.Add64(v, o.operands[i].MaxValue(), 0)
		if c != 0 {
			return math.MaxUint64
		}
	}
	return v
}
