package vm_test

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/grafana/river/ast"
	"github.com/grafana/river/parser"
	"github.com/grafana/river/vm"
	"github.com/stretchr/testify/require"
)

// This file contains tests for decoding blocks.

func TestVM_File(t *testing.T) {
	type block struct {
		String string `river:"string,attr"`
		Number int    `river:"number,attr,optional"`
	}
	type file struct {
		SettingA int `river:"setting_a,attr"`
		SettingB int `river:"setting_b,attr,optional"`

		Block block `river:"some_block,block,optional"`
	}

	input := `
	setting_a = 15 

	some_block {
		string = "Hello, world!"
	}
	`

	expect := file{
		SettingA: 15,
		Block: block{
			String: "Hello, world!",
		},
	}

	res, err := parser.ParseFile(t.Name(), []byte(input))
	require.NoError(t, err)

	eval := vm.New(res)

	var actual file
	require.NoError(t, eval.Evaluate(nil, &actual))
	require.Equal(t, expect, actual)
}

func TestVM_Block_Attributes(t *testing.T) {
	t.Run("Decodes attributes", func(t *testing.T) {
		type block struct {
			Number int    `river:"number,attr"`
			String string `river:"string,attr"`
		}

		input := `some_block {
			number = 15 
			string = "Hello, world!"
		}`
		eval := vm.New(parseBlock(t, input))

		var actual block
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, 15, actual.Number)
		require.Equal(t, "Hello, world!", actual.String)
	})

	t.Run("Fails if attribute used as block", func(t *testing.T) {
		type block struct {
			Number int `river:"number,attr"`
		}

		input := `some_block {
			number {} 
		}`
		eval := vm.New(parseBlock(t, input))

		err := eval.Evaluate(nil, &block{})
		require.EqualError(t, err, `2:4: "number" must be an attribute, but is used as a block`)
	})

	t.Run("Fails if required attributes are not present", func(t *testing.T) {
		type block struct {
			Number int    `river:"number,attr"`
			String string `river:"string,attr"`
		}

		input := `some_block {
			number = 15 
		}`
		eval := vm.New(parseBlock(t, input))

		err := eval.Evaluate(nil, &block{})
		require.EqualError(t, err, `missing required attribute "string"`)
	})

	t.Run("Succeeds if optional attributes are not present", func(t *testing.T) {
		type block struct {
			Number int    `river:"number,attr"`
			String string `river:"string,attr,optional"`
		}

		input := `some_block {
			number = 15 
		}`
		eval := vm.New(parseBlock(t, input))

		var actual block
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, 15, actual.Number)
		require.Equal(t, "", actual.String)
	})

	t.Run("Fails if attribute is not defined in struct", func(t *testing.T) {
		type block struct {
			Number int `river:"number,attr"`
		}

		input := `some_block {
			number  = 15 
			invalid = "This attribute does not exist!"
		}`
		eval := vm.New(parseBlock(t, input))

		err := eval.Evaluate(nil, &block{})
		require.EqualError(t, err, `3:4: unrecognized attribute name "invalid"`)
	})

	t.Run("Tests decoding into an interface", func(t *testing.T) {
		type block struct {
			Anything interface{} `river:"anything,attr"`
		}

		tests := []struct {
			testName        string
			val             string
			expectedValType reflect.Kind
		}{
			{testName: "test_int_1", val: "15", expectedValType: reflect.Int},
			{testName: "test_int_2", val: "-15", expectedValType: reflect.Int},
			{testName: "test_int_3", val: fmt.Sprintf("%v", math.MaxInt64), expectedValType: reflect.Int},
			{testName: "test_int_4", val: fmt.Sprintf("%v", math.MinInt64), expectedValType: reflect.Int},
			{testName: "test_uint_1", val: fmt.Sprintf("%v", uint64(math.MaxInt64)+1), expectedValType: reflect.Uint64},
			{testName: "test_uint_2", val: fmt.Sprintf("%v", uint64(math.MaxUint64)), expectedValType: reflect.Uint64},
			{testName: "test_float_1", val: fmt.Sprintf("%v9", math.MinInt64), expectedValType: reflect.Float64},
			{testName: "test_float_2", val: fmt.Sprintf("%v9", uint64(math.MaxUint64)), expectedValType: reflect.Float64},
			{testName: "test_float_3", val: "16.0", expectedValType: reflect.Float64},
		}

		for _, tt := range tests {
			t.Run(tt.testName, func(t *testing.T) {
				input := fmt.Sprintf(`some_block {
					anything  = %s 
				}`, tt.val)
				eval := vm.New(parseBlock(t, input))

				var actual block
				err := eval.Evaluate(nil, &actual)
				require.NoError(t, err)
				require.Equal(t, tt.expectedValType.String(), reflect.TypeOf(actual.Anything).Kind().String())
			})
		}
	})

	t.Run("Supports arbitrarily nested struct pointer fields", func(t *testing.T) {
		type block struct {
			NumberA int    `river:"number_a,attr"`
			NumberB *int   `river:"number_b,attr"`
			NumberC **int  `river:"number_c,attr"`
			NumberD ***int `river:"number_d,attr"`
		}

		input := `some_block {
			number_a = 15 
			number_b = 20
			number_c = 25
			number_d = 30
		}`
		eval := vm.New(parseBlock(t, input))

		var actual block
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, 15, actual.NumberA)
		require.Equal(t, 20, *actual.NumberB)
		require.Equal(t, 25, **actual.NumberC)
		require.Equal(t, 30, ***actual.NumberD)
	})

	t.Run("Supports squashed attributes", func(t *testing.T) {
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
			input = `some_block {
				outer_field_1 = "value1"
				outer_field_2 = "value2"
				inner_field_1 = "value3"
				inner_field_2 = "value4"
			}`

			expect = OuterStruct{
				OuterField1: "value1",
				Inner: InnerStruct{
					InnerField1: "value3",
					InnerField2: "value4",
				},
				OuterField2: "value2",
			}
		)
		eval := vm.New(parseBlock(t, input))

		var actual OuterStruct
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, expect, actual)
	})

	t.Run("Supports squashed attributes in pointers", func(t *testing.T) {
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
			input = `some_block {
				outer_field_1 = "value1"
				outer_field_2 = "value2"
				inner_field_1 = "value3"
				inner_field_2 = "value4"
			}`

			expect = OuterStruct{
				OuterField1: "value1",
				Inner: &InnerStruct{
					InnerField1: "value3",
					InnerField2: "value4",
				},
				OuterField2: "value2",
			}
		)
		eval := vm.New(parseBlock(t, input))

		var actual OuterStruct
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, expect, actual)
	})
}

