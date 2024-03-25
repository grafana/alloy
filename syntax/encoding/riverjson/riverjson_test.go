package riverjson_test

import (
	"testing"

	river "github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/alloy/syntax/encoding/riverjson"
	"github.com/stretchr/testify/require"
)

func TestValues(t *testing.T) {
	tt := []struct {
		name       string
		input      interface{}
		expectJSON string
	}{
		{
			name:       "null",
			input:      nil,
			expectJSON: `{ "type": "null", "value": null }`,
		},
		{
			name:       "number",
			input:      54,
			expectJSON: `{ "type": "number", "value": 54 }`,
		},
		{
			name:       "string",
			input:      "Hello, world!",
			expectJSON: `{ "type": "string", "value": "Hello, world!" }`,
		},
		{
			name:       "bool",
			input:      true,
			expectJSON: `{ "type": "bool", "value": true }`,
		},
		{
			name:  "simple array",
			input: []int{0, 1, 2, 3, 4},
			expectJSON: `{
				"type": "array",
				"value": [
						{ "type": "number", "value": 0 },
						{ "type": "number", "value": 1 },
						{ "type": "number", "value": 2 },
						{ "type": "number", "value": 3 },
						{ "type": "number", "value": 4 }
				]
			}`,
		},
		{
			name:  "nested array",
			input: []interface{}{"testing", []int{0, 1, 2}},
			expectJSON: `{
				"type": "array",
				"value": [
						{ "type": "string", "value": "testing" },
						{
							"type": "array",
							"value": [
								{ "type": "number", "value": 0 },
								{ "type": "number", "value": 1 },
								{ "type": "number", "value": 2 }
							]
						}
				]
			}`,
		},
		{
			name:  "object",
			input: map[string]any{"foo": "bar", "fizz": "buzz", "year": 2023},
			expectJSON: `{
				"type": "object",
				"value": [
					{ "key": "fizz", "value": { "type": "string", "value": "buzz" }},
					{ "key": "foo", "value": { "type": "string", "value": "bar" }},
					{ "key": "year", "value": { "type": "number", "value": 2023 }}
				]
			}`,
		},
		{
			name:       "function",
			input:      func(i int) int { return i * 2 },
			expectJSON: `{ "type": "function", "value": "function" }`,
		},
		{
			name:       "capsule",
			input:      alloytypes.Secret("foo"),
			expectJSON: `{ "type": "capsule", "value": "(secret)" }`,
		},
		{
			// nil arrays and objects must always be [] instead of null as that's
			// what the API definition says they should be.
			name:       "nil array",
			input:      ([]any)(nil),
			expectJSON: `{ "type": "array", "value": [] }`,
		},
		{
			// nil arrays and objects must always be [] instead of null as that's
			// what the API definition says they should be.
			name:       "nil object",
			input:      (map[string]any)(nil),
			expectJSON: `{ "type": "object", "value": [] }`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := riverjson.MarshalValue(tc.input)
			require.NoError(t, err)
			require.JSONEq(t, tc.expectJSON, string(actual))
		})
	}
}

func TestBlock(t *testing.T) {
	// Zero values should be omitted from result.

	val := testBlock{
		Number: 5,
		Array:  []any{1, 2, 3},
		Labeled: []labeledBlock{
			{
				TestBlock: testBlock{Boolean: true},
				Label:     "label_a",
			},
			{
				TestBlock: testBlock{String: "foo"},
				Label:     "label_b",
			},
		},
		Blocks: []testBlock{
			{String: "hello"},
			{String: "world"},
		},
	}

	expect := `[
		{ 
			"name": "number", 
			"type": "attr", 
			"value": { "type": "number", "value": 5 }
		},
		{
			"name": "array",
			"type": "attr",
			"value": { 
				"type": "array",
				"value": [
					{ "type": "number", "value": 1 },
					{ "type": "number", "value": 2 },
					{ "type": "number", "value": 3 }
				]
			}
		},
		{
			"name": "labeled_block",
			"type": "block",
			"label": "label_a",
			"body": [{
				"name": "boolean",
				"type": "attr",
				"value": { "type": "bool", "value": true }
			}]
		},
		{
			"name": "labeled_block",
			"type": "block",
			"label": "label_b",
			"body": [{
				"name": "string",
				"type": "attr",
				"value": { "type": "string", "value": "foo" }
			}]
		},
		{
			"name": "inner_block",
			"type": "block",
			"body": [{
				"name": "string",
				"type": "attr",
				"value": { "type": "string", "value": "hello" }
			}]
		},
		{
			"name": "inner_block",
			"type": "block",
			"body": [{
				"name": "string",
				"type": "attr",
				"value": { "type": "string", "value": "world" }
			}]
		}
	]`

	actual, err := riverjson.MarshalBody(val)
	require.NoError(t, err)
	require.JSONEq(t, expect, string(actual))
}

