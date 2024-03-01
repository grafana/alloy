package vm_test

import (
	"reflect"
	"strings"
	"testing"
	"unicode"

	"github.com/grafana/river/parser"
	"github.com/grafana/river/scanner"
	"github.com/grafana/river/token"
	"github.com/grafana/river/vm"
	"github.com/stretchr/testify/require"
)

func TestVM_Evaluate_Literals(t *testing.T) {
	tt := map[string]struct {
		input  string
		expect interface{}
	}{
		"number to int":     {`12`, int(12)},
		"number to int8":    {`13`, int8(13)},
		"number to int16":   {`14`, int16(14)},
		"number to int32":   {`15`, int32(15)},
		"number to int64":   {`16`, int64(16)},
		"number to uint":    {`17`, uint(17)},
		"number to uint8":   {`18`, uint8(18)},
		"number to uint16":  {`19`, uint16(19)},
		"number to uint32":  {`20`, uint32(20)},
		"number to uint64":  {`21`, uint64(21)},
		"number to float32": {`22`, float32(22)},
		"number to float64": {`23`, float64(23)},
		"number to string":  {`24`, string("24")},

		"float to float32": {`3.2`, float32(3.2)},
		"float to float64": {`3.5`, float64(3.5)},
		"float to string":  {`3.9`, string("3.9")},

		"float with dot to float32": {`.2`, float32(0.2)},
		"float with dot to float64": {`.5`, float64(0.5)},
		"float with dot to string":  {`.9`, string("0.9")},

		"string to string":  {`"Hello, world!"`, string("Hello, world!")},
		"string to int":     {`"12"`, int(12)},
		"string to float64": {`"12"`, float64(12)},
	}

	for name, tc := range tt {
		t.Run(name, func(t *testing.T) {
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(t, err)

			eval := vm.New(expr)

			vPtr := reflect.New(reflect.TypeOf(tc.expect)).Interface()
			require.NoError(t, eval.Evaluate(nil, vPtr))

			actual := reflect.ValueOf(vPtr).Elem().Interface()
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestVM_Evaluate(t *testing.T) {
	// Shared scope across all tests below
	scope := &vm.Scope{
		Variables: map[string]interface{}{
			"foobar": int(42),
		},
	}

	tt := []struct {
		input  string
		expect interface{}
	}{
		// Binops
		{`true || false`, bool(true)},
		{`false || false`, bool(false)},
		{`true && false`, bool(false)},
		{`true && true`, bool(true)},
		{`3 == 5`, bool(false)},
		{`3 == 3`, bool(true)},
		{`3 != 5`, bool(true)},
		{`3 < 5`, bool(true)},
		{`3 <= 5`, bool(true)},
		{`3 > 5`, bool(false)},
		{`3 >= 5`, bool(false)},
		{`3 + 5`, int(8)},
		{`3 - 5`, int(-2)},
		{`3 * 5`, int(15)},
		{`3.0 / 5.0`, float64(0.6)},
		{`5 % 3`, int(2)},
		{`3 ^ 5`, int(243)},
		{`3 + 5 * 2`, int(13)}, // Chain multiple binops
		{`42.0^-2`, float64(0.0005668934240362812)},

		// Identifier
		{`foobar`, int(42)},

		// Arrays
		{`[]`, []int{}},
		{`[0, 1, 2]`, []int{0, 1, 2}},
		{`[true, false]`, []bool{true, false}},

		// Objects
		{`{ a = 5, b = 10 }`, map[string]int{"a": 5, "b": 10}},
		{
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
		{`{ a = 15 }.a`, int(15)},
		{`{ a = { b = 12 } }.a.b`, int(12)},
		{`{}["foo"]`, nil},

		// Indexing
		{`[0, 1, 2][1]`, int(1)},
		{`[[1,2,3]][0][2]`, int(3)},
		{`[true, false][0]`, bool(true)},

		// Paren
		{`(15)`, int(15)},

		// Unary
		{`!true`, bool(false)},
		{`!false`, bool(true)},
		{`-15`, int(-15)},
	}

	for _, tc := range tt {
		name := trimWhitespace(tc.input)

		t.Run(name, func(t *testing.T) {
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(t, err)

			eval := vm.New(expr)

			var vPtr any
			if tc.expect != nil {
				vPtr = reflect.New(reflect.TypeOf(tc.expect)).Interface()
			} else {
				// Create a new any pointer.
				vPtr = reflect.New(reflect.TypeOf((*any)(nil)).Elem()).Interface()
			}

			require.NoError(t, eval.Evaluate(scope, vPtr))

			actual := reflect.ValueOf(vPtr).Elem().Interface()
			require.Equal(t, tc.expect, actual)
		})
	}
}

func TestVM_Evaluate_Null(t *testing.T) {
	expr, err := parser.ParseExpression("null")
	require.NoError(t, err)

	eval := vm.New(expr)

	var v interface{}
	require.NoError(t, eval.Evaluate(nil, &v))
	require.Nil(t, v)
}

func TestVM_Evaluate_IdentifierExpr(t *testing.T) {
	t.Run("Valid lookup", func(t *testing.T) {
		scope := &vm.Scope{
			Variables: map[string]interface{}{
				"foobar": 15,
			},
		}

		expr, err := parser.ParseExpression(`foobar`)
		require.NoError(t, err)

		eval := vm.New(expr)

		var actual int
		require.NoError(t, eval.Evaluate(scope, &actual))
		require.Equal(t, 15, actual)
	})

	t.Run("Invalid lookup", func(t *testing.T) {
		expr, err := parser.ParseExpression(`foobar`)
		require.NoError(t, err)

		eval := vm.New(expr)

		var v interface{}
		err = eval.Evaluate(nil, &v)
		require.EqualError(t, err, `1:1: identifier "foobar" does not exist`)
	})
}

func TestVM_Evaluate_AccessExpr(t *testing.T) {
	t.Run("Lookup optional field", func(t *testing.T) {
		type Person struct {
			Name string `river:"name,attr,optional"`
		}

		scope := &vm.Scope{
			Variables: map[string]interface{}{
				"person": Person{},
			},
		}

		expr, err := parser.ParseExpression(`person.name`)
		require.NoError(t, err)

		eval := vm.New(expr)

		var actual string
		require.NoError(t, eval.Evaluate(scope, &actual))
		require.Equal(t, "", actual)
	})

	t.Run("Invalid lookup 1", func(t *testing.T) {
		expr, err := parser.ParseExpression(`{ a = 15 }.b`)
		require.NoError(t, err)

		eval := vm.New(expr)

		var v interface{}
		err = eval.Evaluate(nil, &v)
		require.EqualError(t, err, `1:12: field "b" does not exist`)
	})

	t.Run("Invalid lookup 2", func(t *testing.T) {
		_, err := parser.ParseExpression(`{ a = 15 }.7`)
		require.EqualError(t, err, `1:11: expected TERMINATOR, got FLOAT`)
	})

	t.Run("Invalid lookup 3", func(t *testing.T) {
		_, err := parser.ParseExpression(`{ a = { b = 12 }.7 }.a.b`)
		require.EqualError(t, err, `1:17: missing ',' in field list`)
	})

	t.Run("Invalid lookup 4", func(t *testing.T) {
		_, err := parser.ParseExpression(`{ a = { b = 12 } }.a.b.7`)
		require.EqualError(t, err, `1:23: expected TERMINATOR, got FLOAT`)
	})
}

func trimWhitespace(in string) string {
	f := token.NewFile("")

	s := scanner.New(f, []byte(in), nil, 0)

	var out strings.Builder

	for {
		_, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		}

		if lit != "" {
			_, _ = out.WriteString(lit)
		} else {
			_, _ = out.WriteString(tok.String())
		}
	}

	return strings.TrimFunc(out.String(), unicode.IsSpace)
}
