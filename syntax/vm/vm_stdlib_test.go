package vm_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/grafana/river/internal/value"
	"github.com/grafana/river/parser"
	"github.com/grafana/river/rivertypes"
	"github.com/grafana/river/vm"
	"github.com/stretchr/testify/require"
)

func TestVM_Stdlib(t *testing.T) {
	t.Setenv("TEST_VAR", "Hello!")

	tt := []struct {
		name   string
		input  string
		expect interface{}
	}{
		{"env", `env("TEST_VAR")`, string("Hello!")},
		{"concat", `concat([true, "foo"], [], [false, 1])`, []interface{}{true, "foo", false, 1}},
		{"json_decode object", `json_decode("{\"foo\": \"bar\"}")`, map[string]interface{}{"foo": "bar"}},
		{"json_decode array", `json_decode("[0, 1, 2]")`, []interface{}{float64(0), float64(1), float64(2)}},
		{"json_decode nil field", `json_decode("{\"foo\": null}")`, map[string]interface{}{"foo": nil}},
		{"json_decode nil array element", `json_decode("[0, null]")`, []interface{}{float64(0), nil}},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(t, err)

			eval := vm.New(expr)

			rv := reflect.New(reflect.TypeOf(tc.expect))
			require.NoError(t, eval.Evaluate(nil, rv.Interface()))
			require.Equal(t, tc.expect, rv.Elem().Interface())
		})
	}
}

func TestStdlibCoalesce(t *testing.T) {
	t.Setenv("TEST_VAR2", "Hello!")

	tt := []struct {
		name   string
		input  string
		expect interface{}
	}{
		{"coalesce()", `coalesce()`, value.Null},
		{"coalesce(string)", `coalesce("Hello!")`, string("Hello!")},
		{"coalesce(string, string)", `coalesce(env("TEST_VAR2"), "World!")`, string("Hello!")},
		{"(string, string) with fallback", `coalesce(env("NON_DEFINED"), "World!")`, string("World!")},
		{"coalesce(list, list)", `coalesce([], ["fallback"])`, []string{"fallback"}},
		{"coalesce(list, list) with fallback", `coalesce(concat(["item"]), ["fallback"])`, []string{"item"}},
		{"coalesce(int, int, int)", `coalesce(0, 1, 2)`, 1},
		{"coalesce(bool, int, int)", `coalesce(false, 1, 2)`, 1},
		{"coalesce(bool, bool)", `coalesce(false, true)`, true},
		{"coalesce(list, bool)", `coalesce(json_decode("[]"), true)`, true},
		{"coalesce(object, true) and return true", `coalesce(json_decode("{}"), true)`, true},
		{"coalesce(object, false) and return false", `coalesce(json_decode("{}"), false)`, false},
		{"coalesce(list, nil)", `coalesce([],null)`, value.Null},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(t, err)

			eval := vm.New(expr)

			rv := reflect.New(reflect.TypeOf(tc.expect))
			require.NoError(t, eval.Evaluate(nil, rv.Interface()))
			require.Equal(t, tc.expect, rv.Elem().Interface())
		})
	}
}

func TestStdlibJsonPath(t *testing.T) {
	tt := []struct {
		name   string
		input  string
		expect interface{}
	}{
		{"json_path with simple json", `json_path("{\"a\": \"b\"}", ".a")`, []string{"b"}},
		{"json_path with simple json without results", `json_path("{\"a\": \"b\"}", ".nonexists")`, []string{}},
		{"json_path with json array", `json_path("[{\"name\": \"Department\",\"value\": \"IT\"},{\"name\":\"ReferenceNumber\",\"value\":\"123456\"},{\"name\":\"TestStatus\",\"value\":\"Pending\"}]", "[?(@.name == \"Department\")].value")`, []string{"IT"}},
		{"json_path with simple json and return first", `json_path("{\"a\": \"b\"}", ".a")[0]`, "b"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(t, err)

			eval := vm.New(expr)

			rv := reflect.New(reflect.TypeOf(tc.expect))
			require.NoError(t, eval.Evaluate(nil, rv.Interface()))
			require.Equal(t, tc.expect, rv.Elem().Interface())
		})
	}
}

