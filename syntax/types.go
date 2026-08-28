package syntax

import "github.com/grafana/alloy/syntax/internal/value"

// Our types in this file are re-implementations of interfaces from
// value.Capsule. They are *not* defined as type aliases, since pkg.go.dev
// would show the type alias instead of the contents of that type (which IMO is
// a frustrating user experience).
//
// The types below must be kept in sync with the internal package, and the
// checks below ensure they're compatible.
var (
	_ value.Defaulter              = (Defaulter)(nil)
	_ value.Unmarshaler            = (Unmarshaler)(nil)
	_ value.Validator              = (Validator)(nil)
	_ value.Capsule                = (Capsule)(nil)
	_ value.ConvertibleFromCapsule = (ConvertibleFromCapsule)(nil)
	_ value.ConvertibleIntoCapsule = (ConvertibleIntoCapsule)(nil)
)

// The Unmarshaler interface allows a type to hook into Alloy syntax decoding
// and decode into another type or provide pre-decoding logic.
type Unmarshaler interface {
	// UnmarshalAlloy is invoked when decoding a Alloy value into a Go value. f
	// should be called with a pointer to a value to decode into. UnmarshalAlloy
	// will not be called on types which are squashed into the parent struct
	// using `alloy:",squash"`.
	UnmarshalAlloy(f func(v any) error) error
}

// The Defaulter interface allows a type to implement default functionality
// in Alloy configuration evaluation.
type Defaulter interface {
	// SetToDefault is called when evaluating a block or body to set the value
	// to its defaults. SetToDefault will not be called on types which are
	// squashed into the parent struct using `alloy:",squash"`.
	SetToDefault()
}

// The Validator interface allows a type to implement validation functionality
// in Alloy configuration evaluation.
type Validator interface {
	// Validate is called when evaluating a block or body to enforce the
	// value is valid. Validate will not be called on types which are
	// squashed into the parent struct using `alloy:",squash"`.
	Validate() error
}

// Capsule is an interface marker which tells Alloy syntax that a type should
// always be treated as a "capsule type" instead of the default type decoding
// would assign.
//
// Capsule types are useful for passing around arbitrary Go values in Alloy
// expressions and for declaring new synthetic types with custom conversion
// rules.
//
// By default, only two capsule values of the same underlying Go type are
// compatible. Types which implement ConvertibleFromCapsule or
// ConvertibleToCapsule can provide custom logic for conversions from and to
// other types.
type Capsule interface {
	// AlloyCapsule marks the type as a Capsule. AlloyCapsule is never invoked by
	// Alloy.
	AlloyCapsule()
}

// ErrNoConversion is returned by implementations of ConvertibleFromCapsule and
// ConvertibleToCapsule when a conversion with a specific type is unavailable.
//
// Returning this error causes Alloy to fall back to default conversion rules.
var ErrNoConversion = value.ErrNoConversion

// ConvertibleFromCapsule is a Capsule which supports custom conversion from
// any Go type which is not the same as the capsule type.
type ConvertibleFromCapsule interface {
	Capsule

	// ConvertFrom updates the ConvertibleFromCapsule value based on the value of
	// src. src may be any Go value, not just other capsules.
	//
	// ConvertFrom should return ErrNoConversion if no conversion is available
	// from src. Other errors are treated as an Alloy decoding error.
	ConvertFrom(src any) error
}

// ConvertibleIntoCapsule is a Capsule which supports custom conversion into
// any Go type which is not the same as the capsule type.
type ConvertibleIntoCapsule interface {
	Capsule

	// ConvertInto should convert its value and store it into dst. dst will be a
	// pointer to a Go value of any type.
	//
	// ConvertInto should return ErrNoConversion if no conversion into dst is
	// available. Other errors are treated as an Alloy decoding error.
	ConvertInto(dst any) error
}

// Value represents an Alloy value. See the value.Value for more details.
type Value = value.Value

// ValueFromString creates a new Value from a given string.
var ValueFromString = value.String

// ValueFromBool creates a new Value from a given bool.
var ValueFromBool = value.Bool
