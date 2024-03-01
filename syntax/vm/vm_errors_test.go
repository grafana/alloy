package vm_test

import (
	"testing"

	"github.com/grafana/river/parser"
	"github.com/grafana/river/vm"
	"github.com/stretchr/testify/require"
)

func TestVM_ExprErrors(t *testing.T) {
	type Target struct {
		Key struct {
			Object struct {
				Field1 []int `river:"field1,attr"`
			} `river:"object,attr"`
		} `river:"key,attr"`
	}

	tt := []struct {
		name   string
		input  string
		into   interface{}
		scope  *vm.Scope
		expect string
	}{
		{
			name:   "basic wrong type",
			input:  `key = true`,
			into:   &Target{},
			expect: "test:1:7: true should be object, got bool",
		},
		{
			name: "deeply nested literal",
			input: `
				key = {
					object = {
						field1 = [15, 30, "Hello, world!"],
					},
				}
			`,
			into:   &Target{},
			expect: `test:4:25: "Hello, world!" should be number, got string`,
		},
		{
			name:  "deeply nested indirect",
			input: `key = key_value`,
			into:  &Target{},
			scope: &vm.Scope{
				Variables: map[string]interface{}{
					"key_value": map[string]interface{}{
						"object": map[string]interface{}{
							"field1": []interface{}{15, 30, "Hello, world!"},
						},
					},
				},
			},
			expect: `test:1:7: key_value.object.field1[2] should be number, got string`,
		},
		{
			name:  "complex expr",
			input: `key = [0, 1, 2]`,
			into: &struct {
				Key string `river:"key,attr"`
			}{},
			expect: `test:1:7: [0, 1, 2] should be string, got array`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			res, err := parser.ParseFile("test", []byte(tc.input))
			require.NoError(t, err)

			eval := vm.New(res)
			err = eval.Evaluate(tc.scope, tc.into)
			require.EqualError(t, err, tc.expect)
		})
	}
}
