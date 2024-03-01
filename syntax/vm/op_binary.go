package vm

import (
	"fmt"
	"math"
	"reflect"

	"github.com/grafana/river/internal/value"
	"github.com/grafana/river/rivertypes"
	"github.com/grafana/river/token"
)

func evalBinop(lhs value.Value, op token.Token, rhs value.Value) (value.Value, error) {
	// Original parameters of lhs and rhs used for returning errors.
	var (
		origLHS = lhs
		origRHS = rhs
	)

	// Hack to allow OptionalSecrets to be used in binary operations.
	//
	// TODO(rfratto): be more flexible in the future with broader definitions of
	// how capsules can be converted to other types for the purposes of doing a
	// binop.
	if lhs.Type() == value.TypeCapsule {
		lhs = tryUnwrapOptionalSecret(lhs)
	}
	if rhs.Type() == value.TypeCapsule {
		rhs = tryUnwrapOptionalSecret(rhs)
	}

	// TODO(rfratto): evalBinop should check for underflows and overflows

	// We have special handling for EQ and NEQ since it's valid to attempt to
	// compare values of any two types.
	switch op {
	case token.EQ:
		return value.Bool(valuesEqual(lhs, rhs)), nil
	case token.NEQ:
		return value.Bool(!valuesEqual(lhs, rhs)), nil
	}

	// The type of lhs and rhs must be acceptable for the binary operator.
	if !acceptableBinopType(lhs, op) {
		return value.Null, value.Error{
			Value: origLHS,
			Inner: fmt.Errorf("should be one of %v for binop %s, got %s", binopAllowedTypes[op], op, lhs.Type()),
		}
	} else if !acceptableBinopType(rhs, op) {
		return value.Null, value.Error{
			Value: origRHS,
			Inner: fmt.Errorf("should be one of %v for binop %s, got %s", binopAllowedTypes[op], op, rhs.Type()),
		}
	}

	// At this point, regardless of the operator, lhs and rhs must have the same
	// type.
	if lhs.Type() != rhs.Type() {
		return value.Null, value.TypeError{Value: rhs, Expected: lhs.Type()}
	}

	switch op {
	case token.OR: // bool || bool
		return value.Bool(lhs.Bool() || rhs.Bool()), nil
	case token.AND: // bool && Bool
		return value.Bool(lhs.Bool() && rhs.Bool()), nil

	case token.ADD: // number + number, string + string
		if lhs.Type() == value.TypeString {
			return value.String(lhs.Text() + rhs.Text()), nil
		}

		lhsNum, rhsNum := lhs.Number(), rhs.Number()
		switch fitNumberKinds(lhsNum.Kind(), rhsNum.Kind()) {
		case value.NumberKindUint:
			return value.Uint(lhsNum.Uint() + rhsNum.Uint()), nil
		case value.NumberKindInt:
			return value.Int(lhsNum.Int() + rhsNum.Int()), nil
		case value.NumberKindFloat:
			return value.Float(lhsNum.Float() + rhsNum.Float()), nil
		}

	case token.SUB: // number - number
		lhsNum, rhsNum := lhs.Number(), rhs.Number()
		switch fitNumberKinds(lhsNum.Kind(), rhsNum.Kind()) {
		case value.NumberKindUint:
			return value.Uint(lhsNum.Uint() - rhsNum.Uint()), nil
		case value.NumberKindInt:
			return value.Int(lhsNum.Int() - rhsNum.Int()), nil
		case value.NumberKindFloat:
			return value.Float(lhsNum.Float() - rhsNum.Float()), nil
		}

	case token.MUL: // number * number
		lhsNum, rhsNum := lhs.Number(), rhs.Number()
		switch fitNumberKinds(lhsNum.Kind(), rhsNum.Kind()) {
		case value.NumberKindUint:
			return value.Uint(lhsNum.Uint() * rhsNum.Uint()), nil
		case value.NumberKindInt:
			return value.Int(lhsNum.Int() * rhsNum.Int()), nil
		case value.NumberKindFloat:
			return value.Float(lhsNum.Float() * rhsNum.Float()), nil
		}

	case token.DIV: // number / number
		lhsNum, rhsNum := lhs.Number(), rhs.Number()
		switch fitNumberKinds(lhsNum.Kind(), rhsNum.Kind()) {
		case value.NumberKindUint:
			return value.Uint(lhsNum.Uint() / rhsNum.Uint()), nil
		case value.NumberKindInt:
			return value.Int(lhsNum.Int() / rhsNum.Int()), nil
		case value.NumberKindFloat:
			return value.Float(lhsNum.Float() / rhsNum.Float()), nil
		}

	case token.MOD: // number % number
		lhsNum, rhsNum := lhs.Number(), rhs.Number()
		switch fitNumberKinds(lhsNum.Kind(), rhsNum.Kind()) {
		case value.NumberKindUint:
			return value.Uint(lhsNum.Uint() % rhsNum.Uint()), nil
		case value.NumberKindInt:
			return value.Int(lhsNum.Int() % rhsNum.Int()), nil
		case value.NumberKindFloat:
			return value.Float(math.Mod(lhsNum.Float(), rhsNum.Float())), nil
		}

	case token.POW: // number ^ number
		lhsNum, rhsNum := lhs.Number(), rhs.Number()
		switch fitNumberKinds(lhsNum.Kind(), rhsNum.Kind()) {
		case value.NumberKindUint:
			return value.Uint(intPow(lhsNum.Uint(), rhsNum.Uint())), nil
		case value.NumberKindInt:
			return value.Int(intPow(lhsNum.Int(), rhsNum.Int())), nil
		case value.NumberKindFloat:
			return value.Float(math.Pow(lhsNum.Float(), rhsNum.Float())), nil
		}

	case token.LT: // number < number, string < string
		// Check string first.
		if lhs.Type() == value.TypeString {
			return value.Bool(lhs.Text() < rhs.Text()), nil
		}

		// Not a string; must be a number.
		lhsNum, rhsNum := lhs.Number(), rhs.Number()
		switch fitNumberKinds(lhsNum.Kind(), rhsNum.Kind()) {
		case value.NumberKindUint:
			return value.Bool(lhsNum.Uint() < rhsNum.Uint()), nil
		case value.NumberKindInt:
			return value.Bool(lhsNum.Int() < rhsNum.Int()), nil
		case value.NumberKindFloat:
			return value.Bool(lhsNum.Float() < rhsNum.Float()), nil
		}

	case token.GT: // number > number, string > string
		// Check string first.
		if lhs.Type() == value.TypeString {
			return value.Bool(lhs.Text() > rhs.Text()), nil
		}

		// Not a string; must be a number.
		lhsNum, rhsNum := lhs.Number(), rhs.Number()
		switch fitNumberKinds(lhsNum.Kind(), rhsNum.Kind()) {
		case value.NumberKindUint:
			return value.Bool(lhsNum.Uint() > rhsNum.Uint()), nil
		case value.NumberKindInt:
			return value.Bool(lhsNum.Int() > rhsNum.Int()), nil
		case value.NumberKindFloat:
			return value.Bool(lhsNum.Float() > rhsNum.Float()), nil
		}

	case token.LTE: // number <= number, string <= string
		// Check string first.
		if lhs.Type() == value.TypeString {
			return value.Bool(lhs.Text() <= rhs.Text()), nil
		}

		// Not a string; must be a number.
		lhsNum, rhsNum := lhs.Number(), rhs.Number()
		switch fitNumberKinds(lhsNum.Kind(), rhsNum.Kind()) {
		case value.NumberKindUint:
			return value.Bool(lhsNum.Uint() <= rhsNum.Uint()), nil
		case value.NumberKindInt:
			return value.Bool(lhsNum.Int() <= rhsNum.Int()), nil
		case value.NumberKindFloat:
			return value.Bool(lhsNum.Float() <= rhsNum.Float()), nil
		}

	case token.GTE: // number >= number, string >= string
		// Check string first.
		if lhs.Type() == value.TypeString {
			return value.Bool(lhs.Text() >= rhs.Text()), nil
		}

		// Not a string; must be a number.
		lhsNum, rhsNum := lhs.Number(), rhs.Number()
		switch fitNumberKinds(lhsNum.Kind(), rhsNum.Kind()) {
		case value.NumberKindUint:
			return value.Bool(lhsNum.Uint() >= rhsNum.Uint()), nil
		case value.NumberKindInt:
			return value.Bool(lhsNum.Int() >= rhsNum.Int()), nil
		case value.NumberKindFloat:
			return value.Bool(lhsNum.Float() >= rhsNum.Float()), nil
		}
	}

	panic("river/vm: unreachable")
}

