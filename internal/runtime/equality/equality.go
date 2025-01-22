package equality

import (
	"reflect"
)

var customEqualityType = reflect.TypeOf((*CustomEquality)(nil)).Elem()

// CustomEquality allows to define custom Equals implementation. This can be used, for example, with exported types,
// so that the Runtime can short-circuit propagating updates when it is not necessary.
type CustomEquality interface {
	Equals(other any) bool
}

// DeepEqual is a wrapper around reflect.DeepEqual, which first checks if arguments implement CustomEquality. If they
// do, their Equals method is used for comparison instead of reflect.DeepEqual.
// For simplicity, DeepEqual requires x and y to be of the same type before calling CustomEquality.Equals.
// TODO(thampiotr): this will need a lot of tests!!!
func DeepEqual(x, y any) bool {
	if x == nil || y == nil {
		return x == y
	}
	v1 := reflect.ValueOf(x)
	v2 := reflect.ValueOf(y)

	// See if we can compare them using CustomEquality
	if r := deepCustomEqual(v1, v2); r.compared {
		return r.isEqual
	}
	// Otherwise fall back to reflect.DeepEqual
	return reflect.DeepEqual(x, y)
}

type result struct {
	compared bool
	isEqual  bool
}

func successfulCompare(isEqual bool) result { return result{compared: true, isEqual: isEqual} }

var (
	couldNotCompare  = result{compared: false, isEqual: false}
	comparedAndEqual = result{compared: true, isEqual: true}
)

func deepCustomEqual(v1, v2 reflect.Value) result {
	if !v1.IsValid() || !v2.IsValid() {
		return couldNotCompare
	}

	if v1.Type() != v2.Type() {
		return couldNotCompare
	}

	if v1.Type().Implements(customEqualityType) {
		return successfulCompare(v1.Interface().(CustomEquality).Equals(v2.Interface()))
	}

	// Somewhat redundant, but just in case:
	if v2.Type().Implements(customEqualityType) {
		return successfulCompare(v2.Interface().(CustomEquality).Equals(v1.Interface()))
	}

	switch v1.Kind() {
	case reflect.Array:
		for i := 0; i < v1.Len(); i++ {
			partResult := deepCustomEqual(v1.Index(i), v2.Index(i))
			if !partResult.compared || !partResult.isEqual {
				return partResult
			}
		}
		return comparedAndEqual
	case reflect.Slice:
		if v1.IsNil() != v2.IsNil() {
			return couldNotCompare
		}
		if v1.Len() != v2.Len() {
			return couldNotCompare
		}
		for i := 0; i < v1.Len(); i++ {
			partResult := deepCustomEqual(v1.Index(i), v2.Index(i))
			if !partResult.compared || !partResult.isEqual {
				return partResult
			}
		}
		return comparedAndEqual
	case reflect.Interface, reflect.Pointer:
		if v1.IsNil() || v2.IsNil() {
			return couldNotCompare
		}
		return deepCustomEqual(v1.Elem(), v2.Elem())
	case reflect.Struct:
		for i, n := 0, v1.NumField(); i < n; i++ {
			partResult := deepCustomEqual(v1.Field(i), v2.Field(i))
			if !partResult.compared || !partResult.isEqual {
				return partResult
			}
		}
		return comparedAndEqual
	case reflect.Map:
		if v1.IsNil() != v2.IsNil() {
			return couldNotCompare
		}
		if v1.Len() != v2.Len() {
			return couldNotCompare
		}
		iter := v1.MapRange()
		for iter.Next() {
			val1 := iter.Value()
			val2 := v2.MapIndex(iter.Key())
			if !val1.IsValid() || !val2.IsValid() {
				return couldNotCompare
			}
			partResult := deepCustomEqual(val1, val2)
			if !partResult.compared || !partResult.isEqual {
				return partResult
			}
		}
		return comparedAndEqual
	default:
		return couldNotCompare
	}
}
