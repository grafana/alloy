package vm_test

import (
	"reflect"
	"testing"

	"github.com/grafana/river/parser"
	"github.com/grafana/river/rivertypes"
	"github.com/grafana/river/vm"
	"github.com/stretchr/testify/require"
)

func TestVM_OptionalSecret_Conversion(t *testing.T) {
	scope := &vm.Scope{
		Variables: map[string]any{
			"string_val":     "hello",
			"non_secret_val": rivertypes.OptionalSecret{IsSecret: false, Value: "world"},
			"secret_val":     rivertypes.OptionalSecret{IsSecret: true, Value: "secret"},
		},
	}

	tt := []struct {
		name        string
		input       string
		expect      interface{}
		expectError string
	}{
		{
			name:   "string + capsule",
			input:  `string_val + non_secret_val`,
			expect: string("helloworld"),
		},
		{
			name:   "capsule + string",
			input:  `non_secret_val + string_val`,
			expect: string("worldhello"),
		},
		{
			name:   "string == capsule",
			input:  `"world" == non_secret_val`,
			expect: bool(true),
		},
		{
			name:   "capsule == string",
			input:  `non_secret_val == "world"`,
			expect: bool(true),
		},
		{
			name:   "capsule (secret) == capsule (secret)",
			input:  `secret_val == secret_val`,
			expect: bool(true),
		},
		{
			name:   "capsule (non secret) == capsule (non secret)",
			input:  `non_secret_val == non_secret_val`,
			expect: bool(true),
		},
		{
			name:   "capsule (non secret) == capsule (secret)",
			input:  `non_secret_val == secret_val`,
			expect: bool(false),
		},
		{
			name:        "secret + string",
			input:       `secret_val + string_val`,
			expectError: "secret_val should be one of [number string] for binop +",
		},
		{
			name:        "string + secret",
			input:       `string_val + secret_val`,
			expectError: "secret_val should be one of [number string] for binop +",
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(t, err)

			expectTy := reflect.TypeOf(tc.expect)
			if expectTy == nil {
				expectTy = reflect.TypeOf((*any)(nil)).Elem()
			}
			rv := reflect.New(expectTy)

			if err := vm.New(expr).Evaluate(scope, rv.Interface()); tc.expectError == "" {
				require.NoError(t, err)
				require.Equal(t, tc.expect, rv.Elem().Interface())
			} else {
				require.ErrorContains(t, err, tc.expectError)
			}
		})
	}
}