// tryUnwrapOptionalSecret accepts a value and, if it is a
// rivertypes.OptionalSecret where IsSecret is false, returns a string value
// instead.
//
// If val is not a rivertypes.OptionalSecret or IsSecret is true,
// tryUnwrapOptionalSecret returns the input value unchanged.
func tryUnwrapOptionalSecret(val value.Value) value.Value {
	optSecret, ok := val.Interface().(rivertypes.OptionalSecret)
	if !ok || optSecret.IsSecret {
		return val
	}

	return value.String(optSecret.Value)
}

// valuesEqual returns true if two River Values are equal.
func valuesEqual(lhs value.Value, rhs value.Value) bool {
	if lhs.Type() != rhs.Type() {
		// Two values with different types are never equal.
		return false
	}

	switch lhs.Type() {
	case value.TypeNull:
		// Nothing to compare here: both lhs and rhs have the null type,
		// so they're equal.
		return true

	case value.TypeNumber:
		// Two numbers are equal if they have equal values. However, we have to
		// determine what comparison we want to do and upcast the values to a
		// different Go type as needed (so that 3 == 3.0 is true).
		lhsNum, rhsNum := lhs.Number(), rhs.Number()
		switch fitNumberKinds(lhsNum.Kind(), rhsNum.Kind()) {
		case value.NumberKindUint:
			return lhsNum.Uint() == rhsNum.Uint()
		case value.NumberKindInt:
			return lhsNum.Int() == rhsNum.Int()
		case value.NumberKindFloat:
			return lhsNum.Float() == rhsNum.Float()
		}

	case value.TypeString:
		return lhs.Text() == rhs.Text()

	case value.TypeBool:
		return lhs.Bool() == rhs.Bool()

	case value.TypeArray:
		// Two arrays are equal if they have equal elements.
		if lhs.Len() != rhs.Len() {
			return false
		}
		for i := 0; i < lhs.Len(); i++ {
			if !valuesEqual(lhs.Index(i), rhs.Index(i)) {
				return false
			}
		}
		return true

	case value.TypeObject:
		// Two objects are equal if they have equal elements.
		if lhs.Len() != rhs.Len() {
			return false
		}
		for _, key := range lhs.Keys() {
			lhsElement, _ := lhs.Key(key)
			rhsElement, inRHS := rhs.Key(key)
			if !inRHS {
				return false
			}
			if !valuesEqual(lhsElement, rhsElement) {
				return false
			}
		}
		return true

	case value.TypeFunction:
		// Two functions are never equal. We can't compare functions in Go, so
		// there's no way to compare them in River right now.
		return false

	case value.TypeCapsule:
		// Two capsules are only equal if the underlying values are deeply equal.
		return reflect.DeepEqual(lhs.Interface(), rhs.Interface())
	}

	panic("river/vm: unreachable")
}