func TestVM_Block_Children_Blocks(t *testing.T) {
	type childBlock struct {
		Attr bool `river:"attr,attr"`
	}

	t.Run("Decodes children blocks", func(t *testing.T) {
		type block struct {
			Value int        `river:"value,attr"`
			Child childBlock `river:"child.block,block"`
		}

		input := `some_block {
			value = 15 

			child.block { attr = true }
		}`
		eval := vm.New(parseBlock(t, input))

		var actual block
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, 15, actual.Value)
		require.True(t, actual.Child.Attr)
	})

	t.Run("Decodes multiple instances of children blocks", func(t *testing.T) {
		type block struct {
			Value    int          `river:"value,attr"`
			Children []childBlock `river:"child.block,block"`
		}

		input := `some_block {
			value = 10 

			child.block { attr = true }
			child.block { attr = false }
			child.block { attr = true }
		}`
		eval := vm.New(parseBlock(t, input))

		var actual block
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, 10, actual.Value)
		require.Len(t, actual.Children, 3)
		require.Equal(t, true, actual.Children[0].Attr)
		require.Equal(t, false, actual.Children[1].Attr)
		require.Equal(t, true, actual.Children[2].Attr)
	})

	t.Run("Decodes multiple instances of children blocks into an array", func(t *testing.T) {
		type block struct {
			Value    int           `river:"value,attr"`
			Children [3]childBlock `river:"child.block,block"`
		}

		input := `some_block {
			value = 15

			child.block { attr = true }
			child.block { attr = false }
			child.block { attr = true }
		}`
		eval := vm.New(parseBlock(t, input))

		var actual block
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, 15, actual.Value)
		require.Equal(t, true, actual.Children[0].Attr)
		require.Equal(t, false, actual.Children[1].Attr)
		require.Equal(t, true, actual.Children[2].Attr)
	})

	t.Run("Fails if block used as an attribute", func(t *testing.T) {
		type block struct {
			Child childBlock `river:"child,block"`
		}

		input := `some_block {
			child = 15
		}`
		eval := vm.New(parseBlock(t, input))

		err := eval.Evaluate(nil, &block{})
		require.EqualError(t, err, `2:4: "child" must be a block, but is used as an attribute`)
	})

	t.Run("Fails if required children blocks are not present", func(t *testing.T) {
		type block struct {
			Value int        `river:"value,attr"`
			Child childBlock `river:"child.block,block"`
		}

		input := `some_block {
			value = 15
		}`
		eval := vm.New(parseBlock(t, input))

		err := eval.Evaluate(nil, &block{})
		require.EqualError(t, err, `missing required block "child.block"`)
	})

	t.Run("Succeeds if optional children blocks are not present", func(t *testing.T) {
		type block struct {
			Value int        `river:"value,attr"`
			Child childBlock `river:"child.block,block,optional"`
		}

		input := `some_block {
			value = 15 
		}`
		eval := vm.New(parseBlock(t, input))

		var actual block
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, 15, actual.Value)
	})

	t.Run("Fails if child block is not defined in struct", func(t *testing.T) {
		type block struct {
			Value int `river:"value,attr"`
		}

		input := `some_block {
			value = 15

			child.block { attr = true }
		}`
		eval := vm.New(parseBlock(t, input))

		err := eval.Evaluate(nil, &block{})
		require.EqualError(t, err, `4:4: unrecognized block name "child.block"`)
	})

	t.Run("Supports arbitrarily nested struct pointer fields", func(t *testing.T) {
		type block struct {
			BlockA childBlock    `river:"block_a,block"`
			BlockB *childBlock   `river:"block_b,block"`
			BlockC **childBlock  `river:"block_c,block"`
			BlockD ***childBlock `river:"block_d,block"`
		}

		input := `some_block {
			block_a { attr = true } 
			block_b { attr = false } 
			block_c { attr = true } 
			block_d { attr = false } 
		}`
		eval := vm.New(parseBlock(t, input))

		var actual block
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, true, (actual.BlockA).Attr)
		require.Equal(t, false, (*actual.BlockB).Attr)
		require.Equal(t, true, (**actual.BlockC).Attr)
		require.Equal(t, false, (***actual.BlockD).Attr)
	})

	t.Run("Supports squashed blocks", func(t *testing.T) {
		type InnerStruct struct {
			Inner1 childBlock `river:"inner_block_1,block"`
			Inner2 childBlock `river:"inner_block_2,block"`
		}

		type OuterStruct struct {
			Outer1 childBlock  `river:"outer_block_1,block"`
			Inner  InnerStruct `river:",squash"`
			Outer2 childBlock  `river:"outer_block_2,block"`
		}

		var (
			input = `some_block {
				outer_block_1 { attr = true }
				outer_block_2 { attr = false }
				inner_block_1 { attr = true } 
				inner_block_2 { attr = false } 
			}`

			expect = OuterStruct{
				Outer1: childBlock{Attr: true},
				Outer2: childBlock{Attr: false},
				Inner: InnerStruct{
					Inner1: childBlock{Attr: true},
					Inner2: childBlock{Attr: false},
				},
			}
		)
		eval := vm.New(parseBlock(t, input))

		var actual OuterStruct
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, expect, actual)
	})

	t.Run("Supports squashed blocks in pointers", func(t *testing.T) {
		type InnerStruct struct {
			Inner1 *childBlock `river:"inner_block_1,block"`
			Inner2 *childBlock `river:"inner_block_2,block"`
		}

		type OuterStruct struct {
			Outer1 childBlock   `river:"outer_block_1,block"`
			Inner  *InnerStruct `river:",squash"`
			Outer2 childBlock   `river:"outer_block_2,block"`
		}

		var (
			input = `some_block {
				outer_block_1 { attr = true }
				outer_block_2 { attr = false }
				inner_block_1 { attr = true } 
				inner_block_2 { attr = false } 
			}`

			expect = OuterStruct{
				Outer1: childBlock{Attr: true},
				Outer2: childBlock{Attr: false},
				Inner: &InnerStruct{
					Inner1: &childBlock{Attr: true},
					Inner2: &childBlock{Attr: false},
				},
			}
		)
		eval := vm.New(parseBlock(t, input))

		var actual OuterStruct
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, expect, actual)
	})

	// TODO(rfratto): decode all blocks into a []*ast.BlockStmt field.
}

