// Package builder exposes an API to create a River configuration file by
// constructing a set of tokens.
package builder

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/grafana/river/internal/reflectutil"
	"github.com/grafana/river/internal/rivertags"
	"github.com/grafana/river/internal/value"
	"github.com/grafana/river/token"
)

var goRiverDefaulter = reflect.TypeOf((*value.Defaulter)(nil)).Elem()

// An Expr represents a single River expression.
type Expr struct {
	rawTokens []Token
}

// NewExpr creates a new Expr.
func NewExpr() *Expr { return &Expr{} }

// Tokens returns the Expr as a set of Tokens.
func (e *Expr) Tokens() []Token { return e.rawTokens }

// SetValue sets the Expr to a River value converted from a Go value. The Go
// value is encoded using the normal Go to River encoding rules. If any value
// reachable from goValue implements Tokenizer, the printed tokens will instead
// be retrieved by calling the RiverTokenize method.
func (e *Expr) SetValue(goValue interface{}) {
	e.rawTokens = tokenEncode(goValue)
}

// WriteTo renders and formats the File, writing the contents to w.
func (e *Expr) WriteTo(w io.Writer) (int64, error) {
	n, err := printExprTokens(w, e.Tokens())
	return int64(n), err
}

// Bytes renders the File to a formatted byte slice.
func (e *Expr) Bytes() []byte {
	var buf bytes.Buffer
	_, _ = e.WriteTo(&buf)
	return buf.Bytes()
}

// A File represents a River configuration file.
type File struct {
	body *Body
}

// NewFile creates a new File.
func NewFile() *File { return &File{body: newBody()} }

// Tokens returns the File as a set of Tokens.
func (f *File) Tokens() []Token { return f.Body().Tokens() }

// Body returns the Body contents of the file.
func (f *File) Body() *Body { return f.body }

// WriteTo renders and formats the File, writing the contents to w.
func (f *File) WriteTo(w io.Writer) (int64, error) {
	n, err := printFileTokens(w, f.Tokens())
	return int64(n), err
}

// Bytes renders the File to a formatted byte slice.
func (f *File) Bytes() []byte {
	var buf bytes.Buffer
	_, _ = f.WriteTo(&buf)
	return buf.Bytes()
}

// Body is a list of block and attribute statements. A Body cannot be manually
// created, but is retrieved from a File or Block.
type Body struct {
	nodes             []tokenNode
	valueOverrideHook ValueOverrideHook
}

type ValueOverrideHook = func(val interface{}) interface{}

// SetValueOverrideHook sets a hook to override the value that will be token
// encoded. The hook can mutate the value to be encoded or should return it
// unmodified. This hook can be skipped by leaving it nil or setting it to nil.
func (b *Body) SetValueOverrideHook(valueOverrideHook ValueOverrideHook) {
	b.valueOverrideHook = valueOverrideHook
}

func (b *Body) Nodes() []tokenNode {
	return b.nodes
}

// A tokenNode is a structural element which can be converted into a set of
// Tokens.
type tokenNode interface {
	// Tokens builds the set of Tokens from the node.
	Tokens() []Token
}

func newBody() *Body {
	return &Body{}
}

// Tokens returns the File as a set of Tokens.
func (b *Body) Tokens() []Token {
	var rawToks []Token
	for i, node := range b.nodes {
		rawToks = append(rawToks, node.Tokens()...)

		if i+1 < len(b.nodes) {
			// Append a terminator between each statement in the Body.
			rawToks = append(rawToks, Token{
				Tok: token.LITERAL,
				Lit: "\n",
			})
		}
	}
	return rawToks
}

// AppendTokens appends raw tokens to the Body.
func (b *Body) AppendTokens(tokens []Token) {
	b.nodes = append(b.nodes, tokensSlice(tokens))
}

// AppendBlock adds a new block inside of the Body.
func (b *Body) AppendBlock(block *Block) {
	b.nodes = append(b.nodes, block)
}

// AppendFrom sets attributes and appends blocks defined by goValue into the
// Body. If any value reachable from goValue implements Tokenizer, the printed
// tokens will instead be retrieved by calling the RiverTokenize method.
//
// Optional attributes and blocks set to default values are trimmed.
// If goValue implements Defaulter, default values are retrieved by
// calling SetToDefault against a copy. Otherwise, default values are
// the zero value of the respective Go types.
//
// goValue must be a struct or a pointer to a struct that contains River struct
// tags.
func (b *Body) AppendFrom(goValue interface{}) {
	if goValue == nil {
		return
	}

	rv := reflect.ValueOf(goValue)
	b.encodeFields(rv)
}