// binopAllowedTypes maps what type of values are permitted for a specific
// binary operation.
//
// token.EQ and token.NEQ are not included as they're handled separately from
// other binary ops.
var binopAllowedTypes = map[token.Token][]value.Type{
	token.OR:  {value.TypeBool},
	token.AND: {value.TypeBool},

	token.ADD: {value.TypeNumber, value.TypeString},
	token.SUB: {value.TypeNumber},
	token.MUL: {value.TypeNumber},
	token.DIV: {value.TypeNumber},
	token.MOD: {value.TypeNumber},
	token.POW: {value.TypeNumber},

	token.LT:  {value.TypeNumber, value.TypeString},
	token.GT:  {value.TypeNumber, value.TypeString},
	token.LTE: {value.TypeNumber, value.TypeString},
	token.GTE: {value.TypeNumber, value.TypeString},
}

func acceptableBinopType(val value.Value, op token.Token) bool {
	allowed, ok := binopAllowedTypes[op]
	if !ok {
		panic("river/vm: unexpected binop type")
	}

	actualType := val.Type()
	for _, allowType := range allowed {
		if allowType == actualType {
			return true
		}
	}
	return false
}

func fitNumberKinds(a, b value.NumberKind) value.NumberKind {
	aPrec, bPrec := numberKindPrec[a], numberKindPrec[b]
	if aPrec > bPrec {
		return a
	}
	return b
}

var numberKindPrec = map[value.NumberKind]int{
	value.NumberKindUint:  0,
	value.NumberKindInt:   1,
	value.NumberKindFloat: 2,
}

func intPow[Number int64 | uint64](n, m Number) Number {
	if m == 0 {
		return 1
	}
	result := n
	for i := Number(2); i <= m; i++ {
		result *= n
	}
	return result
}
