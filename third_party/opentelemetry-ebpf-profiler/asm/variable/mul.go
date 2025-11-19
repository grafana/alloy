// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package variable // import "go.opentelemetry.io/ebpf-profiler/asm/variable"

func Multiply(vs ...Expression) Expression {
	return MultiplyWithOptions(Options{}, vs...)
}

type Options struct {
	NoUnwrap bool
}

func MultiplyWithOptions(opt Options, vs ...Expression) Expression {
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

	if len(oss) == 2 && !opt.NoUnwrap {
		if a, ok := oss[0].(*op); ok && a.typ == opAdd {
			var res []Expression
			for _, ait := range a.operands {
				res = append(res, MultiplyWithOptions(opt, ait, oss[1]))
			}
			return Add(res...)
		}
	}

	return newOp(opMul, oss)
}
