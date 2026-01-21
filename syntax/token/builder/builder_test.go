package builder_test

import (
	"bytes"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/printer"
	"github.com/grafana/alloy/syntax/token"
	"github.com/grafana/alloy/syntax/token/builder"
)

func TestBuilder_File(t *testing.T) {
	f := builder.NewFile()

	f.Body().SetAttributeTokens("attr_1", []builder.Token{{Tok: token.NUMBER, Lit: "15"}})
	f.Body().SetAttributeTokens("attr_2", []builder.Token{{Tok: token.BOOL, Lit: "true"}})

	b1 := builder.NewBlock([]string{"test", "block"}, "")
	b1.Body().SetAttributeTokens("inner_attr", []builder.Token{{Tok: token.STRING, Lit: `"block 1"`}})
	f.Body().AppendBlock(b1)

	b2 := builder.NewBlock([]string{"test", "block"}, "labeled")
	b2.Body().SetAttributeTokens("inner_attr", []builder.Token{{Tok: token.STRING, Lit: "`\"block 2`"}})
	f.Body().AppendBlock(b2)

	expect := format(t, `
		attr_1 = 15
		attr_2 = true

		test.block {
			inner_attr = "block 1"
		}

		test.block "labeled" {
			inner_attr = `+"`\"block 2`"+`
		}
	`)

	require.Equal(t, expect, string(f.Bytes()))
}

type capsuleConvertibleToObject struct {
	name    string
	address string
}

func (c capsuleConvertibleToObject) ConvertInto(dst any) error {
	switch dst := dst.(type) {
	case *map[string]syntax.Value:
		result := map[string]syntax.Value{
			"name":    syntax.ValueFromString(c.name),
			"address": syntax.ValueFromString(c.address),
		}
		*dst = result
		return nil
	}
	return fmt.Errorf("capsuleConvertibleToObject: conversion to '%T' is not supported", dst)
}

func (c capsuleConvertibleToObject) AlloyCapsule() {}

var (
	_ syntax.Capsule                = capsuleConvertibleToObject{}
	_ syntax.ConvertibleIntoCapsule = capsuleConvertibleToObject{}
)

func TestBuilder_GoEncode(t *testing.T) {
	f := builder.NewFile()

	f.Body().AppendTokens([]builder.Token{{token.COMMENT, "// Hello, world!"}})
	f.Body().SetAttributeValue("null_value", nil)
	f.Body().AppendTokens([]builder.Token{{token.LITERAL, "\n"}})

	f.Body().SetAttributeValue("num", 15)
	f.Body().SetAttributeValue("string", "Hello, world!")
	f.Body().SetAttributeValue("bool", true)
	f.Body().SetAttributeValue("list", []int{0, 1, 2})
	f.Body().SetAttributeValue("func", func(int, int) int { return 0 })
	f.Body().AppendTokens([]builder.Token{{token.LITERAL, "\n"}})

	f.Body().SetAttributeValue("capsule", make(chan int))
	f.Body().SetAttributeValue("mappable_capsule", capsuleConvertibleToObject{
		name:    "Bert",
		address: "11a Sesame St",
	})
	f.Body().SetAttributeValue("mappable_capsule_ptr", &capsuleConvertibleToObject{
		name:    "Ernie",
		address: "11b Sesame St",
	})
	f.Body().AppendTokens([]builder.Token{{token.LITERAL, "\n"}})

	f.Body().SetAttributeValue("map", map[string]any{"foo": "bar"})
	f.Body().SetAttributeValue("map_2", map[string]any{"non ident": "bar"})
	f.Body().AppendTokens([]builder.Token{{token.LITERAL, "\n"}})

	f.Body().SetAttributeValue("mixed_list", []any{
		0,
		true,
		map[string]any{"key": true},
		"Hello!",
	})

	expect := format(t, `
		// Hello, world!
		null_value = null
	
		num    = 15
		string = "Hello, world!"
		bool   = true
		list   = [0, 1, 2]
		func   = function
		
		capsule          = capsule("chan int")
		mappable_capsule = {
			address = "11a Sesame St",
			name    = "Bert",
		}
		mappable_capsule_ptr = {
			address = "11b Sesame St",
			name    = "Ernie",
		}

		map = {
			foo = "bar",
		}
		map_2 = {
			"non ident" = "bar",
		}

		mixed_list = [0, true, {
			key = true,
		}, "Hello!"]
	`)

	require.Equal(t, expect, string(f.Bytes()))
}

