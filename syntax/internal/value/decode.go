package value

import (
	"encoding"
	"errors"
	"fmt"
	"math"
	"reflect"
	"time"

	"github.com/grafana/river/internal/reflectutil"
)

// The Defaulter interface allows a type to implement default functionality
// in River evaluation.
//
// Defaulter will be called only on block and body river types.
//
// When using nested blocks, the wrapping type must also implement
// Defaulter to propagate the defaults of the wrapped type. Otherwise,
// defaults used for the wrapped type become inconsistent:
//
//   - If the wrapped block is NOT defined in the River config, the wrapping
//     type's defaults are used.
//   - If the wrapped block IS defined in the River config, the wrapped type's
//     defaults are used.
type Defaulter interface {
	// SetToDefault is called when evaluating a block or body to set the value
	// to its defaults.
	SetToDefault()
}

// Unmarshaler is a custom type which can be used to hook into the decoder.
type Unmarshaler interface {
	// UnmarshalRiver is called when decoding a value. f should be invoked to
	// continue decoding with a value to decode into.
	UnmarshalRiver(f func(v interface{}) error) error
}

// The Validator interface allows a type to implement validation functionality
// in River evaluation.
type Validator interface {
	// Validate is called when evaluating a block or body to enforce the
	// value is valid.
	Validate() error
}

// Decode assigns a Value val to a Go pointer target. Pointers will be
// allocated as necessary when decoding.
//
// As a performance optimization, the underlying Go value of val will be
// assigned directly to target if the Go types match. This means that pointers,
// slices, and maps will be passed by reference. Callers should take care not
// to modify any Values after decoding, unless it is expected by the contract
// of the type (i.e., when the type exposes a goroutine-safe API). In other
// cases, new maps and slices will be allocated as necessary. Call DecodeCopy
// to make a copy of val instead.
//
// When a direct assignment is not done, Decode first checks to see if target
// implements the Unmarshaler or text.Unmarshaler interface, invoking methods
// as appropriate. It will also use time.ParseDuration if target is
// *time.Duration.
//
// Next, Decode will attempt to convert val to the type expected by target for
// assignment. If val or target implement ConvertibleCapsule, conversion
// between values will be attempted by calling ConvertFrom and ConvertInto as
// appropriate. If val cannot be converted, an error is returned.
//
// River null values will decode into a nil Go pointer or the zero value for
// the non-pointer type.
//
// Decode will panic if target is not a pointer.
func Decode(val Value, target interface{}) error {
	rt := reflect.ValueOf(target)
	if rt.Kind() != reflect.Pointer {
		panic("river/value: Decode called with non-pointer value")
	}

	var d decoder
	return d.decode(val, rt)
}

// DecodeCopy is like Decode but a deep copy of val is always made.
//
// Unlike Decode, DecodeCopy will always invoke Unmarshaler and
// text.Unmarshaler interfaces (if implemented by target).
func DecodeCopy(val Value, target interface{}) error {
	rt := reflect.ValueOf(target)
	if rt.Kind() != reflect.Pointer {
		panic("river/value: Decode called with non-pointer value")
	}

	d := decoder{makeCopy: true}
	return d.decode(val, rt)
}

type decoder struct {
	makeCopy bool
}

