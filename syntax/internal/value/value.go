// Package value holds the internal representation for River values. River
// values act as a lightweight wrapper around reflect.Value.
package value

import (
	"encoding"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/river/internal/reflectutil"
)

// Go types used throughout the package.
var (
	goAny             = reflect.TypeOf((*interface{})(nil)).Elem()
	goString          = reflect.TypeOf(string(""))
	goByteSlice       = reflect.TypeOf([]byte(nil))
	goError           = reflect.TypeOf((*error)(nil)).Elem()
	goTextMarshaler   = reflect.TypeOf((*encoding.TextMarshaler)(nil)).Elem()
	goTextUnmarshaler = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
	goStructWrapper   = reflect.TypeOf(structWrapper{})
	goCapsule         = reflect.TypeOf((*Capsule)(nil)).Elem()
	goDuration        = reflect.TypeOf((time.Duration)(0))
	goDurationPtr     = reflect.TypeOf((*time.Duration)(nil))
	goRiverDefaulter  = reflect.TypeOf((*Defaulter)(nil)).Elem()
	goRiverDecoder    = reflect.TypeOf((*Unmarshaler)(nil)).Elem()
	goRiverValidator  = reflect.TypeOf((*Validator)(nil)).Elem()
	goRawRiverFunc    = reflect.TypeOf((RawFunction)(nil))
	goRiverValue      = reflect.TypeOf(Null)
)

// NOTE(rfratto): This package is extremely sensitive to performance, so
// changes should be made with caution; run benchmarks when changing things.
//
// Value is optimized to be as small as possible and exist fully on the stack.
// This allows many values to avoid allocations, with the exception of creating
// arrays and objects.

// Value represents a River value.
type Value struct {
	rv reflect.Value
	ty Type
}

// Null is the null value.
var Null = Value{}

// Uint returns a Value from a uint64.
func Uint(u uint64) Value { return Value{rv: reflect.ValueOf(u), ty: TypeNumber} }

// Int returns a Value from an int64.
func Int(i int64) Value { return Value{rv: reflect.ValueOf(i), ty: TypeNumber} }

// Float returns a Value from a float64.
func Float(f float64) Value { return Value{rv: reflect.ValueOf(f), ty: TypeNumber} }

// String returns a Value from a string.
func String(s string) Value { return Value{rv: reflect.ValueOf(s), ty: TypeString} }

// Bool returns a Value from a bool.
func Bool(b bool) Value { return Value{rv: reflect.ValueOf(b), ty: TypeBool} }

// Object returns a new value from m. A copy of m is made for producing the
// Value.
func Object(m map[string]Value) Value {
	return Value{
		rv: reflect.ValueOf(m),
		ty: TypeObject,
	}
}

// Array creates an array from the given values. A copy of the vv slice is made
// for producing the Value.
func Array(vv ...Value) Value {
	return Value{
		rv: reflect.ValueOf(vv),
		ty: TypeArray,
	}
}

// Func makes a new function Value from f. Func panics if f does not map to a
// River function.
func Func(f interface{}) Value {
	rv := reflect.ValueOf(f)
	if RiverType(rv.Type()) != TypeFunction {
		panic("river/value: Func called with non-function type")
	}
	return Value{rv: rv, ty: TypeFunction}
}

// Encapsulate creates a new Capsule value from v. Encapsulate panics if v does
// not map to a River capsule.
func Encapsulate(v interface{}) Value {
	rv := reflect.ValueOf(v)
	if RiverType(rv.Type()) != TypeCapsule {
		panic("river/value: Capsule called with non-capsule type")
	}
	return Value{rv: rv, ty: TypeCapsule}
}

// Encode creates a new Value from v. If v is a pointer, v must be considered
// immutable and not change while the Value is used.
func Encode(v interface{}) Value {
	if v == nil {
		return Null
	}
	return makeValue(reflect.ValueOf(v))
}

// FromRaw converts a reflect.Value into a River Value. It is useful to prevent
// downcasting an interface into an any.
func FromRaw(v reflect.Value) Value {
	return makeValue(v)
}

// Type returns the River type for the value.
func (v Value) Type() Type { return v.ty }

// Describe returns a descriptive type name for the value. For capsule values,
// this prints the underlying Go type name. For other values, it prints the
// normal River type.
func (v Value) Describe() string {
	if v.ty != TypeCapsule {
		return v.ty.String()
	}
	return fmt.Sprintf("capsule(%q)", v.rv.Type())
}