func TestBlock_Empty_Required_Block_Slice(t *testing.T) {
	type wrapper struct {
		Blocks []testBlock `river:"some_block,block"`
	}

	tt := []struct {
		name string
		val  any
	}{
		{"nil block slice", wrapper{Blocks: nil}},
		{"empty block slice", wrapper{Blocks: []testBlock{}}},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			expect := `[]`

			actual, err := riverjson.MarshalBody(tc.val)
			require.NoError(t, err)
			require.JSONEq(t, expect, string(actual))
		})
	}
}

type testBlock struct {
	Number  int            `river:"number,attr,optional"`
	String  string         `river:"string,attr,optional"`
	Boolean bool           `river:"boolean,attr,optional"`
	Array   []any          `river:"array,attr,optional"`
	Object  map[string]any `river:"object,attr,optional"`

	Labeled []labeledBlock `river:"labeled_block,block,optional"`
	Blocks  []testBlock    `river:"inner_block,block,optional"`
}

type labeledBlock struct {
	TestBlock testBlock `river:",squash"`
	Label     string    `river:",label"`
}

func TestNilBody(t *testing.T) {
	actual, err := riverjson.MarshalBody(nil)
	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(actual))
}

func TestEmptyBody(t *testing.T) {
	type block struct{}

	actual, err := riverjson.MarshalBody(block{})
	require.NoError(t, err)
	require.JSONEq(t, `[]`, string(actual))
}

func TestHideDefaults(t *testing.T) {
	tt := []struct {
		name       string
		val        defaultsBlock
		expectJSON string
	}{
		{
			name: "no defaults",
			val: defaultsBlock{
				Name: "Jane",
				Age:  41,
			},
			expectJSON: `[
				{ "name": "name", "type": "attr", "value": { "type": "string", "value": "Jane" }},
				{ "name": "age", "type": "attr", "value": { "type": "number", "value": 41 }}
			]`,
		},
		{
			name: "some defaults",
			val: defaultsBlock{
				Name: "John Doe",
				Age:  41,
			},
			expectJSON: `[
				{ "name": "age", "type": "attr", "value": { "type": "number", "value": 41 }}
			]`,
		},
		{
			name: "all defaults",
			val: defaultsBlock{
				Name: "John Doe",
				Age:  35,
			},
			expectJSON: `[]`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := riverjson.MarshalBody(tc.val)
			require.NoError(t, err)
			require.JSONEq(t, tc.expectJSON, string(actual))
		})
	}
}

type defaultsBlock struct {
	Name string `river:"name,attr,optional"`
	Age  int    `river:"age,attr,optional"`
}

var _ river.Defaulter = (*defaultsBlock)(nil)

func (d *defaultsBlock) SetToDefault() {
	*d = defaultsBlock{
		Name: "John Doe",
		Age:  35,
	}
}

func TestMapBlocks(t *testing.T) {
	type block struct {
		Value map[string]any `river:"block,block,optional"`
	}
	val := block{Value: map[string]any{"field": "value"}}

	expect := `[{
		"name": "block",
		"type": "block",
		"body": [{
			"name": "field",
			"type": "attr",
			"value": { "type": "string", "value": "value" }
		}]
	}]`

	bb, err := riverjson.MarshalBody(val)
	require.NoError(t, err)
	require.JSONEq(t, expect, string(bb))
}

func TestRawMap(t *testing.T) {
	val := map[string]any{"field": "value"}

	expect := `[{
        "name": "field",
        "type": "attr",
        "value": { "type": "string", "value": "value" }
    }]`

	bb, err := riverjson.MarshalBody(val)
	require.NoError(t, err)
	require.JSONEq(t, expect, string(bb))
}

func TestRawMap_Capsule(t *testing.T) {
	val := map[string]any{"capsule": alloytypes.Secret("foo")}

	expect := `[{
        "name": "capsule",
        "type": "attr",
        "value": { "type": "capsule", "value": "(secret)" }
    }]`

	bb, err := riverjson.MarshalBody(val)
	require.NoError(t, err)
	require.JSONEq(t, expect, string(bb))
}