// getBlockLabel returns the label for a given block.
func getBlockLabel(rv reflect.Value) string {
	tags := rivertags.Get(rv.Type())
	for _, tag := range tags {
		if tag.Flags&rivertags.FlagLabel != 0 {
			return reflectutil.Get(rv, tag).String()
		}
	}

	return ""
}

func (b *Body) encodeFields(rv reflect.Value) {
	for rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return
		}
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		panic(fmt.Sprintf("river/token/builder: can only encode struct values to bodies, got %s", rv.Type()))
	}

	fields := rivertags.Get(rv.Type())
	defaults := reflect.New(rv.Type()).Elem()
	if defaults.CanAddr() && defaults.Addr().Type().Implements(goRiverDefaulter) {
		defaults.Addr().Interface().(value.Defaulter).SetToDefault()
	}

	for _, field := range fields {
		fieldVal := reflectutil.Get(rv, field)
		fieldValDefault := reflectutil.Get(defaults, field)

		// Check if the values are exactly equal or if they're both equal to the
		// zero value. Checking for both fields being zero handles the case where
		// an empty and nil map are being compared (which are not equal, but are
		// both zero values).
		matchesDefault := reflect.DeepEqual(fieldVal.Interface(), fieldValDefault.Interface())
		isZero := fieldValDefault.IsZero() && fieldVal.IsZero()

		if field.IsOptional() && (matchesDefault || isZero) {
			continue
		}

		b.encodeField(nil, field, fieldVal)
	}
}

func (b *Body) encodeField(prefix []string, field rivertags.Field, fieldValue reflect.Value) {
	fieldName := strings.Join(field.Name, ".")

	for fieldValue.Kind() == reflect.Pointer {
		if fieldValue.IsNil() {
			break
		}
		fieldValue = fieldValue.Elem()
	}

	switch {
	case field.IsAttr():
		b.SetAttributeValue(fieldName, fieldValue.Interface())

	case field.IsBlock():
		fullName := mergeStringSlice(prefix, field.Name)

		switch {
		case fieldValue.Kind() == reflect.Map:
			// Iterate over the map and add each element as an attribute into it.
			if fieldValue.Type().Key().Kind() != reflect.String {
				panic("river/token/builder: unsupported map type for block; expected map[string]T, got " + fieldValue.Type().String())
			}

			inner := NewBlock(fullName, "")
			inner.body.SetValueOverrideHook(b.valueOverrideHook)
			b.AppendBlock(inner)

			iter := fieldValue.MapRange()
			for iter.Next() {
				mapKey, mapValue := iter.Key(), iter.Value()
				inner.body.SetAttributeValue(mapKey.String(), mapValue.Interface())
			}

		case fieldValue.Kind() == reflect.Slice, fieldValue.Kind() == reflect.Array:
			for i := 0; i < fieldValue.Len(); i++ {
				elem := fieldValue.Index(i)

				// Recursively call encodeField for each element in the slice/array for
				// non-zero blocks. The recursive call will hit the case below and add
				// a new block for each field encountered.
				if field.Flags&rivertags.FlagOptional != 0 && elem.IsZero() {
					continue
				}
				b.encodeField(prefix, field, elem)
			}

		case fieldValue.Kind() == reflect.Struct:
			inner := NewBlock(fullName, getBlockLabel(fieldValue))
			inner.body.SetValueOverrideHook(b.valueOverrideHook)
			inner.Body().encodeFields(fieldValue)
			b.AppendBlock(inner)
		}

	case field.IsEnum():
		// Blocks within an enum have a prefix set.
		newPrefix := mergeStringSlice(prefix, field.Name)

		switch {
		case fieldValue.Kind() == reflect.Slice, fieldValue.Kind() == reflect.Array:
			for i := 0; i < fieldValue.Len(); i++ {
				b.encodeEnumElement(newPrefix, fieldValue.Index(i))
			}

		default:
			panic(fmt.Sprintf("river/token/builder: unrecognized enum kind %s", fieldValue.Kind()))
		}
	}
}

