package value

import (
	"math"
	"reflect"
	"strconv"
)

var (
	nativeIntBits  = reflect.TypeOf(int(0)).Bits()
	nativeUintBits = reflect.TypeOf(uint(0)).Bits()
)

// NumberKind categorizes a type of Go number.
type NumberKind uint8

const (
	// NumberKindInt represents an int-like type (e.g., int, int8, etc.).
	NumberKindInt NumberKind = iota
	// NumberKindUint represents a uint-like type (e.g., uint, uint8, etc.).
	NumberKindUint
	// NumberKindFloat represents both float32 and float64.
	NumberKindFloat
)

// makeNumberKind converts a Go kind to a River kind.
func makeNumberKind(k reflect.Kind) NumberKind {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return NumberKindInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return NumberKindUint
	case reflect.Float32, reflect.Float64:
		return NumberKindFloat
	default:
		panic("river/value: makeNumberKind called with unsupported Kind value")
	}
}

// Number is a generic representation of Go numbers. It is intended to be
// created on the fly for numerical operations when the real number type is not
// known.
type Number struct {
	// Value holds the raw data for the number. Note that for numberKindFloat,
	// value is the raw bits of the float64 and must be converted back to a
	// float64 before it can be used.
	value uint64

	bits uint8      // 8, 16, 32, 64, used for overflow checking
	k    NumberKind // int, uint, float
}

func newNumberValue(v reflect.Value) Number {
	var (
		val  uint64
		bits int
		nk   NumberKind
	)

	switch v.Kind() {
	case reflect.Int:
		val, bits, nk = uint64(v.Int()), nativeIntBits, NumberKindInt
	case reflect.Int8:
		val, bits, nk = uint64(v.Int()), 8, NumberKindInt
	case reflect.Int16:
		val, bits, nk = uint64(v.Int()), 16, NumberKindInt
	case reflect.Int32:
		val, bits, nk = uint64(v.Int()), 32, NumberKindInt
	case reflect.Int64:
		val, bits, nk = uint64(v.Int()), 64, NumberKindInt
	case reflect.Uint:
		val, bits, nk = v.Uint(), nativeUintBits, NumberKindUint
	case reflect.Uint8:
		val, bits, nk = v.Uint(), 8, NumberKindUint
	case reflect.Uint16:
		val, bits, nk = v.Uint(), 16, NumberKindUint
	case reflect.Uint32:
		val, bits, nk = v.Uint(), 32, NumberKindUint
	case reflect.Uint64:
		val, bits, nk = v.Uint(), 64, NumberKindUint
	case reflect.Float32:
		val, bits, nk = math.Float64bits(v.Float()), 32, NumberKindFloat
	case reflect.Float64:
		val, bits, nk = math.Float64bits(v.Float()), 64, NumberKindFloat
	default:
		panic("river/value: unrecognized Go number type " + v.Kind().String())
	}

	return Number{val, uint8(bits), nk}
}

// Kind returns the Number's NumberKind.
func (nv Number) Kind() NumberKind { return nv.k }

// Int converts the Number into an int64.
func (nv Number) Int() int64 {
	if nv.k == NumberKindFloat {
		return int64(math.Float64frombits(nv.value))
	}
	return int64(nv.value)
}

// Uint converts the Number into a uint64.
func (nv Number) Uint() uint64 {
	if nv.k == NumberKindFloat {
		return uint64(math.Float64frombits(nv.value))
	}
	return nv.value
}

// Float converts the Number into a float64.
func (nv Number) Float() float64 {
	switch nv.k {
	case NumberKindInt:
		// Convert nv.value to an int64 before converting to a float64 so the sign
		// flag gets handled correctly.
		return float64(int64(nv.value))
	case NumberKindFloat:
		return math.Float64frombits(nv.value)
	}
	return float64(nv.value)
}

// ToString converts the Number to a string.
func (nv Number) ToString() string {
	switch nv.k {
	case NumberKindUint:
		return strconv.FormatUint(nv.value, 10)
	case NumberKindInt:
		return strconv.FormatInt(int64(nv.value), 10)
	case NumberKindFloat:
		return strconv.FormatFloat(math.Float64frombits(nv.value), 'f', -1, 64)
	}
	panic("river/value: unreachable")
}
