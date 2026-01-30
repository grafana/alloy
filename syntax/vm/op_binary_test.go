package vm_test

import (
	"reflect"
	"testing"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/vm"
	"github.com/stretchr/testify/require"
)

func TestVM_Capsule_Conversion(t *testing.T) {
	scope := vm.NewScope(map[string]any{
		"string_val":      "hello",
		"non_secret_val":  alloytypes.OptionalSecret{IsSecret: false, Value: "world"},
		"secret_val":      alloytypes.OptionalSecret{IsSecret: true, Value: "secret"},
		"secret_type_val": alloytypes.Secret("welcome"),
	})

	tt := []struct {
		name        string
		input       string
		expect      any
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
			name:   "capsule (optionalsecret(true)) == capsule (optionalsecret(true))",
			input:  `secret_val == secret_val`,
			expect: bool(true),
		},
		{
			name:   "capsule (optionalsecret(false)) == capsule (optionalsecret(false))",
			input:  `non_secret_val == non_secret_val`,
			expect: bool(true),
		},
		{
			name:   "capsule (optionalsecret(false)) == capsule (optionalsecret(true))",
			input:  `non_secret_val == secret_val`,
			expect: bool(false),
		},
		{
			name:        "capsule (optionalsecret(true)) - string",
			input:       `secret_val - string_val`,
			expectError: "secret_val should be one of [number] for binop -, got capsule",
		},
		{
			name:        "string - capsule (optionalsecret(true))",
			input:       `string_val - secret_val`,
			expectError: "string_val should be one of [number] for binop -, got capsule",
		},
		{
			name:  "capsule (optionalsecret(true)) + string",
			input: `secret_val + string_val`,
			expect: alloytypes.OptionalSecret{
				Value: "secrethello", IsSecret: true,
			},
		},
		{
			name:   "capsule (optionalsecret(false)) + string",
			input:  `non_secret_val + string_val`,
			expect: string("worldhello"),
		},
		{
			name:   "capsule (optionalsecret(false)) + capsule (optionalsecret(false))",
			input:  `non_secret_val + non_secret_val`,
			expect: alloytypes.Secret("worldworld"),
		},
		{
			name:   "capsule (optionalsecret(true)) + capsule (optionalsecret(false))",
			input:  `secret_val + non_secret_val`,
			expect: alloytypes.Secret("secretworld"),
		},
		{
			name:   "capsule (optionalsecret(false)) + capsule (optionalsecret(true))",
			input:  `non_secret_val + secret_val`,
			expect: alloytypes.Secret("worldsecret"),
		},
		{
			name:   "capsule (optionalsecret(false)) + secret",
			input:  `non_secret_val + secret_type_val`,
			expect: alloytypes.Secret("worldwelcome"),
		},
		{
			name:   "capsule (secret) + string",
			input:  `secret_type_val + string_val`,
			expect: alloytypes.Secret("welcomehello"),
		},
		{
			name:   "capsule (secret) + capsule (optionalsecret(false))",
			input:  `secret_type_val + non_secret_val`,
			expect: alloytypes.Secret("welcomeworld"),
		},
		{
			name:   "capsule (secret) + capsule (optionalsecret(true))",
			input:  `secret_type_val + secret_val`,
			expect: alloytypes.Secret("welcomesecret"),
		},
		{
			name:   "capsule (secret) + capsule (secret)",
			input:  `secret_type_val + secret_type_val`,
			expect: alloytypes.Secret("welcomewelcome"),
		},
		{
			name:  "string + capsule (optionalsecret(true))",
			input: `string_val + secret_val`,
			expect: alloytypes.OptionalSecret{
				Value: "hellosecret", IsSecret: true,
			},
		},
		{
			name:   "string + capsule (optionalsecret(false))",
			input:  `string_val + non_secret_val`,
			expect: string("helloworld"),
		},
		{
			name:   "string + string",
			input:  `string_val + string_val`,
			expect: string("hellohello"),
		},
		{
			name:   "string + capsule (secret)",
			input:  `string_val + secret_type_val`,
			expect: alloytypes.Secret("hellowelcome"),
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
