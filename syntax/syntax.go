// Package syntax implements a high-level API for decoding and encoding Alloy
// configuration files. The mapping between Alloy and Go values is described in
// the documentation for the Unmarshal and Marshal functions.
//
// Lower-level APIs which give more control over configuration evaluation are
// available in the inner packages. The implementation of this package is
// minimal and serves as a reference for how to consume the lower-level
// packages.
package syntax

import (
	"bytes"
	"io"

	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/token/builder"
	"github.com/grafana/alloy/syntax/vm"
)

// Marshal returns the pretty-printed encoding of v as a Alloy configuration
// file. v must be a Go struct with alloy struct tags which determine the
// structure of the resulting file.
//
// Marshal traverses the value v recursively, encoding each struct field as a
// Alloy block or attribute, based on the flags provided to the alloy struct
// tag.
//
// When a struct field represents an Alloy block, Marshal creates a new block
// and recursively encodes the value as the body of the block. The name of the
// created block is taken from the name specified by the alloy struct tag.
//
// Struct fields which represent Alloy blocks must be either a Go struct or a
// slice of Go structs. When the field is a Go struct, its value is encoded as
// a single block. When the field is a slice of Go structs, a block is created
// for each element in the slice.
//
// When encoding a block, if the inner Go struct has a struct field
// representing a Alloy block label, the value of that field is used as the
// label name for the created block. Fields used for Alloy block labels must be
// the string type. When specified, there must not be more than one struct
// field which represents a block label.
//
// The alloy tag specifies a name, possibly followed by a comma-separated list
// of options. The name must be empty if the provided options do not support a
// name being defined. The following provides examples for all supported struct
// field tags with their meanings:
//
//	// Field appears as a block named "example". It will always appear in the
//	// resulting encoding. When decoding, "example" is treated as a required
//	// block and must be present in the source text.
//	Field struct{...} `alloy:"example,block"`
//
//	// Field appears as a set of blocks named "example." It will appear in the
//	// resulting encoding if there is at least one element in the slice. When
//	// decoding, "example" is treated as a required block and at least one
//	// "example" block must be present in the source text.
//	Field []struct{...} `alloy:"example,block"`
//
//	// Field appears as block named "example." It will always appear in the
//	// resulting encoding. When decoding, "example" is treated as an optional
//	// block and can be omitted from the source text.
//	Field struct{...} `alloy:"example,block,optional"`
//
//	// Field appears as a set of blocks named "example." It will appear in the
//	// resulting encoding if there is at least one element in the slice. When
//	// decoding, "example" is treated as an optional block and can be omitted
//	// from the source text.
//	Field []struct{...} `alloy:"example,block,optional"`
//
//	// Field appears as an attribute named "example." It will always appear in
//	// the resulting encoding. When decoding, "example" is treated as a
//	// required attribute and must be present in the source text.
//	Field bool `alloy:"example,attr"`
//
//	// Field appears as an attribute named "example." If the field's value is
//	// the Go zero value, "example" is omitted from the resulting encoding.
//	// When decoding, "example" is treated as an optional attribute and can be
//	// omitted from the source text.
//	Field bool `alloy:"example,attr,optional"`
//
//	// The value of Field appears as the block label for the struct being
//	// converted into a block. When decoding, a block label must be provided.
//	Field string `alloy:",label"`
//
//	// The inner attributes and blocks of Field are exposed as top-level
//	// attributes and blocks of the outer struct.
//	Field struct{...} `alloy:",squash"`
//
//	// Field appears as a set of blocks starting with "example.". Only the
//	// first set element in the struct will be encoded. Each field in struct
//	// must be a block. The name of the block is prepended to the enum name.
//	// When decoding, enum blocks are treated as optional blocks and can be
//	// omitted from the source text.
//	Field []struct{...} `alloy:"example,enum"`
//
//	// Field is equivalent to `alloy:"example,enum"`.
//	Field []struct{...} `alloy:"example,enum,optional"`
//
// If an alloy tag specifies a required or optional block, the name is permitted
// to contain period `.` characters.
//
// Marshal will panic if it encounters a struct with invalid alloy tags.
//
// When a struct field represents an Alloy attribute, Marshal encodes the
// struct value as an Alloy value. The attribute name will be taken from the
// name specified by the alloy struct tag. See MarshalValue for the rules used
// to convert a Go value into an Alloy value.
func Marshal(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// MarshalValue returns the pretty-printed encoding of v as an Alloy value.
//
// MarshalValue traverses the value v recursively. If an encountered value
// implements the encoding.TextMarshaler interface, MarshalValue calls its
// MarshalText method and encodes the result as an Alloy string. If a value
// implements the Capsule interface, it always encodes as an Alloy capsule
// value.
//
// Otherwise, MarshalValue uses the following type-dependent default encodings:
//
// Boolean values encode to Alloy bools.
//
// Floating point, integer, and Number values encode to Alloy numbers.
//
// String values encode to Alloy strings.
//
// Array and slice values encode to Alloy arrays, except that []byte is
// converted into a Alloy string. Nil slices encode as an empty array and nil
// []byte slices encode as an empty string.
//
// Structs encode to Alloy objects, using Go struct field tags to determine the
// resulting structure of the Alloy object. Each exported struct field with an
// alloy tag becomes an object field, using the tag name as the field name.
// Other struct fields are ignored. If no struct field has an alloy tag, the
// struct encodes to an Alloy capsule instead.
//
// Function values encode to Alloy functions, which appear in the resulting
// text as strings formatted as "function(GO_TYPE)".
//
// All other Go values encode to Alloy capsules, which appear in the resulting
// text as strings formatted as "capsule(GO_TYPE)".
//
// The alloy tag specifies the field name, possibly followed by a
// comma-separated list of options. The following provides examples for all
// supported struct field tags with their meanings:
//
//	// Field appears as an object field named "my_name". It will always
//	// appear in the resulting encoding. When decoding, "my_name" is treated
//	// as a required attribute and must be present in the source text.
//	Field bool `alloy:"my_name,attr"`
//
//	// Field appears as an object field named "my_name". If the field's value
//	// is the Go zero value, "example" is omitted from the resulting encoding.
//	// When decoding, "my_name" is treated as an optional attribute and can be
//	// omitted from the source text.
//	Field bool `alloy:"my_name,attr,optional"`
func MarshalValue(v any) ([]byte, error) {
	var buf bytes.Buffer
	if err := NewEncoder(&buf).EncodeValue(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Encoder writes Alloy configuration to an output stream. Call NewEncoder to
// create instances of Encoder.
type Encoder struct {
	w io.Writer
}

// NewEncoder returns a new Encoder which writes configuration to w.
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

// Encode converts the value pointed to by v into an Alloy configuration file
// and writes the result to the Decoder's output stream.
//
// See the documentation for Marshal for details about the conversion of Go
// values into Alloy configuration.
func (enc *Encoder) Encode(v any) error {
	f := builder.NewFile()
	f.Body().AppendFrom(v)

	_, err := f.WriteTo(enc.w)
	return err
}

// EncodeValue converts the value pointed to by v into an Alloy value and
// writes the result to the Decoder's output stream.
//
// See the documentation for MarshalValue for details about the conversion of
// Go values into Alloy values.
func (enc *Encoder) EncodeValue(v any) error {
	expr := builder.NewExpr()
	expr.SetValue(v)

	_, err := expr.WriteTo(enc.w)
	return err
}

// Unmarshal converts the Alloy configuration file specified by in and stores
// it in the struct value pointed to by v. If v is nil or not a pointer,
// Unmarshal panics. The configuration specified by in may use expressions to
// compute values while unmarshaling. Refer to the Alloy syntax documentation
// for the list of valid formatting and expression rules.
//
// Unmarshal uses the inverse of the encoding rules that Marshal uses,
// allocating maps, slices, and pointers as necessary.
//
// To unmarshal an Alloy body into a map[string]T, Unmarshal assigns each
// attribute to a key in the map, and decodes the attribute's value as the
// value for the map entry. Only attribute statements are allowed when
// unmarshaling into a map.
//
// To unmarshal an Alloy body into a struct, Unmarshal matches incoming
// attributes and blocks to the alloy struct tags specified by v. Incoming
// attribute and blocks which do not match to an alloy struct tag cause a
// decoding error. Additionally, any attribute or block marked as required by
// the alloy struct tag that are not present in the source text will generate a
// decoding error.
//
// To unmarshal a list of Alloy blocks into a slice, Unmarshal resets the slice
// length to zero and then appends each element to the slice.
//
// To unmarshal a list of Alloy blocks into a Go array, Unmarshal decodes each
// block into the corresponding Go array element. If the number of Alloy blocks
// does not match the length of the Go array, a decoding error is returned.
//
// Unmarshal follows the rules specified by UnmarshalValue when unmarshaling
// the value of an attribute.
func Unmarshal(in []byte, v any) error {
	dec := NewDecoder(bytes.NewReader(in))
	return dec.Decode(v)
}

// UnmarshalValue converts the Alloy configuration file specified by in and
// stores it in the value pointed to by v. If v is nil or not a pointer,
// UnmarshalValue panics. The configuration specified by in may use expressions
// to compute values while unmarshaling. Refer to the Alloy syntax
// documentation for the list of valid formatting and expression rules.
//
// Unmarshal uses the inverse of the encoding rules that MarshalValue uses,
// allocating maps, slices, and pointers as necessary, with the following
// additional rules:
//
// After converting an Alloy value into its Go value counterpart, the Go value
// may be converted into a capsule if the capsule type implements
// ConvertibleIntoCapsule.
//
// To unmarshal an Alloy object into a struct, UnmarshalValue matches incoming
// object fields to the alloy struct tags specified by v. Incoming object
// fields which do not match to an alloy struct tag cause a decoding error.
// Additionally, any object field marked as required by the alloy struct
// tag that are not present in the source text will generate a decoding error.
//
// To unmarshal Alloy into an interface value, Unmarshal stores one of the
// following:
//
//   - bool, for Alloy bools
//   - float64, for floating point Alloy numbers
//     and integers which are too big to fit in either of int/int64/uint64
//   - int/int64/uint64, in this order of preference, for signed and unsigned
//     Alloy integer numbers, depending on how big they are
//   - string, for Alloy strings
//   - []interface{}, for Alloy arrays
//   - map[string]interface{}, for Alloy objects
//
// Capsule and function types will retain their original type when decoding
// into an interface value.
//
// To unmarshal an Alloy array into a slice, Unmarshal resets the slice length
// to zero and then appends each element to the slice.
//
// To unmarshal an Alloy array into a Go array, Unmarshal decodes Alloy array
// elements into the corresponding Go array element. If the number of elements
// does not match the length of the Go array, a decoding error is returned.
//
// To unmarshal an Alloy object into a Map, Unmarshal establishes a map to use.
// If the map is nil, Unmarshal allocates a new map. Otherwise, Unmarshal
// reuses the existing map, keeping existing entries. Unmarshal then stores
// key-value pairs from the Alloy object into the map. The map's key type must
// be string.
func UnmarshalValue(in []byte, v any) error {
	dec := NewDecoder(bytes.NewReader(in))
	return dec.DecodeValue(v)
}

// Decoder reads Alloy configuration from an input stream. Call NewDecoder to
// create instances of Decoder.
type Decoder struct {
	r io.Reader
}

// NewDecoder returns a new Decoder which reads configuration from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: r}
}

// Decode reads the Alloy-encoded file from the Decoder's input and stores it
// in the value pointed to by v. Data will be read from the Decoder's input
// until EOF is reached.
//
// See the documentation for Unmarshal for details about the conversion of
// Alloy configuration into Go values.
func (dec *Decoder) Decode(v any) error {
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

// DecodeValue reads the Alloy-encoded expression from the Decoder's input and
// stores it in the value pointed to by v. Data will be read from the Decoder's
// input until EOF is reached.
//
// See the documentation for UnmarshalValue for details about the conversion of
// Alloy values into Go values.
func (dec *Decoder) DecodeValue(v any) error {
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