func TestVM_Block_Enum_Block(t *testing.T) {
	type childBlock struct {
		Attr int `river:"attr,attr"`
	}

	type enumBlock struct {
		BlockA *childBlock `river:"a,block,optional"`
		BlockB *childBlock `river:"b,block,optional"`
		BlockC *childBlock `river:"c,block,optional"`
		BlockD *childBlock `river:"d,block,optional"`
	}

	t.Run("Decodes enum blocks", func(t *testing.T) {
		type block struct {
			Value  int          `river:"value,attr"`
			Blocks []*enumBlock `river:"child,enum,optional"`
		}

		input := `some_block {
			value = 15

			child.a { attr = 1 }
		}`
		eval := vm.New(parseBlock(t, input))

		expect := block{
			Value: 15,
			Blocks: []*enumBlock{
				{BlockA: &childBlock{Attr: 1}},
			},
		}

		var actual block
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, expect, actual)
	})

	t.Run("Decodes multiple enum blocks", func(t *testing.T) {
		type block struct {
			Value  int          `river:"value,attr"`
			Blocks []*enumBlock `river:"child,enum,optional"`
		}

		input := `some_block {
			value = 15

			child.b { attr = 1 }
			child.a { attr = 2 }
			child.c { attr = 3 }
		}`
		eval := vm.New(parseBlock(t, input))

		expect := block{
			Value: 15,
			Blocks: []*enumBlock{
				{BlockB: &childBlock{Attr: 1}},
				{BlockA: &childBlock{Attr: 2}},
				{BlockC: &childBlock{Attr: 3}},
			},
		}

		var actual block
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, expect, actual)
	})

	t.Run("Decodes multiple enum blocks with repeating blocks", func(t *testing.T) {
		type block struct {
			Value  int          `river:"value,attr"`
			Blocks []*enumBlock `river:"child,enum,optional"`
		}

		input := `some_block {
			value = 15

			child.a { attr = 1 }
			child.b { attr = 2 }
			child.c { attr = 3 }
			child.a { attr = 4 }
		}`
		eval := vm.New(parseBlock(t, input))

		expect := block{
			Value: 15,
			Blocks: []*enumBlock{
				{BlockA: &childBlock{Attr: 1}},
				{BlockB: &childBlock{Attr: 2}},
				{BlockC: &childBlock{Attr: 3}},
				{BlockA: &childBlock{Attr: 4}},
			},
		}

		var actual block
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, expect, actual)
	})
}