// Bool returns the boolean value for v. It panics if v is not a bool.
func (v Value) Bool() bool {
	if v.ty != TypeBool {
		panic("river/value: Bool called on non-bool type")
	}
	return v.rv.Bool()
}

// Number returns a Number value for v. It panics if v is not a Number.
func (v Value) Number() Number {
	if v.ty != TypeNumber {
		panic("river/value: Number called on non-number type")
	}
	return newNumberValue(v.rv)
}

// Int returns an int value for v. It panics if v is not a number.
func (v Value) Int() int64 {
	if v.ty != TypeNumber {
		panic("river/value: Int called on non-number type")
	}
	switch makeNumberKind(v.rv.Kind()) {
	case NumberKindInt:
		return v.rv.Int()
	case NumberKindUint:
		return int64(v.rv.Uint())
	case NumberKindFloat:
		return int64(v.rv.Float())
	}
	panic("river/value: unreachable")
}

// Uint returns an uint value for v. It panics if v is not a number.
func (v Value) Uint() uint64 {
	if v.ty != TypeNumber {
		panic("river/value: Uint called on non-number type")
	}
	switch makeNumberKind(v.rv.Kind()) {
	case NumberKindInt:
		return uint64(v.rv.Int())
	case NumberKindUint:
		return v.rv.Uint()
	case NumberKindFloat:
		return uint64(v.rv.Float())
	}
	panic("river/value: unreachable")
}

// Float returns a float value for v. It panics if v is not a number.
func (v Value) Float() float64 {
	if v.ty != TypeNumber {
		panic("river/value: Float called on non-number type")
	}
	switch makeNumberKind(v.rv.Kind()) {
	case NumberKindInt:
		return float64(v.rv.Int())
	case NumberKindUint:
		return float64(v.rv.Uint())
	case NumberKindFloat:
		return v.rv.Float()
	}
	panic("river/value: unreachable")
}

// Text returns a string value of v. It panics if v is not a string.
func (v Value) Text() string {
	if v.ty != TypeString {
		panic("river/value: Text called on non-string type")
	}

	// Attempt to get an address to v.rv for interface checking.
	//
	// The normal v.rv value is used for other checks.
	addrRV := v.rv
	if addrRV.CanAddr() {
		addrRV = addrRV.Addr()
	}
	switch {
	case addrRV.Type().Implements(goTextMarshaler):
		// TODO(rfratto): what should we do if this fails?
		text, _ := addrRV.Interface().(encoding.TextMarshaler).MarshalText()
		return string(text)

	case v.rv.Type() == goDuration:
		// Special case: v.rv is a duration and its String method should be used.
		return v.rv.Interface().(time.Duration).String()

	default:
		return v.rv.String()
	}
}

// Len returns the length of v. Panics if v is not an array or object.
func (v Value) Len() int {
	switch v.ty {
	case TypeArray:
		return v.rv.Len()
	case TypeObject:
		switch {
		case v.rv.Type() == goStructWrapper:
			return v.rv.Interface().(structWrapper).Len()
		case v.rv.Kind() == reflect.Array, v.rv.Kind() == reflect.Slice: // Array of labeled blocks
			return v.rv.Len()
		case v.rv.Kind() == reflect.Struct:
			return getCachedTags(v.rv.Type()).Len()
		case v.rv.Kind() == reflect.Map:
			return v.rv.Len()
		}
	}
	panic("river/value: Len called on non-array and non-object value")
}

// Index returns index i of the Value. Panics if the value is not an array or
// if it is out of bounds of the array's size.
func (v Value) Index(i int) Value {
	if v.ty != TypeArray {
		panic("river/value: Index called on non-array value")
	}
	return makeValue(v.rv.Index(i))
}

// Interface returns the underlying Go value for the Value.
func (v Value) Interface() interface{} {
	if v.ty == TypeNull {
		return nil
	}
	return v.rv.Interface()
}

// Reflect returns the raw reflection value backing v.
func (v Value) Reflect() reflect.Value { return v.rv }

