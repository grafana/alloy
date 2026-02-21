package all

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSetDefault_NoPointerReuse ensures that calls to SetDefault do not re-use
// pointers. The test iterates through all registered components, and then
// recursively traverses through its Arguments type to guarantee that no two
// calls to SetDefault result in pointer reuse.
//
// Nested types that also implement syntax.Defaulter are also checked.
func TestSetDefault_NoPointerReuse(t *testing.T) {
	allComponents := component.AllNames()
	for _, componentName := range allComponents {
		reg, ok := component.Get(componentName)
		require.True(t, ok, "Expected component %q to exist", componentName)

		t.Run(reg.Name, func(t *testing.T) {
			testNoReusePointer(t, reg)
		})
	}
}

func testNoReusePointer(t *testing.T, reg component.Registration) {
	t.Helper()

	var (
		args1 = reg.CloneArguments()
		args2 = reg.CloneArguments()
	)

	if args1, ok := args1.(syntax.Defaulter); ok {
		args1.SetToDefault()
	}
	if args2, ok := args2.(syntax.Defaulter); ok {
		args2.SetToDefault()
	}

	rv1, rv2 := reflect.ValueOf(args1), reflect.ValueOf(args2)
	ty := rv1.Type().Elem()

	// Edge case: if the component's arguments type is an empty struct, skip.
	// Not skipping causes the test to fail, due to an optimization in
	// reflect.New where initializing the same zero-length object results in the
	// same pointer.
	if rv1.Elem().NumField() == 0 {
		return
	}

	if path, shared := sharePointer(rv1, rv2); shared {
		fullPath := fmt.Sprintf("%s.%s.%s", ty.PkgPath(), ty.Name(), path)

		assert.Fail(t,
			fmt.Sprintf("Detected SetToDefault pointer reuse at %s", fullPath),
			"Types implementing syntax.Defaulter must not reuse pointers across multiple calls. Doing so leads to default values being changed when unmarshaling configuration files. If you're seeing this error, check the path above and ensure that copies are being made of any pointers in all instances of SetToDefault calls where that field is used.",
		)
	}
}

func sharePointer(a, b reflect.Value) (string, bool) {
	// We want to recursively check a and b, so if they're nil they need to be
	// initialized to see if any of their inner values have shared pointers after
	// being initialized with defaults.
	initValue(a)
	initValue(b)

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
			if path, shared := sharePointer(aValue, bValue); shared {
				return path, true
			}
		}
		return "", false

	case reflect.Pointer:
		// same edge case as above: skip if this is a struct ptr and that the structs are empty
		if !a.IsNil() && a.Elem().Kind() == reflect.Struct && a.Elem().NumField() == 0 {
			return "", false
		}
		if pointersMatch(a, b) {
			return "", true
		} else {
			// Recursively navigate inside of the pointer.
			return sharePointer(a.Elem(), b.Elem())
		}

	case reflect.Interface:
		if a.UnsafeAddr() == b.UnsafeAddr() {
			return "", true
		}
		return sharePointer(a.Elem(), b.Elem())

	case reflect.Slice:
		if pointersMatch(a, b) {
			// If the slices are preallocated immutable pointers such as []string{}, we can ignore
			if a.Len() == 0 && a.Cap() == 0 && b.Len() == 0 && b.Cap() == 0 {
				return "", false
			}
			return "", true
		}

		size := min(a.Len(), b.Len())
		for i := range size {
			if path, shared := sharePointer(a.Index(i), b.Index(i)); shared {
				return path, true
			}
		}
		return "", false
	}

	// Recurse into non-pointer types.
	switch a.Kind() {
	case reflect.Array:
		for i := 0; i < a.Len(); i++ {
			if path, shared := sharePointer(a.Index(i), b.Index(i)); shared {
				return path, true
			}
		}
		return "", false

	case reflect.Struct:
		// Check to make sure there are no shared pointers between args1 and args2.
		for i := 0; i < a.NumField(); i++ {
			if path, shared := sharePointer(a.Field(i), b.Field(i)); shared {
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