// TestBuilder_GoEncode_SortMapKeys ensures that object literals from unordered
// values (i.e., Go maps) are printed in a deterministic order by sorting the
// keys lexicographically. Other object literals should be printed in the order
// the keys are reported in (i.e., in the order presented by the Go structs).
func TestBuilder_GoEncode_SortMapKeys(t *testing.T) {
	f := builder.NewFile()

	type Ordered struct {
		SomeKey  string `alloy:"some_key,attr"`
		OtherKey string `alloy:"other_key,attr"`
	}

	// Maps are unordered because you can't iterate over their keys in a
	// consistent order.
	var unordered = map[string]any{
		"key_a": 1,
		"key_c": 3,
		"key_b": 2,
	}

	f.Body().SetAttributeValue("ordered", Ordered{SomeKey: "foo", OtherKey: "bar"})
	f.Body().SetAttributeValue("unordered", unordered)

	expect := format(t, `
		ordered = {
			some_key  = "foo",
			other_key = "bar",
		}
		unordered = {
			key_a = 1,
			key_b = 2,
			key_c = 3,
		}
	`)

	require.Equal(t, expect, string(f.Bytes()))
}

func TestBuilder_AppendFrom(t *testing.T) {
	type InnerBlock struct {
		Number int `alloy:"number,attr"`
	}

	type Structure struct {
		Field string `alloy:"field,attr"`

		Block       InnerBlock   `alloy:"block,block"`
		OtherBlocks []InnerBlock `alloy:"other_block,block"`
	}

	f := builder.NewFile()
	f.Body().AppendFrom(Structure{
		Field: "some_value",

		Block: InnerBlock{Number: 1},
		OtherBlocks: []InnerBlock{
			{Number: 2},
			{Number: 3},
		},
	})

	expect := format(t, `
		field = "some_value"
	
		block {
			number = 1
		}

		other_block {
			number = 2
		}

		other_block {
			number = 3
		}
	`)

	require.Equal(t, expect, string(f.Bytes()))
}

func TestBuilder_AppendFrom_EnumSlice(t *testing.T) {
	type InnerBlock struct {
		Number int `alloy:"number,attr"`
	}

	type EnumBlock struct {
		BlockA InnerBlock `alloy:"a,block,optional"`
		BlockB InnerBlock `alloy:"b,block,optional"`
		BlockC InnerBlock `alloy:"c,block,optional"`
	}

	type Structure struct {
		Field string `alloy:"field,attr"`

		OtherBlocks []EnumBlock `alloy:"block,enum"`
	}

	f := builder.NewFile()
	f.Body().AppendFrom(Structure{
		Field: "some_value",
		OtherBlocks: []EnumBlock{
			{BlockC: InnerBlock{Number: 1}},
			{BlockB: InnerBlock{Number: 2}},
			{BlockC: InnerBlock{Number: 3}},
		},
	})

	expect := format(t, `
		field = "some_value"
	
		block.c {
			number = 1
		}

		block.b {
			number = 2
		}

		block.c {
			number = 3
		}
	`)

	require.Equal(t, expect, string(f.Bytes()))
}

func TestBuilder_AppendFrom_EnumSlice_Pointer(t *testing.T) {
	type InnerBlock struct {
		Number int `alloy:"number,attr"`
	}

	type EnumBlock struct {
		BlockA *InnerBlock `alloy:"a,block,optional"`
		BlockB *InnerBlock `alloy:"b,block,optional"`
		BlockC *InnerBlock `alloy:"c,block,optional"`
	}

	type Structure struct {
		Field string `alloy:"field,attr"`

		OtherBlocks []EnumBlock `alloy:"block,enum"`
	}

	f := builder.NewFile()
	f.Body().AppendFrom(Structure{
		Field: "some_value",
		OtherBlocks: []EnumBlock{
			{BlockC: &InnerBlock{Number: 1}},
			{BlockB: &InnerBlock{Number: 2}},
			{BlockC: &InnerBlock{Number: 3}},
		},
	})

	expect := format(t, `
		field = "some_value"
	
		block.c {
			number = 1
		}

		block.b {
			number = 2
		}

		block.c {
			number = 3
		}
	`)

	require.Equal(t, expect, string(f.Bytes()))
}

