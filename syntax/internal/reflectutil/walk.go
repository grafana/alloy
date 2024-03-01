package reflectutil

import (
	"reflect"

	"github.com/grafana/river/internal/rivertags"
)

// GetOrAlloc returns the nested field of value corresponding to index.
// GetOrAlloc panics if not given a struct.
func GetOrAlloc(value reflect.Value, field rivertags.Field) reflect.Value {
	return GetOrAllocIndex(value, field.Index)
}

// GetOrAllocIndex returns the nested field of value corresponding to index.
// GetOrAllocIndex panics if not given a struct.
//
// It is similar to [reflect/Value.FieldByIndex] but can handle traversing
// through nil pointers. If allocate is true, GetOrAllocIndex allocates any
// intermediate nil pointers while traversing the struct.
func GetOrAllocIndex(value reflect.Value, index []int) reflect.Value {
	if len(index) == 1 {
		return value.Field(index[0])
	}

	if value.Kind() != reflect.Struct {
		panic("GetOrAlloc must be given a Struct, but found " + value.Kind().String())
	}

	for _, next := range index {
		value = deferencePointer(value).Field(next)
	}

	return value
}

func deferencePointer(value reflect.Value) reflect.Value {
	for value.Kind() == reflect.Pointer {
		if value.IsNil() {
			value.Set(reflect.New(value.Type().Elem()))
		}
		value = value.Elem()
	}

	return value
}

// Get returns the nested field of value corresponding to index. Get panics if
// not given a struct.
//
// It is similar to [reflect/Value.FieldByIndex] but can handle traversing
// through nil pointers. If Get traverses through a nil pointer, a non-settable
// zero value for the final field is returned.
func Get(value reflect.Value, field rivertags.Field) reflect.Value {
	if len(field.Index) == 1 {
		return value.Field(field.Index[0])
	}

	if value.Kind() != reflect.Struct {
		panic("Get must be given a Struct, but found " + value.Kind().String())
	}

	for i, next := range field.Index {
		for value.Kind() == reflect.Pointer {
			if value.IsNil() {
				return getZero(value, field.Index[i:])
			}
			value = value.Elem()
		}

		value = value.Field(next)
	}

	return value
}

// getZero returns a non-settable zero value while walking value.
func getZero(value reflect.Value, index []int) reflect.Value {
	typ := value.Type()

	for _, next := range index {
		for typ.Kind() == reflect.Pointer {
			typ = typ.Elem()
		}
		typ = typ.Field(next).Type
	}

	return reflect.Zero(typ)
}