func TestStdlib_Nonsensitive(t *testing.T) {
	scope := &vm.Scope{
		Variables: map[string]any{
			"secret":         rivertypes.Secret("foo"),
			"optionalSecret": rivertypes.OptionalSecret{Value: "bar"},
		},
	}

	tt := []struct {
		name   string
		input  string
		expect interface{}
	}{
		{"secret to string", `nonsensitive(secret)`, string("foo")},
		{"optional secret to string", `nonsensitive(optionalSecret)`, string("bar")},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(t, err)

			eval := vm.New(expr)

			rv := reflect.New(reflect.TypeOf(tc.expect))
			require.NoError(t, eval.Evaluate(scope, rv.Interface()))
			require.Equal(t, tc.expect, rv.Elem().Interface())
		})
	}
}
func TestStdlib_StringFunc(t *testing.T) {
	scope := &vm.Scope{
		Variables: map[string]any{},
	}

	tt := []struct {
		name   string
		input  string
		expect interface{}
	}{
		{"to_lower", `to_lower("String")`, "string"},
		{"to_upper", `to_upper("string")`, "STRING"},
		{"trimspace", `trim_space("   string \n\n")`, "string"},
		{"trimspace+to_upper+trim", `to_lower(to_upper(trim_space("   String   ")))`, "string"},
		{"split", `split("/aaa/bbb/ccc/ddd", "/")`, []string{"", "aaa", "bbb", "ccc", "ddd"}},
		{"split+index", `split("/aaa/bbb/ccc/ddd", "/")[0]`, ""},
		{"join+split", `join(split("/aaa/bbb/ccc/ddd", "/"), "/")`, "/aaa/bbb/ccc/ddd"},
		{"join", `join(["foo", "bar", "baz"], ", ")`, "foo, bar, baz"},
		{"join w/ int", `join([0, 0, 1], ", ")`, "0, 0, 1"},
		{"format", `format("Hello %s", "World")`, "Hello World"},
		{"format+int", `format("%#v", 1)`, "1"},
		{"format+bool", `format("%#v", true)`, "true"},
		{"format+quote", `format("%q", "hello")`, `"hello"`},
		{"replace", `replace("Hello World", " World", "!")`, "Hello!"},
		{"trim", `trim("?!hello?!", "!?")`, "hello"},
		{"trim2", `trim("   hello! world.!  ", "! ")`, "hello! world."},
		{"trim_prefix", `trim_prefix("helloworld", "hello")`, "world"},
		{"trim_suffix", `trim_suffix("helloworld", "world")`, "hello"},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(t, err)

			eval := vm.New(expr)

			rv := reflect.New(reflect.TypeOf(tc.expect))
			require.NoError(t, eval.Evaluate(scope, rv.Interface()))
			require.Equal(t, tc.expect, rv.Elem().Interface())
		})
	}
}

func BenchmarkConcat(b *testing.B) {
	// There's a bit of setup work to do here: we want to create a scope holding
	// a slice of the Person type, which has a fair amount of data in it.
	//
	// We then want to pass it through concat.
	//
	// If the code path is fully optimized, there will be no intermediate
	// translations to interface{}.
	type Person struct {
		Name  string            `river:"name,attr"`
		Attrs map[string]string `river:"attrs,attr"`
	}
	type Body struct {
		Values []Person `river:"values,attr"`
	}

	in := `values = concat(values_ref)`
	f, err := parser.ParseFile("", []byte(in))
	require.NoError(b, err)

	eval := vm.New(f)

	valuesRef := make([]Person, 0, 20)
	for i := 0; i < 20; i++ {
		data := make(map[string]string, 20)
		for j := 0; j < 20; j++ {
			var (
				key   = fmt.Sprintf("key_%d", i+1)
				value = fmt.Sprintf("value_%d", i+1)
			)
			data[key] = value
		}
		valuesRef = append(valuesRef, Person{
			Name:  "Test Person",
			Attrs: data,
		})
	}
	scope := &vm.Scope{
		Variables: map[string]interface{}{
			"values_ref": valuesRef,
		},
	}

	// Reset timer before running the actual test
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var b Body
		_ = eval.Evaluate(scope, &b)
	}
}
