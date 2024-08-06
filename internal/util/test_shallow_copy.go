package util

import (
	"reflect"

	"github.com/grafana/alloy/syntax"
)

func SharePointer(a, b reflect.Value, initValues bool) (string, bool) {
	// We want to recursively check a and b, so if they're nil they need to be
	// initialized to see if any of their inner values have shared pointers after
	// being initialized with defaults.
	if initValues {
		initValue(a)
		initValue(b)
	}

	// From the documentation of reflect.Value.Pointer, values of chan, func,
	// map, pointer, slice, and unsafe pointer are all pointer values.
	//
	// Additionally, we want to recurse into values (even if they don't have
	// addresses) to see if there's shared pointers inside of them.
	switch a.Kind() {
	case reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return "", a.Pointer() == b.Pointer()

	case reflect.Map:
		if pointersMatch(a, b) {
			return "", true
		}

		iter := a.MapRange()
		for iter.Next() {
			aValue, bValue := iter.Value(), b.MapIndex(iter.Key())
			if !bValue.IsValid() {
				continue
			}
			if path, shared := SharePointer(aValue, bValue, initValues); shared {
				return path, true
			}
		}
		return "", false

	case reflect.Pointer:
		if pointersMatch(a, b) {
			return "", true
		} else {
			// Recursively navigate inside of the pointer.
			return SharePointer(a.Elem(), b.Elem(), initValues)
		}

	case reflect.Interface:
		if a.UnsafeAddr() == b.UnsafeAddr() {
			return "", true
		}
		return SharePointer(a.Elem(), b.Elem(), initValues)

	case reflect.Slice:
		if pointersMatch(a, b) {
			// If the slices are preallocated immutable pointers such as []string{}, we can ignore
			if a.Len() == 0 && a.Cap() == 0 && b.Len() == 0 && b.Cap() == 0 {
				return "", false
			}
			return "", true
		}

		size := min(a.Len(), b.Len())
		for i := 0; i < size; i++ {
			if path, shared := SharePointer(a.Index(i), b.Index(i), initValues); shared {
				return path, true
			}
		}
		return "", false
	}

	// Recurse into non-pointer types.
	switch a.Kind() {
	case reflect.Array:
		for i := 0; i < a.Len(); i++ {
			if path, shared := SharePointer(a.Index(i), b.Index(i), initValues); shared {
				return path, true
			}
		}
		return "", false

	case reflect.Struct:
		// Check to make sure there are no shared pointers between args1 and args2.
		for i := 0; i < a.NumField(); i++ {
			if path, shared := SharePointer(a.Field(i), b.Field(i), initValues); shared {
				fullPath := a.Type().Field(i).Name
				if path != "" {
					fullPath += "." + path
				}
				return fullPath, true
			}
		}
		return "", false
	}

	return "", false
}

func pointersMatch(a, b reflect.Value) bool {
	if a.IsNil() || b.IsNil() {
		return false
	}
	return a.Pointer() == b.Pointer()
}

// initValue initializes nil pointers. If the nil pointer implements
// syntax.Defaulter, it is also set to default values.
func initValue(rv reflect.Value) {
	if rv.Kind() == reflect.Pointer && rv.IsNil() {
		rv.Set(reflect.New(rv.Type().Elem()))
		if defaulter, ok := rv.Interface().(syntax.Defaulter); ok {
			defaulter.SetToDefault()
		}
	}
}