func (d *decoder) decode(val Value, into reflect.Value) (err error) {
	// If everything has decoded successfully, run Validate if implemented.
	defer func() {
		if err == nil {
			if into.CanAddr() && into.Addr().Type().Implements(goRiverValidator) {
				err = into.Addr().Interface().(Validator).Validate()
			} else if into.Type().Implements(goRiverValidator) {
				err = into.Interface().(Validator).Validate()
			}
		}
	}()

	// Store the raw value from val and try to address it so we can do underlying
	// type match assignment.
	rawValue := val.rv
	if rawValue.CanAddr() {
		rawValue = rawValue.Addr()
	}

	// Fully deference into and allocate pointers as necessary.
	for into.Kind() == reflect.Pointer {
		// Check for direct assignments before allocating pointers and dereferencing.
		// This preserves pointer addresses when decoding an *int into an *int.
		switch {
		case into.CanSet() && val.Type() == TypeNull:
			into.Set(reflect.Zero(into.Type()))
			return nil
		case into.CanSet() && d.canDirectlyAssign(rawValue.Type(), into.Type()):
			into.Set(rawValue)
			return nil
		case into.CanSet() && d.canDirectlyAssign(val.rv.Type(), into.Type()):
			into.Set(val.rv)
			return nil
		}

		if into.IsNil() {
			into.Set(reflect.New(into.Type().Elem()))
		}
		into = into.Elem()
	}

	// Ww need to preform the same switch statement as above after the loop to
	// check for direct assignment one more time on the fully deferenced types.
	//
	// NOTE(rfratto): we skip the rawValue assignment check since that's meant
	// for assigning pointers, and into is never a pointer when we reach here.
	switch {
	case into.CanSet() && val.Type() == TypeNull:
		into.Set(reflect.Zero(into.Type()))
		return nil
	case into.CanSet() && d.canDirectlyAssign(val.rv.Type(), into.Type()):
		into.Set(val.rv)
		return nil
	}

	// Special decoding rules:
	//
	// 1. If into is an interface{}, go through decodeAny so it gets assigned
	//    predictable types.
	// 2. If into implements a supported interface, use the interface for
	//    decoding instead.
	if into.Type() == goAny {
		return d.decodeAny(val, into)
	} else if ok, err := d.decodeFromInterface(val, into); ok {
		return err
	}

	if into.CanAddr() && into.Addr().Type().Implements(goRiverDefaulter) {
		into.Addr().Interface().(Defaulter).SetToDefault()
	} else if into.Type().Implements(goRiverDefaulter) {
		into.Interface().(Defaulter).SetToDefault()
	}

	targetType := RiverType(into.Type())

	// Track a value to use for decoding. This value will be updated if
	// conversion is necessary.
	//
	// NOTE(rfratto): we don't reassign to val here, since Go 1.18 thinks that
	// means it escapes the heap. We need to create a local variable to avoid
	// extra allocations.
	convVal := val

	// Convert the value.
	switch {
	case val.rv.Type() == goByteSlice && into.Type() == goString: // []byte -> string
		into.Set(val.rv.Convert(goString))
		return nil
	case val.rv.Type() == goString && into.Type() == goByteSlice: // string -> []byte
		into.Set(val.rv.Convert(goByteSlice))
		return nil
	case convVal.Type() != targetType:
		converted, err := tryCapsuleConvert(convVal, into, targetType)
		if err != nil {
			return err
		} else if converted {
			return nil
		}

		convVal, err = convertValue(convVal, targetType)
		if err != nil {
			return err
		}
	}

	// Slowest case: recursive decoding. Once we've reached this point, we know
	// that convVal.rv and into are compatible Go types.
	switch convVal.Type() {
	case TypeNumber:
		into.Set(convertGoNumber(convVal.Number(), into.Type()))
		return nil
	case TypeString:
		// Call convVal.Text() to get the final string value, since convVal.rv
		// might not be a string.
		into.Set(reflect.ValueOf(convVal.Text()))
		return nil
	case TypeBool:
		into.Set(reflect.ValueOf(convVal.Bool()))
		return nil
	case TypeArray:
		return d.decodeArray(convVal, into)
	case TypeObject:
		return d.decodeObject(convVal, into)
	case TypeFunction:
		// The Go types for two functions must be the same.
		//
		// TODO(rfratto): we may want to consider being more lax here, potentially
		// creating an adapter between the two functions.
		if convVal.rv.Type() == into.Type() {
			into.Set(convVal.rv)
			return nil
		}

		return Error{
			Value: val,
			Inner: fmt.Errorf("expected function(%s), got function(%s)", into.Type(), convVal.rv.Type()),
		}
	case TypeCapsule:
		// The Go types for the capsules must be the same or able to be converted.
		if convVal.rv.Type() == into.Type() {
			into.Set(convVal.rv)
			return nil
		}

		converted, err := tryCapsuleConvert(convVal, into, targetType)
		if err != nil {
			return err
		} else if converted {
			return nil
		}

		// TODO(rfratto): return a TypeError for this instead. TypeError isn't
		// appropriate at the moment because it would just print "capsule", which
		// doesn't contain all the information the user would want to know (e.g., a
		// capsule of what inner type?).
		return Error{
			Value: val,
			Inner: fmt.Errorf("expected capsule(%q), got %s", into.Type(), convVal.Describe()),
		}
	default:
		panic("river/value: unexpected kind " + convVal.Type().String())
	}
}

