package vm

import (
	"errors"
	"fmt"
	"math"
	"reflect"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/alloy/syntax/internal/value"
	"github.com/grafana/alloy/syntax/token"
)

func evalBinop(lhs value.Value, op token.Token, rhs value.Value) (value.Value, error) {
	// Original parameters of lhs and rhs used for returning errors.
	var (
		origLHS = lhs
		origRHS = rhs
	)

	if lhs.Type() == value.TypeCapsule || rhs.Type() == value.TypeCapsule {
		lhs, rhs = tryUnwrapCapsules(lhs, rhs)
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

	// The type of lhs must be acceptable for the binary operator.
	if !acceptableBinopType(lhs, op) {
		return value.Null, value.Error{
			Value: origLHS,
			Inner: fmt.Errorf("should be one of %v for binop %s, got %s", binopAllowedTypes[op], op, lhs.Type()),
		}
	}

	// The type of rhs must be acceptable for the binary operator.
	if !acceptableBinopType(rhs, op) {
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

	// number + number
	// string + string
	// capsule(OptionalSecret) + capsule(OptionalSecret)
	// capsule(Secret) + capsule(Secret)
	case token.ADD:
		if lhs.Type() == value.TypeString {
			return value.String(lhs.Text() + rhs.Text()), nil
		}

		if lhs.Type() == value.TypeCapsule {
			switch lhsValue := lhs.Interface().(type) {
			case alloytypes.OptionalSecret:
				rhsOptSecret, _ := rhs.Interface().(alloytypes.OptionalSecret)
				return value.Encapsulate(
					alloytypes.OptionalSecret{
						Value:    lhsValue.Value + rhsOptSecret.Value,
						IsSecret: (lhsValue.IsSecret || rhsOptSecret.IsSecret),
					},
				), nil
			case alloytypes.Secret:
				rhsSecret, _ := rhs.Interface().(alloytypes.Secret)
				return value.Encapsulate(alloytypes.Secret(string(lhsValue) + string(rhsSecret))), nil
			default:
				return value.Null, value.Error{
					Value: origLHS,
					Inner: fmt.Errorf("could not perform binop %s for unknown types lhs %s rhs %s", op, lhs.Type(), rhs.Type()),
				}
			}
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
		switch rhsNum.Kind() {
		case value.NumberKindUint:
			if rhsNum.Uint() == uint64(0) {
				return value.Null, value.Error{
					Value: origRHS,
					Inner: errors.New("divide by zero error"),
				}
			}
		case value.NumberKindInt:
			if rhsNum.Int() == int64(0) {
				return value.Null, value.Error{
					Value: origRHS,
					Inner: errors.New("divide by zero error"),
				}
			}
		case value.NumberKindFloat:
			if rhsNum.Float() == float64(0) {
				return value.Null, value.Error{
					Value: origRHS,
					Inner: errors.New("divide by zero error"),
				}
			}
		}
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
		switch rhsNum.Kind() {
		case value.NumberKindUint:
			if rhsNum.Uint() == uint64(0) {
				return value.Null, value.Error{
					Value: origRHS,
					Inner: errors.New("divide by zero error"),
				}
			}
		case value.NumberKindInt:
			if rhsNum.Int() == int64(0) {
				return value.Null, value.Error{
					Value: origRHS,
					Inner: errors.New("divide by zero error"),
				}
			}
		case value.NumberKindFloat:
			if rhsNum.Float() == float64(0) {
				return value.Null, value.Error{
					Value: origRHS,
					Inner: errors.New("divide by zero error"),
				}
			}
		}
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

	panic("syntax/vm: unreachable")
}

// tryUnwrapCapsules accepts two value and tries to unwarp them to similar types
// If none of the condition matches it will return the inputs
func tryUnwrapCapsules(lhs value.Value, rhs value.Value) (value.Value, value.Value) {
	switch lhs.Interface().(type) {
	case alloytypes.OptionalSecret:
		return tryUnwrapOptionalSecrets(lhs, rhs)
	case alloytypes.Secret:
		return tryUnwrapSecret(lhs, rhs)
	case string:
		return tryUnwrapString(lhs, rhs)
	default:
		return lhs, rhs
	}
}

// tryUnwrapOptionalSecrets tries to Unwarp the rhs value corresponding to the
// lhs value of type OptionalSecret. If lhs is not of type OptionalSecret it returns the input
//  1. lhs(OptionalSecrets), rhs(Secret) = lhs(Secret), rhs(Secret)
//  2. lhs(OptionalSecrets), rhs(string):
//     2(a). lhs(OptionalSecrets{IsSecret: false}), rhs(string) = lhs(string), rhs(string)
//     2(b). lhs(OptionalSecrets{IsSecret: true}), rhs(string) = lhs(OptionalSecrets), rhs(OptionalSecrets)
//  3. If no condition matches it returns the inputs
func tryUnwrapOptionalSecrets(lhs value.Value, rhs value.Value) (value.Value, value.Value) {
	lhsOptSecret, lhsIsOptSecret := lhs.Interface().(alloytypes.OptionalSecret)
	if !lhsIsOptSecret {
		return lhs, rhs
	}

	switch rhsValue := rhs.Interface().(type) {
	case alloytypes.Secret:
		return value.Encapsulate(alloytypes.Secret(lhsOptSecret.Value)), rhs
	case string:
		if lhsOptSecret.IsSecret {
			return lhs, value.Encapsulate(alloytypes.OptionalSecret{Value: rhsValue, IsSecret: true})
		}
		return value.String(lhsOptSecret.Value), value.String(rhsValue)
	default:
		return lhs, rhs
	}
}

// tryUnwrapSecret tries to Unwarp the rhs value corresponding to the
// lhs value of type Secret. If lhs is not of type Secret it returns the input
//  1. lhs(Secret), rhs(OptionalSecret) = lhs(Secret), rhs(Secret)
//  2. lhs(Secret), rhs(string) = lhs(Secret), rhs(Secret)
//  3. If no condition matches it returns the inputs
func tryUnwrapSecret(lhs value.Value, rhs value.Value) (value.Value, value.Value) {
	_, lhsIsSecret := lhs.Interface().(alloytypes.Secret)
	if !lhsIsSecret {
		return lhs, rhs
	}

	switch rhsValue := rhs.Interface().(type) {
	case alloytypes.OptionalSecret:
		return lhs, value.Encapsulate(alloytypes.Secret(rhsValue.Value))
	case string:
		return lhs, value.Encapsulate(alloytypes.Secret(rhsValue))
	default:
		return lhs, rhs
	}
}

// tryUnwrapString tries to Unwarp the rhs value corresponding to the
// lhs value of type string. If lhs is not of type string it returns the input
//  1. lhs(string), rhs(OptionalSecrets):
//     1(a). lhs(string), rhs(OptionalSecrets{IsSecret: false}) = lhs(string), rhs(string)
//     1(b). lhs(string), rhs(OptionalSecrets{IsSecret: true}) = lhs(OptionalSecrets), rhs(OptionalSecrets)
//  2. lhs(string), rhs(Secret) = lhs(Secret), rhs(Secret)
//  3. If no condition matches it returns the inputs
func tryUnwrapString(lhs value.Value, rhs value.Value) (value.Value, value.Value) {
	lhsString, lhsIsString := lhs.Interface().(string)
	if !lhsIsString {
		return lhs, rhs
	}

	switch rhsValue := rhs.Interface().(type) {
	case alloytypes.OptionalSecret:
		if rhsValue.IsSecret {
			return value.Encapsulate(alloytypes.OptionalSecret{Value: lhsString, IsSecret: true}), rhs
		}
		return value.String(lhsString), value.String(rhsValue.Value)
	case alloytypes.Secret:
		return value.Encapsulate(alloytypes.Secret(lhsString)), rhs
	default:
		return lhs, rhs
	}
}

// valuesEqual returns true if two Values are equal.
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
		// there's no way to compare them in Alloy syntax right now.
		return false

	case value.TypeCapsule:
		// Two capsules are only equal if the underlying values are deeply equal.
		return reflect.DeepEqual(lhs.Interface(), rhs.Interface())
	}

	panic("syntax/vm: unreachable")
}

// binopAllowedTypes maps what type of values are permitted for a specific
// binary operation.
//
// token.EQ and token.NEQ are not included as they're handled separately from
// other binary ops.
var binopAllowedTypes = map[token.Token][]value.Type{
	token.OR:  {value.TypeBool},
	token.AND: {value.TypeBool},

	token.ADD: {value.TypeNumber, value.TypeString, value.TypeCapsule},
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
		panic("syntax/vm: unexpected binop type")
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
	switch {
	case m == 0 || n == 1:
		return 1
	case n == 0 && m > 0:
		return 0
	}
	result := n
	for i := Number(2); i <= m; i++ {
		result *= n
	}
	return result
}
