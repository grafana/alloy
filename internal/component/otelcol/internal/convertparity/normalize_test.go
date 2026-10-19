package convertparity_test

import (
	"reflect"
	"strings"
	"unsafe"
)

// normalize returns a copy of v rewritten to its runtime-effective form, so
// representations upstream treats identically compare equal:
//   - configoptional: only Some has a value at runtime (HasValue), so Default and None both reduce to the zero Optional.
func normalize(v any) any {
	if v == nil {
		return nil
	}
	rv := reflect.ValueOf(v)
	cp := reflect.New(rv.Type()).Elem()
	cp.Set(rv)
	walk(cp, map[uintptr]bool{})
	return cp.Interface()
}

func walk(v reflect.Value, seen map[uintptr]bool) {
	if !v.IsValid() {
		return
	}
	v = settable(v)
	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() || seen[v.Pointer()] {
			return
		}
		seen[v.Pointer()] = true
		walk(v.Elem(), seen)
		return
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		inner := v.Elem()
		cp := reflect.New(inner.Type()).Elem()
		cp.Set(inner)
		walk(cp, seen)
		v.Set(cp)
		return
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			walk(v.Index(i), seen)
		}
		return
	case reflect.Map:
		if v.IsNil() {
			return
		}
		for _, k := range v.MapKeys() {
			e := v.MapIndex(k)
			cp := reflect.New(e.Type()).Elem()
			cp.Set(e)
			walk(cp, seen)
			v.SetMapIndex(k, cp)
		}
		return
	case reflect.Struct:
	default:
		return
	}

	if isOptional(v.Type()) {
		if f := settable(v.FieldByName("flavor")); f.IsValid() && f.Int() != 2 {
			v.Set(reflect.Zero(v.Type()))
			return
		}
	}
	for i := 0; i < v.NumField(); i++ {
		walk(v.Field(i), seen)
	}
}

func isOptional(t reflect.Type) bool {
	return t.PkgPath() == "go.opentelemetry.io/collector/config/configoptional" && strings.HasPrefix(t.Name(), "Optional[")
}

// settable makes unexported fields of an addressable value writable.
func settable(v reflect.Value) reflect.Value {
	if v.CanSet() || !v.CanAddr() {
		return v
	}
	return reflect.NewAt(v.Type(), unsafe.Pointer(v.UnsafeAddr())).Elem()
}
