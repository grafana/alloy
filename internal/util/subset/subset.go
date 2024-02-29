// Package subset implements functions to check if one value is a subset of
// another.
package subset

import (
	"fmt"
	"reflect"

	"gopkg.in/yaml.v2"
)

// Assert checks whether target is a subset of source. source and target must
// be the same type. target is a subset of source when:
//
//   - If target and source are slices or arrays, then target must have the same
//     number of elements as source. Each element in target must be a subset of
//     the corresponding element from source.
//
//   - If target and source are maps, each key in source must exist in target.
//     The value for each element in target must be a subset of the corresponding
//     element from source.
//
//   - Otherwise, target and source must be deeply equal.
//
// An instance of Error will be returned when target is not a subset of source.
//
// Subset checking is primarily useful when doing things like YAML assertions,
// where you only want to ensure that a subset of YAML is defined as expected.
func Assert(source, target interface{}) error {
	return assert(reflect.ValueOf(source), reflect.ValueOf(target))
}

func assert(source, target reflect.Value) error {
	// Deference interface/pointers for direct comparison
	for canElem(source) {
		source = source.Elem()
	}
	for canElem(target) {
		target = target.Elem()
	}

	if source.Type() != target.Type() {
		return &Error{Message: fmt.Sprintf("type mismatch: %T != %T", source.Interface(), target.Interface())}
	}

	switch source.Kind() {
	case reflect.Slice, reflect.Array:
		if source.Len() != target.Len() {
			return &Error{Message: fmt.Sprintf("length mismatch: %d != %d", source.Len(), target.Len())}
		}
		for i := 0; i < source.Len(); i++ {
			if err := assert(source.Index(i), target.Index(i)); err != nil {
				return &Error{
					Message: fmt.Sprintf("element %d", i),
					Inner:   err,
				}
			}
		}
		return nil

	case reflect.Map:
		iter := source.MapRange()
		for iter.Next() {
			var (
				sourceElement = iter.Value()
				targetElement = target.MapIndex(iter.Key())
			)
			if !targetElement.IsValid() {
				return &Error{Message: fmt.Sprintf("missing key %v", iter.Key().Interface())}
			}
			if err := assert(sourceElement, targetElement); err != nil {
				return &Error{
					Message: fmt.Sprintf("%v", iter.Key().Interface()),
					Inner:   err,
				}
			}
		}
		return nil

	default:
		if !reflect.DeepEqual(source.Interface(), target.Interface()) {
			return &Error{Message: fmt.Sprintf("%v != %v", source, target)}
		}
		return nil
	}
}

func canElem(v reflect.Value) bool {
	return v.Kind() == reflect.Interface || v.Kind() == reflect.Ptr
}

// Error is a subset assertion error.
type Error struct {
	Message string // Message of the error
	Inner   error  // Optional inner error
}

// Error implements error.
func (e *Error) Error() string {
	if e.Inner == nil {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Message, e.Inner)
}

// Unwrap returns the inner error, if set.
func (e *Error) Unwrap() error { return e.Inner }

// YAMLAssert is like Assert but accepts YAML bytes as input.
func YAMLAssert(source, target []byte) error {
	var sourceValue interface{}
	if err := yaml.Unmarshal(source, &sourceValue); err != nil {
		return err
	}
	var targetValue interface{}
	if err := yaml.Unmarshal(target, &targetValue); err != nil {
		return err
	}
	return Assert(sourceValue, targetValue)
}