// makeValue converts a reflect value into a Value, dereferencing any pointers or
// interface{} values.
func makeValue(v reflect.Value) Value {
	// Early check: if v is interface{}, we need to deference it to get the
	// concrete value.
	if v.IsValid() && v.Type() == goAny {
		v = v.Elem()
	}

	// Special case: a reflect.Value may be a value.Value when it's coming from a
	// River array or object. We can unwrap the inner value here before
	// continuing.
	if v.IsValid() && v.Type() == goRiverValue {
		// Unwrap the inner value.
		v = v.Interface().(Value).rv
	}

	// Before we get the River type of the Value, we need to see if it's possible
	// to get a pointer to v. This ensures that if v is a non-pointer field of an
	// addressable struct, still detect the type of v as if it was a pointer.
	if v.CanAddr() {
		v = v.Addr()
	}

	if !v.IsValid() {
		return Null
	}
	riverType := RiverType(v.Type())

	// Finally, deference the pointer fully and use the type we detected.
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return Null
		}
		v = v.Elem()
	}
	return Value{rv: v, ty: riverType}
}

// OrderedKeys reports if v represents an object with consistently ordered
// keys. It panics if v is not an object.
func (v Value) OrderedKeys() bool {
	if v.ty != TypeObject {
		panic("river/value: OrderedKeys called on non-object value")
	}

	// Maps are the only type of unordered River object, since their keys can't
	// be iterated over in a deterministic order. Every other type of River
	// object comes from a struct or a slice where the order of keys stays the
	// same.
	return v.rv.Kind() != reflect.Map
}

// Keys returns the keys in v in unspecified order. It panics if v is not an
// object.
func (v Value) Keys() []string {
	if v.ty != TypeObject {
		panic("river/value: Keys called on non-object value")
	}

	switch {
	case v.rv.Type() == goStructWrapper:
		return v.rv.Interface().(structWrapper).Keys()

	case v.rv.Kind() == reflect.Struct:
		return wrapStruct(v.rv, true).Keys()

	case v.rv.Kind() == reflect.Array, v.rv.Kind() == reflect.Slice:
		// List of labeled blocks.
		labelField, _ := getCachedTags(v.rv.Type().Elem()).LabelField()

		keys := make([]string, v.rv.Len())
		for i := range keys {
			keys[i] = reflectutil.Get(v.rv.Index(i), labelField).String()
		}
		return keys

	case v.rv.Kind() == reflect.Map:
		reflectKeys := v.rv.MapKeys()
		res := make([]string, len(reflectKeys))
		for i, rk := range reflectKeys {
			res[i] = rk.String()
		}
		return res
	}

	panic("river/value: unreachable")
}

// Key returns the value for a key in v. It panics if v is not an object. ok
// will be false if the key did not exist in the object.
func (v Value) Key(key string) (index Value, ok bool) {
	if v.ty != TypeObject {
		panic("river/value: Key called on non-object value")
	}

	switch {
	case v.rv.Type() == goStructWrapper:
		return v.rv.Interface().(structWrapper).Key(key)
	case v.rv.Kind() == reflect.Struct:
		// We return the struct with the label intact.
		return wrapStruct(v.rv, true).Key(key)
	case v.rv.Kind() == reflect.Map:
		val := v.rv.MapIndex(reflect.ValueOf(key))
		if !val.IsValid() {
			return Null, false
		}
		return makeValue(val), true

	case v.rv.Kind() == reflect.Slice, v.rv.Kind() == reflect.Array:
		// List of labeled blocks.
		labelField, _ := getCachedTags(v.rv.Type().Elem()).LabelField()

		for i := 0; i < v.rv.Len(); i++ {
			elem := v.rv.Index(i)

			label := reflectutil.Get(elem, labelField).String()
			if label == key {
				// We discard the label since the key here represents the label value.
				ws := wrapStruct(elem, false)
				return ws.Value(), true
			}
		}
	default:
		panic("river/value: unreachable")
	}

	return
}

