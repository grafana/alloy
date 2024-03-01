package vm

import (
	"github.com/grafana/river/internal/value"
	"github.com/grafana/river/token"
)

func evalUnaryOp(op token.Token, val value.Value) (value.Value, error) {
	switch op {
	case token.NOT:
		if val.Type() != value.TypeBool {
			return value.Null, value.TypeError{Value: val, Expected: value.TypeBool}
		}
		return value.Bool(!val.Bool()), nil

	case token.SUB:
		if val.Type() != value.TypeNumber {
			return value.Null, value.TypeError{Value: val, Expected: value.TypeNumber}
		}

		valNum := val.Number()
		switch valNum.Kind() {
		case value.NumberKindInt, value.NumberKindUint:
			// It doesn't make much sense to invert a uint, so we always cast to an
			// int and return an int.
			return value.Int(-valNum.Int()), nil
		case value.NumberKindFloat:
			return value.Float(-valNum.Float()), nil
		}
	}

	panic("river/vm: unreachable")
}