func TestVM_Block_Label(t *testing.T) {
	t.Run("Decodes label into string field", func(t *testing.T) {
		type block struct {
			Label string `river:",label"`
		}

		input := `some_block "label_value_1" {}`
		eval := vm.New(parseBlock(t, input))

		var actual block
		require.NoError(t, eval.Evaluate(nil, &actual))
		require.Equal(t, "label_value_1", actual.Label)
	})

	t.Run("Struct must have label field if block is labeled", func(t *testing.T) {
		type block struct{}

		input := `some_block "label_value_2" {}`
		eval := vm.New(parseBlock(t, input))

		err := eval.Evaluate(nil, &block{})
		require.EqualError(t, err, `1:1: block "some_block" does not support specifying labels`)
	})

	t.Run("Block must have label if struct accepts label", func(t *testing.T) {
		type block struct {
			Label string `river:",label"`
		}

		input := `some_block {}`
		eval := vm.New(parseBlock(t, input))

		err := eval.Evaluate(nil, &block{})
		require.EqualError(t, err, `1:1: block "some_block" requires non-empty label`)
	})

	t.Run("Block must have non-empty label if struct accepts label", func(t *testing.T) {
		type block struct {
			Label string `river:",label"`
		}

		input := `some_block "" {}`
		eval := vm.New(parseBlock(t, input))

		err := eval.Evaluate(nil, &block{})
		require.EqualError(t, err, `1:1: block "some_block" requires non-empty label`)
	})
}

