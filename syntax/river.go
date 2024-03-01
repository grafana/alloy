// Package river implements a high-level API for decoding and encoding River
// configuration files. The mapping between River and Go values is described in
// the documentation for the Unmarshal and Marshal functions.
//
// Lower-level APIs which give more control over configuration evaluation are
// available in the inner packages. The implementation of this package is
// minimal and serves as a reference for how to consume the lower-level
// packages.
package river

import (
	"bytes"
	"io"

	"github.com/grafana/river/parser"
	"github.com/grafana/river/token/builder"
	"github.com/grafana/river/vm"
)

// Marshal returns the pretty-printed encoding of v as a River configuration
// file. v must be a Go struct with river struct tags which determine the
// structure of the resulting file.
//
// Marshal traverses the value v recursively, encoding each struct field as a
// River block or River attribute, based on the flags provided to the river
// struct tag.
//
// When a struct field represents a River block, Marshal creates a new block
// and recursively encodes the value as the body of the block. The name of the
// created block is taken from the name specified by the river struct tag.
//
// Struct fields which represent River blocks must be either a Go struct or a
// slice of Go structs. When the field is a Go struct, its value is encoded as
// a single block. When the field is a slice of Go structs, a block is created
// for each element in the slice.
//
// When encoding a block, if the inner Go struct has a struct field
// representing a River block label, the value of that field is used as the
// label name for the created block. Fields used for River block labels must be
// the string type. When specified, there must not be more than one struct
// field which represents a block label.
//
// The river tag specifies a name, possibly followed by a comma-separated list
// of options. The name must be empty if the provided options do not support a
// name being defined. The following provides examples for all supported struct
// field tags with their meanings:
//
//	// Field appears as a block named "example". It will always appear in the
//	// resulting encoding. When decoding, "example" is treated as a required
//	// block and must be present in the source text.
//	Field struct{...} `river:"example,block"`
//
//	// Field appears as a set of blocks named "example." It will appear in the
//	// resulting encoding if there is at least one element in the slice. When
//	// decoding, "example" is treated as a required block and at least one
//	// "example" block must be present in the source text.
//	Field []struct{...} `river:"example,block"`
//
//	// Field appears as block named "example." It will always appear in the
//	// resulting encoding. When decoding, "example" is treated as an optional
//	// block and can be omitted from the source text.
//	Field struct{...} `river:"example,block,optional"`
//
//	// Field appears as a set of blocks named "example." It will appear in the
//	// resulting encoding if there is at least one element in the slice. When
//	// decoding, "example" is treated as an optional block and can be omitted
//	// from the source text.
//	Field []struct{...} `river:"example,block,optional"`
//
//	// Field appears as an attribute named "example." It will always appear in
//	// the resulting encoding. When decoding, "example" is treated as a
//	// required attribute and must be present in the source text.
//	Field bool `river:"example,attr"`
//
//	// Field appears as an attribute named "example." If the field's value is
//	// the Go zero value, "example" is omitted from the resulting encoding.
//	// When decoding, "example" is treated as an optional attribute and can be
//	// omitted from the source text.
//	Field bool `river:"example,attr,optional"`
//
//	// The value of Field appears as the block label for the struct being
//	// converted into a block. When decoding, a block label must be provided.
//	Field string `river:",label"`
//
//	// The inner attributes and blocks of Field are exposed as top-level
//	// attributes and blocks of the outer struct.
//	Field struct{...} `river:",squash"`
//
//	// Field appears as a set of blocks starting with "example.". Only the
//	// first set element in the struct will be encoded. Each field in struct
//	// must be a block. The name of the block is prepended to the enum name.
//	// When decoding, enum blocks are treated as optional blocks and can be
//	// omitted from the source text.
//	Field []struct{...} `river:"example,enum"`
//
//	// Field is equivalent to `river:"example,enum"`.
//	Field []struct{...} `river:"example,enum,optional"`
//
// If a river tag specifies a required or optional block, the name is permitted
// to contain period `.` characters.
//
// Marshal will panic if it encounters a struct with invalid river tags.
//
// When a struct field represents a River attribute, Marshal encodes the struct
// value as a River value. The attribute name will be taken from the name
// specified by the river struct tag. See MarshalValue for the rules used to
// convert a Go value into a River value.
func Marshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MarshalValue returns the pretty-printed encoding of v as a River value.
//
// MarshalValue traverses the value v recursively. If an encountered value
// implements the encoding.TextMarshaler interface, MarshalValue calls its
// MarshalText method and encodes the result as a River string. If a value
// implements the Capsule interface, it always encodes as a River capsule
// value.
//
// Otherwise, MarshalValue uses the following type-dependent default encodings:
//
// Boolean values encode to River bools.
//
// Floating point, integer, and Number values encode to River numbers.
//
// String values encode to River strings.
//
// Array and slice values encode to River arrays, except that []byte is
// converted into a River string. Nil slices encode as an empty array and nil
// []byte slices encode as an empty string.
//
// Structs encode to River objects, using Go struct field tags to determine the
// resulting structure of the River object. Each exported struct field with a
// river tag becomes an object field, using the tag name as the field name.
// Other struct fields are ignored. If no struct field has a river tag, the
// struct encodes to a River capsule instead.
//
// Function values encode to River functions, which appear in the resulting
// text as strings formatted as "function(GO_TYPE)".
//
// All other Go values encode to River capsules, which appear in the resulting
// text as strings formatted as "capsule(GO_TYPE)".
//
// The river tag specifies the field name, possibly followed by a
// comma-separated list of options. The following provides examples for all
// supported struct field tags with their meanings:
//
//	// Field appears as an object field named "my_name". It will always
//	// appear in the resulting encoding. When decoding, "my_name" is treated
//	// as a required attribute and must be present in the source text.
//	Field bool `river:"my_name,attr"`
//
//	// Field appears as an object field named "my_name". If the field's value
//	// is the Go zero value, "example" is omitted from the resulting encoding.
//	// When decoding, "my_name" is treated as an optional attribute and can be
//	// omitted from the source text.
//	Field bool `river:"my_name,attr,optional"`
func MarshalValue(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := NewEncoder(&buf).EncodeValue(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Encoder writes River configuration to an output stream. Call NewEncoder to
// create instances of Encoder.
type Encoder struct {
	w io.Writer
}

// NewEncoder returns a new Encoder which writes configuration to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode converts the value pointed to by v into a River configuration file
// and writes the result to the Decoder's output stream.
//
// See the documentation for Marshal for details about the conversion of Go
// values into River configuration.
func (enc *Encoder) Encode(v interface{}) error {
	f := builder.NewFile()
	f.Body().AppendFrom(v)

	_, err := f.WriteTo(enc.w)
	return err
}

// EncodeValue converts the value pointed to by v into a River value and writes
// the result to the Decoder's output stream.
//
// See the documentation for MarshalValue for details about the conversion of
// Go values into River values.
func (enc *Encoder) EncodeValue(v interface{}) error {
	expr := builder.NewExpr()
	expr.SetValue(v)

	_, err := expr.WriteTo(enc.w)
	return err
}

// Unmarshal converts the River configuration file specified by in and stores
// it in the struct value pointed to by v. If v is nil or not a pointer,
// Unmarshal panics. The configuration specified by in may use expressions to
// compute values while unmarshaling. Refer to the River language documentation
// for the list of valid formatting and expression rules.
//
// Unmarshal uses the inverse of the encoding rules that Marshal uses,
// allocating maps, slices, and pointers as necessary.
//
// To unmarshal a River body into a map[string]T, Unmarshal assigns each
// attribute to a key in the map, and decodes the attribute's value as the
// value for the map entry. Only attribute statements are allowed when
// unmarshaling into a map.
//
// To unmarshal a River body into a struct, Unmarshal matches incoming
// attributes and blocks to the river struct tags specified by v. Incoming
// attribute and blocks which do not match to a river struct tag cause a
// decoding error. Additionally, any attribute or block marked as required by
// the river struct tag that are not present in the source text will generate a
// decoding error.
//
// To unmarshal a list of River blocks into a slice, Unmarshal resets the slice
// length to zero and then appends each element to the slice.
//
// To unmarshal a list of River blocks into a Go array, Unmarshal decodes each
// block into the corresponding Go array element. If the number of River blocks
// does not match the length of the Go array, a decoding error is returned.
//
// Unmarshal follows the rules specified by UnmarshalValue when unmarshaling
// the value of an attribute.
func Unmarshal(in []byte, v interface{}) error {
	dec := NewDecoder(bytes.NewReader(in))
	return dec.Decode(v)
}

// UnmarshalValue converts the River configuration file specified by in and
// stores it in the value pointed to by v. If v is nil or not a pointer,
// UnmarshalValue panics. The configuration specified by in may use expressions
// to compute values while unmarshaling. Refer to the River language
// documentation for the list of valid formatting and expression rules.
//
// Unmarshal uses the inverse of the encoding rules that MarshalValue uses,
// allocating maps, slices, and pointers as necessary, with the following
// additional rules:
//
// After converting a River value into its Go value counterpart, the Go value
// may be converted into a capsule if the capsule type implements
// ConvertibleIntoCapsule.
//
// To unmarshal a River object into a struct, UnmarshalValue matches incoming
// object fields to the river struct tags specified by v. Incoming object
// fields which do not match to a river struct tag cause a decoding error.
// Additionally, any object field marked as required by the river struct
// tag that are not present in the source text will generate a decoding error.
//
// To unmarshal River into an interface value, Unmarshal stores one of the
// following:
//
//   - bool, for River bools
//   - float64, for floating point River numbers
//     and integers which are too big to fit in either of int/int64/uint64
//   - int/int64/uint64, in this order of preference, for signed and unsigned
//     River integer numbers, depending on how big they are
//   - string, for River strings
//   - []interface{}, for River arrays
//   - map[string]interface{}, for River objects
//
// Capsule and function types will retain their original type when decoding
// into an interface value.
//
// To unmarshal a River array into a slice, Unmarshal resets the slice length
// to zero and then appends each element to the slice.
//
// To unmarshal a River array into a Go array, Unmarshal decodes River array
// elements into the corresponding Go array element. If the number of River
// elements does not match the length of the Go array, a decoding error is
// returned.
//
// To unmarshal a River object into a Map, Unmarshal establishes a map to use.
// If the map is nil, Unmarshal allocates a new map. Otherwise, Unmarshal
// reuses the existing map, keeping existing entries. Unmarshal then stores
// key-value pairs from the River object into the map. The map's key type must
// be string.
func UnmarshalValue(in []byte, v interface{}) error {
	dec := NewDecoder(bytes.NewReader(in))
	return dec.DecodeValue(v)
}

// Decoder reads River configuration from an input stream. Call NewDecoder to
// create instances of Decoder.
type Decoder struct {
	r io.Reader
}

// NewDecoder returns a new Decoder which reads configuration from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// Decode reads the River-encoded file from the Decoder's input and stores it
// in the value pointed to by v. Data will be read from the Decoder's input
// until EOF is reached.
//
// See the documentation for Unmarshal for details about the conversion of River
// configuration into Go values.
func (dec *Decoder) Decode(v interface{}) error {
	bb, err := io.ReadAll(dec.r)
	if err != nil {
		return err
	}

	f, err := parser.ParseFile("", bb)
	if err != nil {
		return err
	}

	eval := vm.New(f)
	return eval.Evaluate(nil, v)
}

// DecodeValue reads the River-encoded expression from the Decoder's input and
// stores it in the value pointed to by v. Data will be read from the Decoder's
// input until EOF is reached.
//
// See the documentation for UnmarshalValue for details about the conversion of
// River values into Go values.
func (dec *Decoder) DecodeValue(v interface{}) error {
	bb, err := io.ReadAll(dec.r)
	if err != nil {
		return err
	}

	f, err := parser.ParseExpression(string(bb))
	if err != nil {
		return err
	}

	eval := vm.New(f)
	return eval.Evaluate(nil, v)
}
