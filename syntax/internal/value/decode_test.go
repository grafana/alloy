package value_test

import (
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"
	"unsafe"

	"github.com/grafana/river/internal/value"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecode_Numbers(t *testing.T) {
	// There's a lot of values that can represent numbers, so we construct a
	// matrix dynamically of all the combinations here.
	vals := []interface{}{
		int(15), int8(15), int16(15), int32(15), int64(15),
		uint(15), uint8(15), uint16(15), uint32(15), uint64(15),
		float32(15), float64(15),
		string("15"), // string holding a valid number (which can be converted to a number)
	}

	for _, input := range vals {
		for _, expect := range vals {
			val := value.Encode(input)

			name := fmt.Sprintf(
				"%s to %s",
				reflect.TypeOf(input),
				reflect.TypeOf(expect),
			)

			t.Run(name, func(t *testing.T) {
				vPtr := reflect.New(reflect.TypeOf(expect)).Interface()
				require.NoError(t, value.Decode(val, vPtr))

				actual := reflect.ValueOf(vPtr).Elem().Interface()
				require.Equal(t, expect, actual)
			})
		}
	}
}

func TestDecode(t *testing.T) {
	// Declare some types to use for testing. Person2 is used as a struct
	// equivalent to Person, but with a different Go type to force casting.
	type Person struct {
		Name string `river:"name,attr"`
	}

	type Person2 struct {
		Name string `river:"name,attr"`
	}

	tt := []struct {
		input, expect interface{}
	}{
		{nil, (*int)(nil)},

		// Non-number primitives.
		{string("Hello!"), string("Hello!")},
		{bool(true), bool(true)},

		// Arrays
		{[]int{1, 2, 3}, []int{1, 2, 3}},
		{[]int{1, 2, 3}, [...]int{1, 2, 3}},
		{[...]int{1, 2, 3}, []int{1, 2, 3}},
		{[...]int{1, 2, 3}, [...]int{1, 2, 3}},

		// Maps
		{map[string]int{"year": 2022}, map[string]uint{"year": 2022}},
		{map[string]string{"name": "John"}, map[string]string{"name": "John"}},
		{map[string]string{"name": "John"}, Person{Name: "John"}},
		{Person{Name: "John"}, map[string]string{"name": "John"}},
		{Person{Name: "John"}, Person{Name: "John"}},
		{Person{Name: "John"}, Person2{Name: "John"}},
		{Person2{Name: "John"}, Person{Name: "John"}},

		// NOTE(rfratto): we don't test capsules or functions here because they're
		// not comparable in the same way as we do the other tests.
		//
		// See TestDecode_Functions and TestDecode_Capsules for specific decoding
		// tests of those types.
	}

	for _, tc := range tt {
		val := value.Encode(tc.input)

		name := fmt.Sprintf(
			"%s (%s) to %s",
			val.Type(),
			reflect.TypeOf(tc.input),
			reflect.TypeOf(tc.expect),
		)

		t.Run(name, func(t *testing.T) {
			vPtr := reflect.New(reflect.TypeOf(tc.expect)).Interface()
			require.NoError(t, value.Decode(val, vPtr))

			actual := reflect.ValueOf(vPtr).Elem().Interface()

			require.Equal(t, tc.expect, actual)
		})
	}
}

// TestDecode_PreservePointer ensures that pointer addresses can be preserved
// when decoding.
func TestDecode_PreservePointer(t *testing.T) {
	num := 5
	val := value.Encode(&num)

	var nump *int
	require.NoError(t, value.Decode(val, &nump))
	require.Equal(t, unsafe.Pointer(nump), unsafe.Pointer(&num))
}

// TestDecode_PreserveMapReference ensures that map references can be preserved
// when decoding.
func TestDecode_PreserveMapReference(t *testing.T) {
	m := make(map[string]string)
	val := value.Encode(m)

	var actual map[string]string
	require.NoError(t, value.Decode(val, &actual))

	// We can't check to see if the pointers of m and actual match, but we can
	// modify m to see if actual is also modified.
	m["foo"] = "bar"
	require.Equal(t, "bar", actual["foo"])
}

// TestDecode_PreserveSliceReference ensures that slice references can be
// preserved when decoding.
func TestDecode_PreserveSliceReference(t *testing.T) {
	s := make([]string, 3)
	val := value.Encode(s)

	var actual []string
	require.NoError(t, value.Decode(val, &actual))

	// We can't check to see if the pointers of m and actual match, but we can
	// modify s to see if actual is also modified.
	s[0] = "Hello, world!"
	require.Equal(t, "Hello, world!", actual[0])
}
func TestDecode_Functions(t *testing.T) {
	val := value.Encode(func() int { return 15 })

	var f func() int
	require.NoError(t, value.Decode(val, &f))
	require.Equal(t, 15, f())
}

func TestDecode_Capsules(t *testing.T) {
	expect := make(chan int, 5)

	var actual chan int
	require.NoError(t, value.Decode(value.Encode(expect), &actual))
	require.Equal(t, expect, actual)
}

type ValueInterface interface{ SomeMethod() }

type Value1 struct{ test string }

func (c Value1) SomeMethod() {}

// TestDecode_CapsuleInterface tests that we are able to decode when
// the target `into` is an interface.
func TestDecode_CapsuleInterface(t *testing.T) {
	tt := []struct {
		name     string
		value    ValueInterface
		expected ValueInterface
	}{
		{
			name:     "Capsule to Capsule",
			value:    Value1{test: "true"},
			expected: Value1{test: "true"},
		},
		{
			name:     "Capsule Pointer to Capsule",
			value:    &Value1{test: "true"},
			expected: &Value1{test: "true"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var actual ValueInterface
			require.NoError(t, value.Decode(value.Encode(tc.value), &actual))

			// require.Same validates the memory address matches after Decode.
			if reflect.TypeOf(tc.value).Kind() == reflect.Pointer {
				require.Same(t, tc.value, actual)
			}

			// We use tc.expected to validate the properties of actual match the
			// original tc.value properties (nothing has mutated them during the test).
			require.Equal(t, tc.expected, actual)
		})
	}
}

// TestDecode_CapsulesError tests that we are unable to decode when
// the target `into` is not an interface.
func TestDecode_CapsulesError(t *testing.T) {
	type Capsule1 struct{ test string }
	type Capsule2 Capsule1

	v := Capsule1{test: "true"}
	actual := Capsule2{}

	require.EqualError(t, value.Decode(value.Encode(v), &actual), `expected capsule("value_test.Capsule2"), got capsule("value_test.Capsule1")`)
}

// TestDecodeCopy_SliceCopy ensures that copies are made during decoding
// instead of setting values directly.
func TestDecodeCopy_SliceCopy(t *testing.T) {
	orig := []int{1, 2, 3}

	var res []int
	require.NoError(t, value.DecodeCopy(value.Encode(orig), &res))

	res[0] = 10
	require.Equal(t, []int{1, 2, 3}, orig, "Original slice should not have been modified")
}

// TestDecodeCopy_ArrayCopy ensures that copies are made during decoding
// instead of setting values directly.
func TestDecode_ArrayCopy(t *testing.T) {
	orig := [...]int{1, 2, 3}

	var res [3]int
	require.NoError(t, value.DecodeCopy(value.Encode(orig), &res))

	res[0] = 10
	require.Equal(t, [3]int{1, 2, 3}, orig, "Original array should not have been modified")
}

func TestDecode_CustomTypes(t *testing.T) {
	t.Run("object to Unmarshaler", func(t *testing.T) {
		var actual customUnmarshaler
		require.NoError(t, value.Decode(value.Object(nil), &actual))
		require.True(t, actual.UnmarshalCalled, "UnmarshalRiver was not invoked")
		require.True(t, actual.DefaultCalled, "SetToDefault was not invoked")
		require.True(t, actual.ValidateCalled, "Validate was not invoked")
	})

	t.Run("TextMarshaler to TextUnmarshaler", func(t *testing.T) {
		now := time.Now()

		var actual time.Time
		require.NoError(t, value.Decode(value.Encode(now), &actual))
		require.True(t, now.Equal(actual))
	})

	t.Run("time.Duration to time.Duration", func(t *testing.T) {
		dur := 15 * time.Second

		var actual time.Duration
		require.NoError(t, value.Decode(value.Encode(dur), &actual))
		require.Equal(t, dur, actual)
	})

	t.Run("string to TextUnmarshaler", func(t *testing.T) {
		now := time.Now()
		nowBytes, _ := now.MarshalText()

		var actual time.Time
		require.NoError(t, value.Decode(value.String(string(nowBytes)), &actual))

		actualBytes, _ := actual.MarshalText()
		require.Equal(t, nowBytes, actualBytes)
	})

	t.Run("string to time.Duration", func(t *testing.T) {
		dur := 15 * time.Second

		var actual time.Duration
		require.NoError(t, value.Decode(value.String(dur.String()), &actual))
		require.Equal(t, dur.String(), actual.String())
	})
}

type customUnmarshaler struct {
	UnmarshalCalled bool `river:"unmarshal_called,attr,optional"`
	DefaultCalled   bool `river:"default_called,attr,optional"`
	ValidateCalled  bool `river:"validate_called,attr,optional"`
}

func (cu *customUnmarshaler) UnmarshalRiver(f func(interface{}) error) error {
	cu.UnmarshalCalled = true
	return f((*customUnmarshalerTarget)(cu))
}

type customUnmarshalerTarget customUnmarshaler

func (s *customUnmarshalerTarget) SetToDefault() {
	s.DefaultCalled = true
}

func (s *customUnmarshalerTarget) Validate() error {
	s.ValidateCalled = true
	return nil
}

type textEnumType bool

func (et *textEnumType) UnmarshalText(text []byte) error {
	*et = false

	switch string(text) {
	case "accepted_value":
		*et = true
		return nil
	default:
		return fmt.Errorf("unrecognized value %q", string(text))
	}
}

func TestDecode_TextUnmarshaler(t *testing.T) {
	t.Run("valid type and value", func(t *testing.T) {
		var et textEnumType
		require.NoError(t, value.Decode(value.String("accepted_value"), &et))
		require.Equal(t, textEnumType(true), et)
	})

	t.Run("invalid type", func(t *testing.T) {
		var et textEnumType
		err := value.Decode(value.Bool(true), &et)
		require.EqualError(t, err, "expected string, got bool")
	})

	t.Run("invalid value", func(t *testing.T) {
		var et textEnumType
		err := value.Decode(value.String("bad_value"), &et)
		require.EqualError(t, err, `unrecognized value "bad_value"`)
	})

	t.Run("unmarshaler nested in other value", func(t *testing.T) {
		input := value.Array(
			value.String("accepted_value"),
			value.String("accepted_value"),
			value.String("accepted_value"),
		)

		var ett []textEnumType
		require.NoError(t, value.Decode(input, &ett))
		require.Equal(t, []textEnumType{true, true, true}, ett)
	})
}

func TestDecode_ErrorChain(t *testing.T) {
	type Target struct {
		Key struct {
			Object struct {
				Field1 []int `river:"field1,attr"`
			} `river:"object,attr"`
		} `river:"key,attr"`
	}

	val := value.Object(map[string]value.Value{
		"key": value.Object(map[string]value.Value{
			"object": value.Object(map[string]value.Value{
				"field1": value.Array(
					value.Int(15),
					value.Int(30),
					value.String("Hello, world!"),
				),
			}),
		}),
	})

	// NOTE(rfratto): strings of errors from the value package are fairly limited
	// in the amount of information they show, since the value package doesn't
	// have a great way to pretty-print the chain of errors.
	//
	// For example, with the error below, the message doesn't explain where the
	// string is coming from, even though the error values hold that context.
	//
	// Callers consuming errors should print the error chain with extra context
	// so it's more useful to users.
	err := value.Decode(val, &Target{})
	expectErr := `expected number, got string`
	require.EqualError(t, err, expectErr)
}

type boolish int

var _ value.ConvertibleFromCapsule = (*boolish)(nil)
var _ value.ConvertibleIntoCapsule = (boolish)(0)

func (b boolish) RiverCapsule() {}

func (b *boolish) ConvertFrom(src interface{}) error {
	switch v := src.(type) {
	case bool:
		if v {
			*b = 1
		} else {
			*b = 0
		}
		return nil
	}

	return value.ErrNoConversion
}

func (b boolish) ConvertInto(dst interface{}) error {
	switch d := dst.(type) {
	case *bool:
		if b == 0 {
			*d = false
		} else {
			*d = true
		}
		return nil
	}

	return value.ErrNoConversion
}

func TestDecode_CustomConvert(t *testing.T) {
	t.Run("compatible type to custom", func(t *testing.T) {
		var b boolish
		err := value.Decode(value.Bool(true), &b)
		require.NoError(t, err)
		require.Equal(t, boolish(1), b)
	})

	t.Run("custom to compatible type", func(t *testing.T) {
		var b bool
		err := value.Decode(value.Encapsulate(boolish(10)), &b)
		require.NoError(t, err)
		require.Equal(t, true, b)
	})

	t.Run("incompatible type to custom", func(t *testing.T) {
		var b boolish
		err := value.Decode(value.String("true"), &b)
		require.EqualError(t, err, "expected capsule, got string")
	})

	t.Run("custom to incompatible type", func(t *testing.T) {
		src := boolish(10)

		var s string
		err := value.Decode(value.Encapsulate(&src), &s)
		require.EqualError(t, err, "expected string, got capsule")
	})
}

func TestDecode_SquashedFields(t *testing.T) {
	type InnerStruct struct {
		InnerField1 string `river:"inner_field_1,attr,optional"`
		InnerField2 string `river:"inner_field_2,attr,optional"`
	}

	type OuterStruct struct {
		OuterField1 string      `river:"outer_field_1,attr,optional"`
		Inner       InnerStruct `river:",squash"`
		OuterField2 string      `river:"outer_field_2,attr,optional"`
	}

	var (
		in = map[string]string{
			"outer_field_1": "value1",
			"outer_field_2": "value2",
			"inner_field_1": "value3",
			"inner_field_2": "value4",
		}
		expect = OuterStruct{
			OuterField1: "value1",
			Inner: InnerStruct{
				InnerField1: "value3",
				InnerField2: "value4",
			},
			OuterField2: "value2",
		}
	)

	var out OuterStruct
	err := value.Decode(value.Encode(in), &out)
	require.NoError(t, err)
	require.Equal(t, expect, out)
}

func TestDecode_SquashedFields_Pointer(t *testing.T) {
	type InnerStruct struct {
		InnerField1 string `river:"inner_field_1,attr,optional"`
		InnerField2 string `river:"inner_field_2,attr,optional"`
	}

	type OuterStruct struct {
		OuterField1 string       `river:"outer_field_1,attr,optional"`
		Inner       *InnerStruct `river:",squash"`
		OuterField2 string       `river:"outer_field_2,attr,optional"`
	}

	var (
		in = map[string]string{
			"outer_field_1": "value1",
			"outer_field_2": "value2",
			"inner_field_1": "value3",
			"inner_field_2": "value4",
		}
		expect = OuterStruct{
			OuterField1: "value1",
			Inner: &InnerStruct{
				InnerField1: "value3",
				InnerField2: "value4",
			},
			OuterField2: "value2",
		}
	)

	var out OuterStruct
	err := value.Decode(value.Encode(in), &out)
	require.NoError(t, err)
	require.Equal(t, expect, out)
}

func TestDecode_Slice(t *testing.T) {
	type Block struct {
		Attr int `river:"attr,attr"`
	}

	type Struct struct {
		Blocks []Block `river:"block.a,block,optional"`
	}

	var (
		in = map[string]interface{}{
			"block": map[string]interface{}{
				"a": []map[string]interface{}{
					{"attr": 1},
					{"attr": 2},
					{"attr": 3},
					{"attr": 4},
				},
			},
		}
		expect = Struct{
			Blocks: []Block{
				{Attr: 1},
				{Attr: 2},
				{Attr: 3},
				{Attr: 4},
			},
		}
	)

	var out Struct
	err := value.Decode(value.Encode(in), &out)
	require.NoError(t, err)
	require.Equal(t, expect, out)
}

func TestDecode_SquashedSlice(t *testing.T) {
	type Block struct {
		Attr int `river:"attr,attr"`
	}

	type InnerStruct struct {
		BlockA Block `river:"a,block,optional"`
		BlockB Block `river:"b,block,optional"`
		BlockC Block `river:"c,block,optional"`
	}

	type OuterStruct struct {
		OuterField1 string        `river:"outer_field_1,attr,optional"`
		Inner       []InnerStruct `river:"block,enum"`
		OuterField2 string        `river:"outer_field_2,attr,optional"`
	}

	var (
		in = map[string]interface{}{
			"outer_field_1": "value1",
			"outer_field_2": "value2",

			"block": []map[string]interface{}{
				{"a": map[string]interface{}{"attr": 1}},
				{"b": map[string]interface{}{"attr": 2}},
				{"c": map[string]interface{}{"attr": 3}},
				{"a": map[string]interface{}{"attr": 4}},
			},
		}
		expect = OuterStruct{
			OuterField1: "value1",
			OuterField2: "value2",

			Inner: []InnerStruct{
				{BlockA: Block{Attr: 1}},
				{BlockB: Block{Attr: 2}},
				{BlockC: Block{Attr: 3}},
				{BlockA: Block{Attr: 4}},
			},
		}
	)

	var out OuterStruct
	err := value.Decode(value.Encode(in), &out)
	require.NoError(t, err)
	require.Equal(t, expect, out)
}

func TestDecode_SquashedSlice_Pointer(t *testing.T) {
	type Block struct {
		Attr int `river:"attr,attr"`
	}

	type InnerStruct struct {
		BlockA *Block `river:"a,block,optional"`
		BlockB *Block `river:"b,block,optional"`
		BlockC *Block `river:"c,block,optional"`
	}

	type OuterStruct struct {
		OuterField1 string        `river:"outer_field_1,attr,optional"`
		Inner       []InnerStruct `river:"block,enum"`
		OuterField2 string        `river:"outer_field_2,attr,optional"`
	}

	var (
		in = map[string]interface{}{
			"outer_field_1": "value1",
			"outer_field_2": "value2",

			"block": []map[string]interface{}{
				{"a": map[string]interface{}{"attr": 1}},
				{"b": map[string]interface{}{"attr": 2}},
				{"c": map[string]interface{}{"attr": 3}},
				{"a": map[string]interface{}{"attr": 4}},
			},
		}
		expect = OuterStruct{
			OuterField1: "value1",
			OuterField2: "value2",

			Inner: []InnerStruct{
				{BlockA: &Block{Attr: 1}},
				{BlockB: &Block{Attr: 2}},
				{BlockC: &Block{Attr: 3}},
				{BlockA: &Block{Attr: 4}},
			},
		}
	)

	var out OuterStruct
	err := value.Decode(value.Encode(in), &out)
	require.NoError(t, err)
	require.Equal(t, expect, out)
}

// TestDecode_KnownTypes_Any asserts that decoding River values into an
// any/interface{} results in known types.
func TestDecode_KnownTypes_Any(t *testing.T) {
	tt := []struct {
		input  any
		expect any
	}{
		// expect "int"
		{int(0), 0},
		{int(-1), -1},
		{int(15), 15},
		{int8(15), 15},
		{int16(15), 15},
		{int32(15), 15},
		{int64(15), 15},
		{uint(0), 0},
		{uint(15), 15},
		{uint8(15), 15},
		{uint16(15), 15},
		{uint32(15), 15},
		{uint64(15), 15},
		{int64(math.MinInt64), math.MinInt64},
		{int64(math.MaxInt64), math.MaxInt64},
		// expect "uint"
		{uint64(math.MaxInt64 + 1), uint64(math.MaxInt64 + 1)},
		{uint64(math.MaxUint64), uint64(math.MaxUint64)},
		// expect "float"
		{float32(2.5), float64(2.5)},
		{float64(2.5), float64(2.5)},
		{float64(math.MinInt64) - 10, float64(math.MinInt64) - 10},
		{float64(math.MaxInt64) + 10, float64(math.MaxInt64) + 10},

		{bool(true), bool(true)},
		{string("Hello"), string("Hello")},

		{
			input:  []int{1, 2, 3},
			expect: []any{1, 2, 3},
		},

		{
			input:  map[string]int{"number": 15},
			expect: map[string]any{"number": 15},
		},
		{
			input: struct {
				Name string `river:"name,attr"`
			}{Name: "John"},

			expect: map[string]any{"name": "John"},
		},
	}

	t.Run("basic types", func(t *testing.T) {
		for _, tc := range tt {
			var actual any
			err := value.Decode(value.Encode(tc.input), &actual)

			if assert.NoError(t, err) {
				assert.Equal(t, tc.expect, actual,
					"Expected %[1]v (%[1]T) to transcode to %[2]v (%[2]T)", tc.input, tc.expect)
			}
		}
	})

	t.Run("inside maps", func(t *testing.T) {
		for _, tc := range tt {
			input := map[string]any{
				"key": tc.input,
			}

			var actual map[string]any
			err := value.Decode(value.Encode(input), &actual)

			if assert.NoError(t, err) {
				assert.Equal(t, tc.expect, actual["key"],
					"Expected %[1]v (%[1]T) to transcode to %[2]v (%[2]T) inside a map", tc.input, tc.expect)
			}
		}
	})
}

func TestRetainCapsulePointer(t *testing.T) {
	capsuleVal := &capsule{}

	in := map[string]any{
		"foo": capsuleVal,
	}

	var actual map[string]any
	err := value.Decode(value.Encode(in), &actual)
	require.NoError(t, err)

	expect := map[string]any{
		"foo": capsuleVal,
	}
	require.Equal(t, expect, actual)
}

type capsule struct{}

func (*capsule) RiverCapsule() {}