func TestVM_Block_Unmarshaler(t *testing.T) {
	type OuterBlock struct {
		FieldA   string  `river:"field_a,attr"`
		Settings Setting `river:"some.settings,block"`
	}

	input := `
		field_a = "foobar"
		some.settings {
			field_a = "fizzbuzz"
			field_b = "helloworld"
		}
	`

	file, err := parser.ParseFile(t.Name(), []byte(input))
	require.NoError(t, err)

	eval := vm.New(file)

	var actual OuterBlock
	require.NoError(t, eval.Evaluate(nil, &actual))
	require.True(t, actual.Settings.UnmarshalCalled, "UnmarshalRiver did not get invoked")
	require.True(t, actual.Settings.DefaultCalled, "SetToDefault did not get invoked")
	require.True(t, actual.Settings.ValidateCalled, "Validate did not get invoked")
}

func TestVM_Block_UnmarshalToMap(t *testing.T) {
	type OuterBlock struct {
		Settings map[string]interface{} `river:"some.settings,block"`
	}

	tt := []struct {
		name        string
		input       string
		expect      OuterBlock
		expectError string
	}{
		{
			name: "decodes successfully",
			input: `
				some.settings {
					field_a = 12345
					field_b = "helloworld"
				}
			`,
			expect: OuterBlock{
				Settings: map[string]interface{}{
					"field_a": 12345,
					"field_b": "helloworld",
				},
			},
		},
		{
			name: "rejects labeled blocks",
			input: `
				some.settings "foo" {
					field_a = 12345
				}
			`,
			expectError: `block "some.settings" requires non-empty label`,
		},

		{
			name: "rejects nested maps",
			input: `
				some.settings {
					inner_map {
						field_a = 12345
					}
				}
			`,
			expectError: "nested blocks not supported here",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			file, err := parser.ParseFile(t.Name(), []byte(tc.input))
			require.NoError(t, err)

			eval := vm.New(file)

			var actual OuterBlock
			err = eval.Evaluate(nil, &actual)

			if tc.expectError == "" {
				require.NoError(t, err)
				require.Equal(t, tc.expect, actual)
			} else {
				require.ErrorContains(t, err, tc.expectError)
			}
		})
	}
}

func TestVM_Block_UnmarshalToAny(t *testing.T) {
	type OuterBlock struct {
		Settings any `river:"some.settings,block"`
	}

	input := `
		some.settings {
			field_a = 12345
			field_b = "helloworld"
		}
	`

	file, err := parser.ParseFile(t.Name(), []byte(input))
	require.NoError(t, err)

	eval := vm.New(file)

	var actual OuterBlock
	require.NoError(t, eval.Evaluate(nil, &actual))

	expect := map[string]interface{}{
		"field_a": 12345,
		"field_b": "helloworld",
	}
	require.Equal(t, expect, actual.Settings)
}

type Setting struct {
	FieldA string `river:"field_a,attr"`
	FieldB string `river:"field_b,attr"`

	UnmarshalCalled bool
	DefaultCalled   bool
	ValidateCalled  bool
}

func (s *Setting) UnmarshalRiver(f func(interface{}) error) error {
	s.UnmarshalCalled = true
	return f((*settingUnmarshalTarget)(s))
}

type settingUnmarshalTarget Setting

func (s *settingUnmarshalTarget) SetToDefault() {
	s.DefaultCalled = true
}

func (s *settingUnmarshalTarget) Validate() error {
	s.ValidateCalled = true
	return nil
}

func parseBlock(t *testing.T, input string) *ast.BlockStmt {
	t.Helper()

	res, err := parser.ParseFile("", []byte(input))
	require.NoError(t, err)
	require.Len(t, res.Body, 1)

	stmt, ok := res.Body[0].(*ast.BlockStmt)
	require.True(t, ok, "Expected stmt to be a ast.BlockStmt, got %T", res.Body[0])
	return stmt
}
