package vm_test

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/grafana/river/parser"
	"github.com/grafana/river/vm"
	"github.com/stretchr/testify/require"
)

func BenchmarkExprs(b *testing.B) {
	// Shared scope across all tests below
	scope := &vm.Scope{
		Variables: map[string]interface{}{
			"foobar": int(42),
		},
	}

	tt := []struct {
		name   string
		input  string
		expect interface{}
	}{
		// Binops
		{"or", `false || true`, bool(true)},
		{"and", `true && false`, bool(false)},
		{"math/eq", `3 == 5`, bool(false)},
		{"math/neq", `3 != 5`, bool(true)},
		{"math/lt", `3 < 5`, bool(true)},
		{"math/lte", `3 <= 5`, bool(true)},
		{"math/gt", `3 > 5`, bool(false)},
		{"math/gte", `3 >= 5`, bool(false)},
		{"math/add", `3 + 5`, int(8)},
		{"math/sub", `3 - 5`, int(-2)},
		{"math/mul", `3 * 5`, int(15)},
		{"math/div", `3 / 5`, int(0)},
		{"math/mod", `5 % 3`, int(2)},
		{"math/pow", `3 ^ 5`, int(243)},
		{"binop chain", `3 + 5 * 2`, int(13)}, // Chain multiple binops

		// Identifier
		{"ident lookup", `foobar`, int(42)},

		// Arrays
		{"array", `[0, 1, 2]`, []int{0, 1, 2}},

		// Objects
		{"object to map", `{ a = 5, b = 10 }`, map[string]int{"a": 5, "b": 10}},
		{
			name: "object to struct",
			input: `{
					name = "John Doe", 
					age = 42,
			}`,
			expect: struct {
				Name    string `river:"name,attr"`
				Age     int    `river:"age,attr"`
				Country string `river:"country,attr,optional"`
			}{
				Name: "John Doe",
				Age:  42,
			},
		},

		// Access
		{"access", `{ a = 15 }.a`, int(15)},
		{"nested access", `{ a = { b = 12 } }.a.b`, int(12)},

		// Indexing
		{"index", `[0, 1, 2][1]`, int(1)},
		{"nested index", `[[1,2,3]][0][2]`, int(3)},

		// Paren
		{"paren", `(15)`, int(15)},

		// Unary
		{"unary not", `!true`, bool(false)},
		{"unary neg", `-15`, int(-15)},
		{"unary float", `-15.0`, float64(-15.0)},
		{"unary int64", fmt.Sprintf("%v", math.MaxInt64), math.MaxInt64},
		{"unary uint64", fmt.Sprintf("%v", uint64(math.MaxInt64)+1), uint64(math.MaxInt64) + 1},
		// math.MaxUint64 + 1 = 18446744073709551616
		{"unary float64 from overflowing uint", "18446744073709551616", float64(18446744073709551616)},
	}

	for _, tc := range tt {
		b.Run(tc.name, func(b *testing.B) {
			b.StopTimer()
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(b, err)

			eval := vm.New(expr)
			b.StartTimer()

			expectType := reflect.TypeOf(tc.expect)

			for i := 0; i < b.N; i++ {
				vPtr := reflect.New(expectType).Interface()
				_ = eval.Evaluate(scope, vPtr)
			}
		})
	}
}
