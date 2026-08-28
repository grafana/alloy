package value

import "fmt"

// Error is used for reporting on a value-level error. It is the most general
// type of error for a value.
type Error struct {
	Value Value
	Inner error
}

// TypeError is used for reporting on a value having an unexpected type.
type TypeError struct {
	// Value which caused the error.
	Value    Value
	Expected Type
}

// Error returns the string form of the TypeError.
func (te TypeError) Error() string {
	return fmt.Sprintf("expected %s, got %s", te.Expected, te.Value.Type())
}

// Error returns the message of the decode error.
func (de Error) Error() string { return de.Inner.Error() }

// MissingKeyError is used for reporting that a value is missing a key.
type MissingKeyError struct {
	Value   Value
	Missing string
}

// Error returns the string form of the MissingKeyError.
func (mke MissingKeyError) Error() string {
	return fmt.Sprintf("key %q does not exist", mke.Missing)
}

// ElementError is used to report on an error inside of an array.
type ElementError struct {
	Value Value // The Array value
	Index int   // The index of the element with the issue
	Inner error // The error from the element
}

// Error returns the text of the inner error.
func (ee ElementError) Error() string { return ee.Inner.Error() }

// FieldError is used to report on an invalid field inside an object.
type FieldError struct {
	Value Value  // The Object value
	Field string // The field name with the issue
	Inner error  // The error from the field
}

// Error returns the text of the inner error.
func (fe FieldError) Error() string { return fe.Inner.Error() }

// ArgError is used to report on an invalid argument to a function.
type ArgError struct {
	Function Value
	Argument Value
	Index    int
	Inner    error
}

// Error returns the text of the inner error.
func (ae ArgError) Error() string { return ae.Inner.Error() }

// WalkError walks err for all value-related errors in this package.
// WalkError returns false if err is not an error from this package.
func WalkError(err error, f func(err error)) bool {
	var foundOne bool

	nextError := err
	for nextError != nil {
		switch ne := nextError.(type) {
		case Error:
			f(nextError)
			nextError = ne.Inner
			foundOne = true
		case TypeError:
			f(nextError)
			nextError = nil
			foundOne = true
		case MissingKeyError:
			f(nextError)
			nextError = nil
			foundOne = true
		case ElementError:
			f(nextError)
			nextError = ne.Inner
			foundOne = true
		case FieldError:
			f(nextError)
			nextError = ne.Inner
			foundOne = true
		case ArgError:
			f(nextError)
			nextError = ne.Inner
			foundOne = true
		default:
			nextError = nil
		}
	}

	return foundOne
}