// canDirectlyAssign returns true if the `from` type can be directly asssigned
// to the `into` type. This always returns false if the decoder is set to make
// copies or into contains an interface{} type anywhere in its type definition
// to allow for decoding interfaces{} into a set of known types.
func (d *decoder) canDirectlyAssign(from reflect.Type, into reflect.Type) bool {
	if d.makeCopy {
		return false
	}
	if from != into {
		return false
	}
	return !containsAny(into)
}

// containsAny recursively traverses through into, returning true if it
// contains an interface{} value anywhere in its structure.
func containsAny(into reflect.Type) bool {
	// TODO(rfratto): cache result of this function?

	if into == goAny {
		return true
	}

	switch into.Kind() {
	case reflect.Array, reflect.Pointer, reflect.Slice:
		return containsAny(into.Elem())
	case reflect.Map:
		if into.Key() == goString {
			return containsAny(into.Elem())
		}
		return false

	case reflect.Struct:
		for i := 0; i < into.NumField(); i++ {
			if containsAny(into.Field(i).Type) {
				return true
			}
		}
		return false

	default:
		// Other kinds are not River types where the decodeAny check applies.
		return false
	}
}

func (d *decoder) decodeFromInterface(val Value, into reflect.Value) (ok bool, err error) {
	// into may only implement interface types for a pointer receiver, so we want
	// to address into if possible.
	if into.CanAddr() {
		into = into.Addr()
	}

	switch {
	case into.Type() == goDurationPtr:
		var s string
		err := d.decode(val, reflect.ValueOf(&s))
		if err != nil {
			return true, err
		}
		dur, err := time.ParseDuration(s)
		if err != nil {
			return true, Error{Value: val, Inner: err}
		}
		*into.Interface().(*time.Duration) = dur
		return true, nil

	case into.Type().Implements(goRiverDecoder):
		err := into.Interface().(Unmarshaler).UnmarshalRiver(func(v interface{}) error {
			return d.decode(val, reflect.ValueOf(v))
		})
		if err != nil {
			// TODO(rfratto): we need to detect if error is one of the error types
			// from this package and only wrap it in an Error if it isn't.
			return true, Error{Value: val, Inner: err}
		}
		return true, nil

	case into.Type().Implements(goTextUnmarshaler):
		var s string
		err := d.decode(val, reflect.ValueOf(&s))
		if err != nil {
			return true, err
		}
		err = into.Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(s))
		if err != nil {
			return true, Error{Value: val, Inner: err}
		}
		return true, nil
	}

	return false, nil
}

func tryCapsuleConvert(from Value, into reflect.Value, intoType Type) (ok bool, err error) {
	// Check to see if we can use capsule conversion.
	if from.Type() == TypeCapsule {
		cc, ok := from.Interface().(ConvertibleIntoCapsule)
		if ok {
			// It's always possible to Addr the reflect.Value below since we expect
			// it to be a settable non-pointer value.
			err := cc.ConvertInto(into.Addr().Interface())
			if err == nil {
				return true, nil
			} else if err != nil && !errors.Is(err, ErrNoConversion) {
				return false, Error{Value: from, Inner: err}
			}
		}
	}

	if intoType == TypeCapsule {
		cc, ok := into.Addr().Interface().(ConvertibleFromCapsule)
		if ok {
			err := cc.ConvertFrom(from.Interface())
			if err == nil {
				return true, nil
			} else if err != nil && !errors.Is(err, ErrNoConversion) {
				return false, Error{Value: from, Inner: err}
			}
		}
	}

	// Last attempt: allow converting two capsules if the Go types are compatible
	// and the into kind is an interface.
	//
	// TODO(rfratto): we may consider expanding this to allowing conversion to
	// any compatible Go type in the future (not just interfaces).
	if from.Type() == TypeCapsule && intoType == TypeCapsule && into.Kind() == reflect.Interface {
		// We try to convert a pointer to from first to avoid making unnecessary
		// copies.
		if from.Reflect().CanAddr() && from.Reflect().Addr().CanConvert(into.Type()) {
			val := from.Reflect().Addr().Convert(into.Type())
			into.Set(val)
			return true, nil
		} else if from.Reflect().CanConvert(into.Type()) {
			val := from.Reflect().Convert(into.Type())
			into.Set(val)
			return true, nil
		}
	}

	return false, nil
}