func mergeStringSlice(a, b []string) []string {
	if len(a) == 0 {
		return b
	} else if len(b) == 0 {
		return a
	}

	res := make([]string, 0, len(a)+len(b))
	res = append(res, a...)
	res = append(res, b...)
	return res
}

func (b *Body) encodeEnumElement(prefix []string, enumElement reflect.Value) {
	for enumElement.Kind() == reflect.Pointer {
		if enumElement.IsNil() {
			return
		}
		enumElement = enumElement.Elem()
	}

	fields := rivertags.Get(enumElement.Type())

	// Find the first non-zero field and encode it.
	for _, field := range fields {
		fieldVal := reflectutil.Get(enumElement, field)
		if !fieldVal.IsValid() || fieldVal.IsZero() {
			continue
		}

		b.encodeField(prefix, field, fieldVal)
		break
	}
}

// SetAttributeTokens sets an attribute to the Body whose value is a set of raw
// tokens. If the attribute was previously set, its value tokens are updated.
//
// Attributes will be written out in the order they were initially created.
func (b *Body) SetAttributeTokens(name string, tokens []Token) {
	attr := b.getOrCreateAttribute(name)
	attr.RawTokens = tokens
}

func (b *Body) getOrCreateAttribute(name string) *attribute {
	for _, n := range b.nodes {
		if attr, ok := n.(*attribute); ok && attr.Name == name {
			return attr
		}
	}

	newAttr := &attribute{Name: name}
	b.nodes = append(b.nodes, newAttr)
	return newAttr
}

// SetAttributeValue sets an attribute in the Body whose value is converted
// from a Go value to a River value. The Go value is encoded using the normal
// Go to River encoding rules. If any value reachable from goValue implements
// Tokenizer, the printed tokens will instead be retrieved by calling the
// RiverTokenize method.
//
// If the attribute was previously set, its value tokens are updated.
//
// Attributes will be written out in the order they were initially crated.
func (b *Body) SetAttributeValue(name string, goValue interface{}) {
	attr := b.getOrCreateAttribute(name)

	if b.valueOverrideHook != nil {
		attr.RawTokens = tokenEncode(b.valueOverrideHook(goValue))
	} else {
		attr.RawTokens = tokenEncode(goValue)
	}
}

type attribute struct {
	Name      string
	RawTokens []Token
}

func (attr *attribute) Tokens() []Token {
	var toks []Token

	toks = append(toks, Token{Tok: token.IDENT, Lit: attr.Name})
	toks = append(toks, Token{Tok: token.ASSIGN})
	toks = append(toks, attr.RawTokens...)

	return toks
}

// A Block encapsulates a body within a named and labeled River block. Blocks
// must be created by calling NewBlock, but its public struct fields may be
// safely modified by callers.
type Block struct {
	// Public fields, safe to be changed by callers:

	Name  []string
	Label string

	// Private fields:

	body *Body
}

// NewBlock returns a new Block with the given name and label. The name/label
// can be updated later by modifying the Block's public fields.
func NewBlock(name []string, label string) *Block {
	return &Block{
		Name:  name,
		Label: label,

		body: newBody(),
	}
}

// Tokens returns the File as a set of Tokens.
func (b *Block) Tokens() []Token {
	var toks []Token

	for i, frag := range b.Name {
		toks = append(toks, Token{Tok: token.IDENT, Lit: frag})
		if i+1 < len(b.Name) {
			toks = append(toks, Token{Tok: token.DOT})
		}
	}

	toks = append(toks, Token{Tok: token.LITERAL, Lit: " "})

	if b.Label != "" {
		toks = append(toks, Token{Tok: token.STRING, Lit: fmt.Sprintf("%q", b.Label)})
	}

	toks = append(toks, Token{Tok: token.LCURLY}, Token{Tok: token.LITERAL, Lit: "\n"})
	toks = append(toks, b.body.Tokens()...)
	toks = append(toks, Token{Tok: token.LITERAL, Lit: "\n"}, Token{Tok: token.RCURLY})

	return toks
}

// Body returns the Body contained within the Block.
func (b *Block) Body() *Body { return b.body }

type tokensSlice []Token

func (tn tokensSlice) Tokens() []Token { return []Token(tn) }
