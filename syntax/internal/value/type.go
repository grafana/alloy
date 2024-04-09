package value

import (
	"fmt"
	"reflect"
)

// Type represents the type of an Alloy value loosely. For example, a Value may
// be TypeArray, but this does not imply anything about the type of that
// array's elements (all of which may be any type).
//
// TypeCapsule is a special type which encapsulates arbitrary Go values.
type Type uint8

// Supported Type values.
const (
	TypeNull Type = iota
	TypeNumber
	TypeString
	TypeBool
	TypeArray
	TypeObject
	TypeFunction
	TypeCapsule
)

var typeStrings = [...]string{
	TypeNull:     "null",
	TypeNumber:   "number",
	TypeString:   "string",
	TypeBool:     "bool",
	TypeArray:    "array",
	TypeObject:   "object",
	TypeFunction: "function",
	TypeCapsule:  "capsule",
}

// String returns the name of t.
func (t Type) String() string {
	if int(t) < len(typeStrings) {
		return typeStrings[t]
	}
	return fmt.Sprintf("Type(%d)", t)
}

// GoString returns the name of t.
func (t Type) GoString() string { return t.String() }

// AlloyType returns the Alloy type from the Go type.
//
// Go types map to Alloy types using the following rules:
//
//  1. Go numbers (ints, uints, floats) map to an Alloy number.
//  2. Go strings map to an Alloy string.
//  3. Go bools map to an Alloy bool.
//  4. Go arrays and slices map to an Alloy array.
//  5. Go map[string]T map to an Alloy object.
//  6. Go structs map to an Alloy object, provided they have at least one field
//     with an alloy tag.
//  7. Valid Go functions map to an Alloy function.
//  8. Go interfaces map to an Alloy capsule.
//  9. All other Go values map to an Alloy capsule.
//
// Go functions are only valid for Alloy if they have one non-error return type
// (the first return type) and one optional error return type (the second
// return type). Other function types are treated as capsules.
//
// As an exception, any type which implements the Capsule interface is forced
// to be a capsule.
func AlloyType(t reflect.Type) Type {
	// We don't know if the AlloyCapsule interface is implemented for a pointer
	// or non-pointer type, so we have to check before and after dereferencing.

	for t.Kind() == reflect.Pointer {
		switch {
		case t.Implements(goCapsule):
			return TypeCapsule
		case t.Implements(goTextMarshaler):
			return TypeString
		}

		t = t.Elem()
	}

	switch {
	case t.Implements(goCapsule):
		return TypeCapsule
	case t.Implements(goTextMarshaler):
		return TypeString
	case t == goDuration:
		return TypeString
	}

	switch t.Kind() {
	case reflect.Invalid:
		return TypeNull

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return TypeNumber
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return TypeNumber
	case reflect.Float32, reflect.Float64:
		return TypeNumber

	case reflect.String:
		return TypeString

	case reflect.Bool:
		return TypeBool

	case reflect.Array, reflect.Slice:
		if inner := t.Elem(); inner.Kind() == reflect.Struct {
			if _, labeled := getCachedTags(inner).LabelField(); labeled {
				// An slice/array of labeled blocks is an object, where each label is a
				// top-level key.
				return TypeObject
			}
		}
		return TypeArray

	case reflect.Map:
		if t.Key() != goString {
			// Objects must be keyed by string. Anything else is forced to be a
			// Capsule.
			return TypeCapsule
		}
		return TypeObject

	case reflect.Struct:
		if getCachedTags(t).Len() == 0 {
			return TypeCapsule
		}
		return TypeObject

	case reflect.Func:
		switch t.NumOut() {
		case 1:
			if t.Out(0) == goError {
				return TypeCapsule
			}
			return TypeFunction
		case 2:
			if t.Out(0) == goError || t.Out(1) != goError {
				return TypeCapsule
			}
			return TypeFunction
		default:
			return TypeCapsule
		}

	case reflect.Interface:
		return TypeCapsule

	default:
		return TypeCapsule
	}
}