func TestBuilder_SkipOptional(t *testing.T) {
	type Structure struct {
		OptFieldA string `alloy:"opt_field_a,attr,optional"`
		OptFieldB string `alloy:"opt_field_b,attr,optional"`
		ReqFieldA string `alloy:"req_field_a,attr"`
		ReqFieldB string `alloy:"req_field_b,attr"`
	}

	f := builder.NewFile()
	f.Body().AppendFrom(Structure{
		OptFieldA: "some_value",
		OptFieldB: "", // Zero value
		ReqFieldA: "some_value",
		ReqFieldB: "", // Zero value
	})

	expect := format(t, `
		opt_field_a = "some_value"
		req_field_a = "some_value"
		req_field_b = ""
	`)

	require.Equal(t, expect, string(f.Bytes()))
}

func format(t *testing.T, in string) string {
	t.Helper()

	f, err := parser.ParseFile(t.Name(), []byte(in))
	require.NoError(t, err)

	var buf bytes.Buffer
	require.NoError(t, printer.Fprint(&buf, f))

	return buf.String()
}

type CustomTokenizer bool

var _ builder.Tokenizer = (CustomTokenizer)(false)

func (ct CustomTokenizer) AlloyTokenize() []builder.Token {
	return []builder.Token{{Tok: token.LITERAL, Lit: "CUSTOM_TOKENS"}}
}

func TestBuilder_GoEncode_Tokenizer(t *testing.T) {
	t.Run("Tokenizer", func(t *testing.T) {
		f := builder.NewFile()
		f.Body().SetAttributeValue("value", CustomTokenizer(true))

		expect := format(t, `value = CUSTOM_TOKENS`)
		require.Equal(t, expect, string(f.Bytes()))
	})

	t.Run("TextMarshaler", func(t *testing.T) {
		now := time.Now()
		expectBytes, err := now.MarshalText()
		require.NoError(t, err)

		f := builder.NewFile()
		f.Body().SetAttributeValue("value", now)

		expect := format(t, fmt.Sprintf(`value = %q`, string(expectBytes)))
		require.Equal(t, expect, string(f.Bytes()))
	})

	t.Run("Duration", func(t *testing.T) {
		dur := 15 * time.Second

		f := builder.NewFile()
		f.Body().SetAttributeValue("value", dur)

		expect := format(t, fmt.Sprintf(`value = %q`, dur.String()))
		require.Equal(t, expect, string(f.Bytes()))
	})
}

func TestBuilder_ValueOverrideHook(t *testing.T) {
	type InnerBlock struct {
		AnotherField string `alloy:"another_field,attr"`
	}

	type Structure struct {
		Field string `alloy:"field,attr"`

		Block       InnerBlock   `alloy:"block,block"`
		OtherBlocks []InnerBlock `alloy:"other_block,block"`
	}

	f := builder.NewFile()
	f.Body().SetValueOverrideHook(func(val any) any {
		return "some other value"
	})
	f.Body().AppendFrom(Structure{
		Field: "some_value",

		Block: InnerBlock{AnotherField: "some_value"},
		OtherBlocks: []InnerBlock{
			{AnotherField: "some_value"},
			{AnotherField: "some_value"},
		},
	})

	expect := format(t, `
		field = "some other value"
	
		block {
			another_field = "some other value"
		}

		other_block {
			another_field = "some other value"
		}

		other_block {
			another_field = "some other value"
		}
	`)

	require.Equal(t, expect, string(f.Bytes()))
}

func TestBuilder_MapBlocks(t *testing.T) {
	type block struct {
		Value map[string]any `alloy:"block,block,optional"`
	}

	f := builder.NewFile()
	f.Body().AppendFrom(block{
		Value: map[string]any{
			"field": "value",
		},
	})

	expect := format(t, `
		block {
			field = "value"
		}
	`)

	require.Equal(t, expect, string(f.Bytes()))
}