// decodeAny is invoked by decode when into is an interface{}. We assign the
// interface{} a known type based on the River value being decoded:
//
//	Null values:   nil
//	Number values: float64, int, int64, or uint64.
//	               If the underlying type is a float, always decode to a float64.
//	               For non-floats the order of preference is int -> int64 -> uint64.
//	Arrays:        []interface{}
//	Objects:       map[string]interface{}
//	Bool:          bool
//	String:        string
//	Function:      Passthrough of the underlying function value
//	Capsule:       Passthrough of the underlying capsule value
//
// In the cases where we do not pass through the underlying value, we create a
// value of that type, recursively call decode to populate that new value, and
// then store that value into the interface{}.
func (d *decoder) decodeAny(val Value, into reflect.Value) error {
	var ptr reflect.Value

	switch val.Type() {
	case TypeNull:
		into.Set(reflect.Zero(into.Type()))
		return nil

	case TypeNumber:

		switch val.Number().Kind() {
		case NumberKindFloat:
			var v float64
			ptr = reflect.ValueOf(&v)
		case NumberKindUint:
			uint64Val := val.Uint()
			if uint64Val <= math.MaxInt {
				var v int
				ptr = reflect.ValueOf(&v)
			} else if uint64Val <= math.MaxInt64 {
				var v int64
				ptr = reflect.ValueOf(&v)
			} else {
				var v uint64
				ptr = reflect.ValueOf(&v)
			}
		case NumberKindInt:
			int64Val := val.Int()
			if math.MinInt <= int64Val && int64Val <= math.MaxInt {
				var v int
				ptr = reflect.ValueOf(&v)
			} else {
				var v int64
				ptr = reflect.ValueOf(&v)
			}

		default:
			panic("river/value: unreachable")
		}

	case TypeArray:
		var v []interface{}
		ptr = reflect.ValueOf(&v)

	case TypeObject:
		var v map[string]interface{}
		ptr = reflect.ValueOf(&v)

	case TypeBool:
		var v bool
		ptr = reflect.ValueOf(&v)

	case TypeString:
		var v string
		ptr = reflect.ValueOf(&v)

	case TypeFunction, TypeCapsule:
		// Functions and capsules must be directly assigned since there's no
		// "generic" representation for either.
		//
		// We retain the pointer if we were given a pointer.

		if val.rv.CanAddr() {
			into.Set(val.rv.Addr())
			return nil
		}

		into.Set(val.rv)
		return nil

	default:
		panic("river/value: unreachable")
	}

	if err := d.decode(val, ptr); err != nil {
		return err
	}
	into.Set(ptr.Elem())
	return nil
}

func (d *decoder) decodeArray(val Value, rt reflect.Value) error {
	switch rt.Kind() {
	case reflect.Slice:
		res := reflect.MakeSlice(rt.Type(), val.Len(), val.Len())
		for i := 0; i < val.Len(); i++ {
			// Decode the original elements into the new elements.
			if err := d.decode(val.Index(i), res.Index(i)); err != nil {
				return ElementError{Value: val, Index: i, Inner: err}
			}
		}
		rt.Set(res)

	case reflect.Array:
		res := reflect.New(rt.Type()).Elem()

		if val.Len() != res.Len() {
			return Error{
				Value: val,
				Inner: fmt.Errorf("array must have exactly %d elements, got %d", res.Len(), val.Len()),
			}
		}

		for i := 0; i < val.Len(); i++ {
			if err := d.decode(val.Index(i), res.Index(i)); err != nil {
				return ElementError{Value: val, Index: i, Inner: err}
			}
		}
		rt.Set(res)

	default:
		panic(fmt.Sprintf("river/value: unexpected array type %s", val.rv.Kind()))
	}

	return nil
}