// Call invokes a function value with the provided arguments. It panics if v is
// not a function. If v is a variadic function, args should be the full flat
// list of arguments.
//
// An ArgError will be returned if one of the arguments is invalid. An Error
// will be returned if the function call returns an error or if the number of
// arguments doesn't match.
func (v Value) Call(args ...Value) (Value, error) {
	if v.ty != TypeFunction {
		panic("river/value: Call called on non-function type")
	}

	if v.rv.Type() == goRawRiverFunc {
		return v.rv.Interface().(RawFunction)(v, args...)
	}

	var (
		variadic     = v.rv.Type().IsVariadic()
		expectedArgs = v.rv.Type().NumIn()
	)

	if variadic && len(args) < expectedArgs-1 {
		return Null, Error{
			Value: v,
			Inner: fmt.Errorf("expected at least %d args, got %d", expectedArgs-1, len(args)),
		}
	} else if !variadic && len(args) != expectedArgs {
		return Null, Error{
			Value: v,
			Inner: fmt.Errorf("expected %d args, got %d", expectedArgs, len(args)),
		}
	}

	reflectArgs := make([]reflect.Value, len(args))
	for i, arg := range args {
		var argVal reflect.Value
		if variadic && i >= expectedArgs-1 {
			argType := v.rv.Type().In(expectedArgs - 1).Elem()
			argVal = reflect.New(argType).Elem()
		} else {
			argType := v.rv.Type().In(i)
			argVal = reflect.New(argType).Elem()
		}

		var d decoder
		if err := d.decode(arg, argVal); err != nil {
			return Null, ArgError{
				Function: v,
				Argument: arg,
				Index:    i,
				Inner:    err,
			}
		}

		reflectArgs[i] = argVal
	}

	outs := v.rv.Call(reflectArgs)
	switch len(outs) {
	case 1:
		return makeValue(outs[0]), nil
	case 2:
		// When there's 2 return values, the second is always an error.
		err, _ := outs[1].Interface().(error)
		if err != nil {
			return Null, Error{Value: v, Inner: err}
		}
		return makeValue(outs[0]), nil

	default:
		// It's not possible to reach here; we enforce that function values always
		// have 1 or 2 return values.
		panic("river/value: unreachable")
	}
}

func convertValue(val Value, toType Type) (Value, error) {
	// TODO(rfratto): Use vm benchmarks to see if making this a method on Value
	// changes anything.

	fromType := val.Type()

	if fromType == toType {
		// no-op: val is already the right kind.
		return val, nil
	}

	switch fromType {
	case TypeNumber:
		switch toType {
		case TypeString: // number -> string
			strVal := newNumberValue(val.rv).ToString()
			return makeValue(reflect.ValueOf(strVal)), nil
		}

	case TypeString:
		sourceStr := val.rv.String()

		switch toType {
		case TypeNumber: // string -> number
			switch {
			case sourceStr == "":
				return Null, TypeError{Value: val, Expected: toType}

			case sourceStr[0] == '-':
				// String starts with a -; parse as a signed int.
				parsed, err := strconv.ParseInt(sourceStr, 10, 64)
				if err != nil {
					return Null, TypeError{Value: val, Expected: toType}
				}
				return Int(parsed), nil
			case strings.ContainsAny(sourceStr, ".eE"):
				// String contains something that a floating-point number would use;
				// convert.
				parsed, err := strconv.ParseFloat(sourceStr, 64)
				if err != nil {
					return Null, TypeError{Value: val, Expected: toType}
				}
				return Float(parsed), nil
			default:
				// Otherwise, treat the number as an unsigned int.
				parsed, err := strconv.ParseUint(sourceStr, 10, 64)
				if err != nil {
					return Null, TypeError{Value: val, Expected: toType}
				}
				return Uint(parsed), nil
			}
		}
	}

	return Null, TypeError{Value: val, Expected: toType}
}

func convertGoNumber(nval Number, target reflect.Type) reflect.Value {
	switch target.Kind() {
	case reflect.Int:
		return reflect.ValueOf(int(nval.Int()))
	case reflect.Int8:
		return reflect.ValueOf(int8(nval.Int()))
	case reflect.Int16:
		return reflect.ValueOf(int16(nval.Int()))
	case reflect.Int32:
		return reflect.ValueOf(int32(nval.Int()))
	case reflect.Int64:
		return reflect.ValueOf(nval.Int())
	case reflect.Uint:
		return reflect.ValueOf(uint(nval.Uint()))
	case reflect.Uint8:
		return reflect.ValueOf(uint8(nval.Uint()))
	case reflect.Uint16:
		return reflect.ValueOf(uint16(nval.Uint()))
	case reflect.Uint32:
		return reflect.ValueOf(uint32(nval.Uint()))
	case reflect.Uint64:
		return reflect.ValueOf(nval.Uint())
	case reflect.Float32:
		return reflect.ValueOf(float32(nval.Float()))
	case reflect.Float64:
		return reflect.ValueOf(nval.Float())
	}

	panic("unsupported number conversion")
}