func (d *decoder) decodeObject(val Value, rt reflect.Value) error {
	switch rt.Kind() {
	case reflect.Struct:
		targetTags := getCachedTags(rt.Type())
		return d.decodeObjectToStruct(val, rt, targetTags, false)

	case reflect.Slice, reflect.Array: // Slice of labeled blocks
		keys := val.Keys()

		var res reflect.Value

		if rt.Kind() == reflect.Slice {
			res = reflect.MakeSlice(rt.Type(), len(keys), len(keys))
		} else { // Array
			res = reflect.New(rt.Type()).Elem()

			if res.Len() != len(keys) {
				return Error{
					Value: val,
					Inner: fmt.Errorf("object must have exactly %d keys, got %d", res.Len(), len(keys)),
				}
			}
		}

		fields := getCachedTags(rt.Type().Elem())
		labelField, _ := fields.LabelField()

		for i, key := range keys {
			// First decode the key into the label.
			elem := res.Index(i)
			reflectutil.GetOrAlloc(elem, labelField).Set(reflect.ValueOf(key))

			// Now decode the inner object.
			value, _ := val.Key(key)
			if err := d.decodeObjectToStruct(value, elem, fields, true); err != nil {
				return FieldError{Value: val, Field: key, Inner: err}
			}
		}
		rt.Set(res)

	case reflect.Map:
		if rt.Type().Key() != goString {
			// Maps with non-string types are treated as capsules and can't be
			// decoded from maps.
			return TypeError{Value: val, Expected: RiverType(rt.Type())}
		}

		res := reflect.MakeMapWithSize(rt.Type(), val.Len())

		// Create a shared value to decode each element into. This will be zeroed
		// out for each key, and then copied when setting the map index.
		into := reflect.New(rt.Type().Elem()).Elem()
		intoZero := reflect.Zero(into.Type())

		for i, key := range val.Keys() {
			// We ignore the ok value because we know it exists.
			value, _ := val.Key(key)

			// Zero out the value if it was decoded in the previous loop.
			if i > 0 {
				into.Set(intoZero)
			}
			// Decode into our element.
			if err := d.decode(value, into); err != nil {
				return FieldError{Value: val, Field: key, Inner: err}
			}

			// Then set the map index.
			res.SetMapIndex(reflect.ValueOf(key), into)
		}

		rt.Set(res)

	default:
		panic(fmt.Sprintf("river/value: unexpected target type %s", rt.Kind()))
	}

	return nil
}

func (d *decoder) decodeObjectToStruct(val Value, rt reflect.Value, fields *objectFields, decodedLabel bool) error {
	// TODO(rfratto): this needs to check for required keys being set

	for _, key := range val.Keys() {
		// We ignore the ok value because we know it exists.
		value, _ := val.Key(key)

		// Struct labels should be decoded first, since objects are wrapped in
		// labels. If we have yet to decode the label, decode it now.
		if lf, ok := fields.LabelField(); ok && !decodedLabel {
			// Safety check: if the inner field isn't an object, there's something
			// wrong here. It's unclear if a user can craft an expression that hits
			// this case, but it's left in for safety.
			if value.Type() != TypeObject {
				return FieldError{
					Value: val,
					Field: key,
					Inner: TypeError{Value: value, Expected: TypeObject},
				}
			}

			// Decode the key into the label.
			reflectutil.GetOrAlloc(rt, lf).Set(reflect.ValueOf(key))

			// ...and then code the rest of the object.
			if err := d.decodeObjectToStruct(value, rt, fields, true); err != nil {
				return err
			}
			continue
		}

		switch fields.Has(key) {
		case objectKeyTypeInvalid:
			return MissingKeyError{Value: value, Missing: key}
		case objectKeyTypeNestedField: // Block with multiple name fragments
			next, _ := fields.NestedField(key)
			// Recurse the call with the inner value.
			if err := d.decodeObjectToStruct(value, rt, next, decodedLabel); err != nil {
				return err
			}
		case objectKeyTypeField: // Single-name fragment
			targetField, _ := fields.Field(key)
			targetValue := reflectutil.GetOrAlloc(rt, targetField)

			if err := d.decode(value, targetValue); err != nil {
				return FieldError{Value: val, Field: key, Inner: err}
			}
		}
	}

	return nil
}
