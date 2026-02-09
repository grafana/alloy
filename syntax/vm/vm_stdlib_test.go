package vm_test

import (
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/alloy/syntax/internal/value"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/vm"
	"github.com/stretchr/testify/require"
)

func TestVM_Stdlib(t *testing.T) {
	t.Setenv("TEST_VAR", "Hello!")

	tt := []struct {
		name   string
		input  string
		expect any
	}{
		// deprecated tests
		{"env", `env("TEST_VAR")`, string("Hello!")},
		{"concat", `concat([true, "foo"], [], [false, 1])`, []any{true, "foo", false, 1}},
		{"json_decode object", `json_decode("{\"foo\": \"bar\"}")`, map[string]any{"foo": "bar"}},
		{"yaml_decode object", "yaml_decode(`foo: bar`)", map[string]any{"foo": "bar"}},
		{"base64_decode", `base64_decode("Zm9vYmFyMTIzIT8kKiYoKSctPUB+")`, string(`foobar123!?$*&()'-=@~`)},

		{"sys.env", `sys.env("TEST_VAR")`, string("Hello!")},
		{"array.concat", `array.concat([true, "foo"], [], [false, 1])`, []any{true, "foo", false, 1}},
		{"encoding.from_json object", `encoding.from_json("{\"foo\": \"bar\"}")`, map[string]any{"foo": "bar"}},
		{"encoding.from_json array", `encoding.from_json("[0, 1, 2]")`, []any{float64(0), float64(1), float64(2)}},
		{"encoding.from_json nil field", `encoding.from_json("{\"foo\": null}")`, map[string]any{"foo": nil}},
		{"encoding.from_json nil array element", `encoding.from_json("[0, null]")`, []any{float64(0), nil}},
		{"encoding.from_yaml object", "encoding.from_yaml(`foo: bar`)", map[string]any{"foo": "bar"}},
		{"encoding.from_yaml array", "encoding.from_yaml(`[0, 1, 2]`)", []any{0, 1, 2}},
		{"encoding.from_yaml array float", "encoding.from_yaml(`[0.0, 1.0, 2.0]`)", []any{float64(0), float64(1), float64(2)}},
		{"encoding.from_yaml nil field", "encoding.from_yaml(`foo: null`)", map[string]any{"foo": nil}},
		{"encoding.from_yaml nil array element", `encoding.from_yaml("[0, null]")`, []any{0, nil}},
		{"encoding.from_base64", `encoding.from_base64("Zm9vYmFyMTIzIT8kKiYoKSctPUB+")`, string(`foobar123!?$*&()'-=@~`)},
		{"encoding.from_URLbase64", `encoding.from_URLbase64("c3RyaW5nMTIzIT8kKiYoKSctPUB-")`, string(`string123!?$*&()'-=@~`)},
		{"encoding.to_base64", `encoding.to_base64("string123!?$*&()'-=@~")`, string(`c3RyaW5nMTIzIT8kKiYoKSctPUB+`)},
		{"encoding.to_URLbase64", `encoding.to_URLbase64("string123!?$*&()'-=@~")`, string(`c3RyaW5nMTIzIT8kKiYoKSctPUB-`)},
		{"encoding.url_encode", `encoding.url_encode("string123!?$*&()'-=@~")`, string(`string123%21%3F%24%2A%26%28%29%27-%3D%40~`)},
		{"encoding.url_decode", `encoding.url_decode("string123%21%3F%24%2A%26%28%29%27-%3D%40~")`, string(`string123!?$*&()'-=@~`)},
		{
			"encoding.to_json object",
			`encoding.to_json({"modules"={"http_2xx"={"prober"="http","timeout"="5s","http"={"headers"={"Authorization"=sys.env("TEST_VAR")}}}}})`,
			string(`{"modules":{"http_2xx":{"http":{"headers":{"Authorization":"Hello!"}},"prober":"http","timeout":"5s"}}}`),
		},
		// Map tests
		{
			// Basic case. No conflicting key/val pairs.
			"array.combine_maps",
			`array.combine_maps([{"a" = "a1", "b" = "b1"}], [{"a" = "a1", "c" = "c1"}], ["a"])`,
			[]map[string]any{{"a": "a1", "b": "b1", "c": "c1"}},
		},
		{
			// The first array has 2 maps, each with the same key/val pairs.
			"array.combine_maps",
			`array.combine_maps([{"a" = "a1", "b" = "b1"}, {"a" = "a1", "b" = "b1"}], [{"a" = "a1", "c" = "c1"}], ["a"])`,
			[]map[string]any{{"a": "a1", "b": "b1", "c": "c1"}, {"a": "a1", "b": "b1", "c": "c1"}},
		},
		{
			// Non-unique merge criteria.
			"array.combine_maps",
			`array.combine_maps([{"pod" = "a", "lbl" = "q"}, {"pod" = "b", "lbl" = "q"}], [{"pod" = "c", "lbl" = "q"}, {"pod" = "d", "lbl" = "q"}], ["lbl"])`,
			[]map[string]any{{"lbl": "q", "pod": "c"}, {"lbl": "q", "pod": "d"}, {"lbl": "q", "pod": "c"}, {"lbl": "q", "pod": "d"}},
		},
		{
			// Basic case. Integer and string values.
			"array.combine_maps",
			`array.combine_maps([{"a" = 1, "b" = 2.2}], [{"a" = 1, "c" = "c1"}], ["a"])`,
			[]map[string]any{{"a": 1, "b": 2.2, "c": "c1"}},
		},
		{
			// The second map will override a value from the first.
			"array.combine_maps",
			`array.combine_maps([{"a" = 1, "b" = 2.2}], [{"a" = 1, "b" = "3.3"}], ["a"])`,
			[]map[string]any{{"a": 1, "b": "3.3"}},
		},
		{
			// Not enough matches for a join.
			"array.combine_maps",
			`array.combine_maps([{"a" = 1, "b" = 2.2}], [{"a" = 2, "b" = "3.3"}], ["a"])`,
			[]map[string]any{},
		},
		{
			// Not enough matches for a join, but all elements from the first array are passed through.
			"array.combine_maps",
			`array.combine_maps([{"a" = 1, "b" = 4.2, "c" = 5}, {"d" = "asdf"}], [{"a" = 2, "b" = "5.3"}], ["a"], true)`,
			[]map[string]any{{"a": 1, "b": 4.2, "c": 5}, {"d": "asdf"}},
		},
		{
			// Only one element from the first array matches, but all elements from the first array are passed through.
			"array.combine_maps",
			`array.combine_maps([{"a" = 1, "b" = 4.2, "c" = 5}, {"d" = "asdf"}, {"a" = 2, "b" = "1", "z" = "z1"}], [{"a" = 2, "b" = "5.3"}], ["a"], true)`,
			[]map[string]any{{"a": 1, "b": 4.2, "c": 5}, {"d": "asdf"}, {"a": 2, "z": "z1", "b": "5.3"}},
		},
		{
			// Not enough matches for a join.
			// The "a" value has differing types.
			"array.combine_maps",
			`array.combine_maps([{"a" = 1, "b" = 2.2}], [{"a" = "1", "b" = "3.3"}], ["a"])`,
			[]map[string]any{},
		},
		{
			// Basic case. Some values are arrays and maps.
			"array.combine_maps",
			`array.combine_maps([{"a" = 1, "b" = [1,2,3]}], [{"a" = 1, "c" = {"d" = {"e" = 10}}}], ["a"])`,
			[]map[string]any{{"a": 1, "b": []any{1, 2, 3}, "c": map[string]any{"d": map[string]any{"e": 10}}}},
		},
		{
			// Join key not present in ARG2
			"array.combine_maps",
			`array.combine_maps([{"a" = 1, "n" = 1.1}], [{"a" = 1, "n" = 2.1}, {"n" = 2.2}], ["a"])`,
			[]map[string]any{{"a": 1, "n": 2.1}},
		},
		{
			// Join key not present in ARG1
			"array.combine_maps",
			`array.combine_maps([{"a" = 1, "n" = 1.1}, {"n" = 1.2}], [{"a" = 1, "n" = 2.1}], ["a"])`,
			[]map[string]any{{"a": 1, "n": 2.1}},
		},
		{
			// Join with multiple keys
			"array.combine_maps",
			`array.combine_maps([{"a" = 1, "b" = 3, "n" = 1.1}], [{"a" = 1, "b" = 3, "n" = 2.1}], ["a", "b"])`,
			[]map[string]any{{"a": 1, "b": 3, "n": 2.1}},
		},
		{
			// Join with multiple keys
			// Some maps don't match all keys
			"array.combine_maps",
			`array.combine_maps([{"a" = 1, "n" = 1.1}, {"a" = 1, "b" = 3, "n" = 1.1}, {"b" = 3, "n" = 1.1}], [{"a" = 1, "n" = 2.3}, {"b" = 1, "n" = 2.3}, {"a" = 1, "b" = 3, "n" = 2.1}], ["a", "b"])`,
			[]map[string]any{{"a": 1, "b": 3, "n": 2.1}},
		},
		{
			// Join with multiple keys
			// No match because one key is missing
			"array.combine_maps",
			`array.combine_maps([{"a" = 1, "n" = 1.1}, {"a" = 1, "b" = 3, "n" = 1.1}, {"b" = 3, "n" = 1.1}], [{"a" = 1, "n" = 2.3}, {"b" = 1, "n" = 2.3}, {"a" = 1, "b" = 3, "n" = 2.1}], ["a", "b", "c"])`,
			[]map[string]any{},
		},
		{
			// Multi match ends up with len(ARG1) * len(ARG2) maps
			"array.combine_maps",
			`array.combine_maps([{"a" = 1, "n" = 1.1}, {"a" = 1, "n" = 1.2}, {"a" = 1, "n" = 1.3}], [{"a" = 1, "n" = 2.1}, {"a" = 1, "n" = 2.2}, {"a" = 1, "n" = 2.3}], ["a"])`,
			[]map[string]any{
				{"a": 1, "n": 2.1}, {"a": 1, "n": 2.2}, {"a": 1, "n": 2.3},
				{"a": 1, "n": 2.1}, {"a": 1, "n": 2.2}, {"a": 1, "n": 2.3},
				{"a": 1, "n": 2.1}, {"a": 1, "n": 2.2}, {"a": 1, "n": 2.3},
			},
		},
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

func TestVM_Stdlib_Errors(t *testing.T) {
	tt := []struct {
		name        string
		input       string
		expectedErr string
	}{
		// Map tests
		{
			// Error: invalid RHS type - string.
			"array.combine_maps",
			`array.combine_maps([{"a" = "a1", "b" = "b1"}], "a", ["a"])`,
			`"a" should be array, got string`,
		},
		{
			// Error: invalid RHS type - an array with strings.
			"array.combine_maps",
			`array.combine_maps([{"a" = "a1", "b" = "b1"}], ["a"], ["a"])`,
			`"a" should be object, got string`,
		},
		{
			"array.combine_maps",
			`array.combine_maps([{"a" = "a1", "b" = "b1"}], [{"a" = "a1", "c" = "b1"}], [])`,
			`combine_maps: merge conditions must not be empty`,
		},
		{
			"encoding.to_json",
			`encoding.to_json(12)`,
			`encoding.to_json jsonEncode only supports map`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(t, err)

			eval := vm.New(expr)

			rv := reflect.New(reflect.TypeOf([]map[string]any{}))
			err = eval.Evaluate(nil, rv.Interface())
			require.ErrorContains(t, err, tc.expectedErr)
		})
	}
}

func TestStdlibCoalesce(t *testing.T) {
	t.Setenv("TEST_VAR2", "Hello!")

	scope := vm.NewScope(map[string]any{
		"optionalSecretStr": alloytypes.OptionalSecret{Value: "bar"},
		"optionalSecretInt": alloytypes.OptionalSecret{Value: "123", IsSecret: false},
	})

	tt := []struct {
		name   string
		input  string
		expect any
	}{
		{"coalesce()", `coalesce()`, value.Null},
		{"coalesce(string)", `coalesce("Hello!")`, string("Hello!")},
		{"coalesce(string, string)", `coalesce(sys.env("TEST_VAR2"), "World!")`, string("Hello!")},
		{"(string, string) with fallback", `coalesce(sys.env("NON_DEFINED"), "World!")`, string("World!")},
		{"coalesce(list, list)", `coalesce([], ["fallback"])`, []string{"fallback"}},
		{"coalesce(list, list) with fallback", `coalesce(array.concat(["item"]), ["fallback"])`, []string{"item"}},
		{"coalesce(int, int, int)", `coalesce(0, 1, 2)`, 1},
		{"coalesce(bool, int, int)", `coalesce(false, 1, 2)`, 1},
		{"coalesce(bool, bool)", `coalesce(false, true)`, true},
		{"coalesce(list, bool)", `coalesce(encoding.from_json("[]"), true)`, true},
		{"coalesce(object, true) and return true", `coalesce(encoding.from_json("{}"), true)`, true},
		{"coalesce(object, false) and return false", `coalesce(encoding.from_json("{}"), false)`, false},
		{"coalesce(list, nil)", `coalesce([],null)`, value.Null},
		{"optional secret str first in coalesce", `coalesce(optionalSecretStr, 1)`, string("bar")},
		{"optional secret str second in coalesce", `coalesce("foo", optionalSecretStr)`, string("foo")},
		{"optional secret int first in coalesce", `coalesce(optionalSecretInt, 1)`, int(123)},
		{"optional secret int second in coalesce", `coalesce(1, optionalSecretInt)`, int(1)},
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

func TestStdlibJsonPath(t *testing.T) {
	tt := []struct {
		name   string
		input  string
		expect any
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
	scope := vm.NewScope(map[string]any{
		"secret":         alloytypes.Secret("foo"),
		"optionalSecret": alloytypes.OptionalSecret{Value: "bar"},
	})

	tt := []struct {
		name   string
		input  string
		expect any
	}{
		// deprecated tests
		{"deprecated secret to string", `nonsensitive(secret)`, string("foo")},

		{"secret to string", `convert.nonsensitive(secret)`, string("foo")},
		{"optional secret to string", `convert.nonsensitive(optionalSecret)`, string("bar")},
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
	scope := vm.NewScope(make(map[string]any))

	tt := []struct {
		name   string
		input  string
		expect any
	}{
		// deprecated tests
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

		{"string.to_lower", `string.to_lower("String")`, "string"},
		{"string.to_upper", `string.to_upper("string")`, "STRING"},
		{"string.trimspace", `string.trim_space("   string \n\n")`, "string"},
		{"string.trimspace+string.to_upper+string.trim", `string.to_lower(string.to_upper(string.trim_space("   String   ")))`, "string"},
		{"string.split", `string.split("/aaa/bbb/ccc/ddd", "/")`, []string{"", "aaa", "bbb", "ccc", "ddd"}},
		{"string.split+index", `string.split("/aaa/bbb/ccc/ddd", "/")[0]`, ""},
		{"string.join+split", `string.join(string.split("/aaa/bbb/ccc/ddd", "/"), "/")`, "/aaa/bbb/ccc/ddd"},
		{"string.join", `string.join(["foo", "bar", "baz"], ", ")`, "foo, bar, baz"},
		{"string.join w/ int", `string.join([0, 0, 1], ", ")`, "0, 0, 1"},
		{"string.format", `string.format("Hello %s", "World")`, "Hello World"},
		{"string.format+int", `string.format("%#v", 1)`, "1"},
		{"string.format+bool", `string.format("%#v", true)`, "true"},
		{"string.format+quote", `string.format("%q", "hello")`, `"hello"`},
		{"string.replace", `string.replace("Hello World", " World", "!")`, "Hello!"},
		{"string.trim", `string.trim("?!hello?!", "!?")`, "hello"},
		{"string.trim2", `string.trim("   hello! world.!  ", "! ")`, "hello! world."},
		{"string.trim_prefix", `string.trim_prefix("helloworld", "hello")`, "world"},
		{"string.trim_suffix", `string.trim_suffix("helloworld", "world")`, "hello"},
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

func TestStdlibFileFunc(t *testing.T) {
	tt := []struct {
		name   string
		input  string
		expect any
	}{
		{"file.path_join", `file.path_join("this/is", "a/path")`, filepath.Join("this", "is", "a", "path")},
		{"file.path_join empty", `file.path_join()`, ""},
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

func BenchmarkConcat(b *testing.B) {
	// There's a bit of setup work to do here: we want to create a scope holding
	// a slice of the Person type, which has a fair amount of data in it.
	//
	// We then want to pass it through concat.
	//
	// If the code path is fully optimized, there will be no intermediate
	// translations to interface{}.
	type Person struct {
		Name  string            `alloy:"name,attr"`
		Attrs map[string]string `alloy:"attrs,attr"`
	}
	type Body struct {
		Values []Person `alloy:"values,attr"`
	}

	in := `values = array.concat(values_ref)`
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
	scope := vm.NewScope(map[string]any{
		"values_ref": valuesRef,
	})

	// Reset timer before running the actual test
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		var b Body
		_ = eval.Evaluate(scope, &b)
	}
}

func TestStdlibGroupBy(t *testing.T) {
	tt := []struct {
		name   string
		input  string
		expect any
	}{
		{
			"basic grouping",
			`array.group_by([{"type" = "fruit", "name" = "apple"}, {"type" = "fruit", "name" = "banana"}, {"type" = "vegetable", "name" = "carrot"}], "type", false)`,
			[]map[string]any{
				{"type": "fruit", "items": []any{
					map[string]any{"type": "fruit", "name": "apple"},
					map[string]any{"type": "fruit", "name": "banana"},
				}},
				{"type": "vegetable", "items": []any{
					map[string]any{"type": "vegetable", "name": "carrot"},
				}},
			},
		},
		{
			"drop missing keys",
			`array.group_by([{"name" = "alice", "age" = "20"}, {"name" = "bob"}, {"name" = "charlie", "age" = "30"}], "age", true)`,
			[]map[string]any{
				{"age": "20", "items": []any{
					map[string]any{"name": "alice", "age": "20"},
				}},
				{"age": "30", "items": []any{
					map[string]any{"name": "charlie", "age": "30"},
				}},
			},
		},
		{
			"keep missing keys",
			`array.group_by([{"name" = "alice", "age" = "20"}, {"name" = "bob"}, {"name" = "charlie", "age" = "30"}], "age", false)`,
			[]map[string]any{
				{"age": "20", "items": []any{
					map[string]any{"name": "alice", "age": "20"},
				}},
				{"age": "30", "items": []any{
					map[string]any{"name": "charlie", "age": "30"},
				}},
				{"age": "", "items": []any{
					map[string]any{"name": "bob"},
				}},
			},
		},
		{
			"empty array",
			`array.group_by([], "age", false)`,
			[]map[string]any{},
		},
		{
			"all items missing key",
			`array.group_by([{"name" = "alice"}, {"name" = "bob"}], "age", false)`,
			[]map[string]any{
				{"age": "", "items": []any{
					map[string]any{"name": "alice"},
					map[string]any{"name": "bob"},
				}},
			},
		},
		{
			"key refers to a nested object",
			`array.group_by([{"name" = "alice", "age" = 20, "address" = {"city" = "New York", "state" = "NY"}}], "address.city", false)`,
			// The key should be present at the top level of the object. In this case, the group_by assumes that the key is missing.
			[]map[string]any{
				{"address.city": "", "items": []any{
					map[string]any{"name": "alice", "age": 20, "address": map[string]any{"city": "New York", "state": "NY"}},
				}},
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(t, err)

			eval := vm.New(expr)

			rv := reflect.New(reflect.TypeOf(tc.expect))
			require.NoError(t, eval.Evaluate(nil, rv.Interface()))
			result := rv.Elem().Interface().([]map[string]any)
			expected := tc.expect.([]map[string]any)
			require.ElementsMatch(t, expected, result, "groups should match without order")
		})
	}
}

func TestStdlibGroupBy_Errors(t *testing.T) {
	tt := []struct {
		name        string
		input       string
		expectedErr string
	}{
		{
			"wrong number of arguments",
			`array.group_by([{"name" = "alice"}], "age")`,
			`group_by: expected 3 arguments, got 2`,
		},
		{
			"first argument not array",
			`array.group_by("not an array", "age", false)`,
			`"not an array" should be array, got string`,
		},
		{
			"second argument not string",
			`array.group_by([{"name" = "alice"}], 123, false)`,
			`123 should be string, got number`,
		},
		{
			"third argument not bool",
			`array.group_by([{"name" = "alice"}], "age", "not a bool")`,
			`"not a bool" should be bool, got string`,
		},
		{
			"array element not object",
			`array.group_by(["not an object"], "age", false)`,
			`"not an object" should be object, got string`,
		},
		{
			"key value not string",
			`array.group_by([{"name" = "alice", "age" = 20}], "age", false)`,
			`20 should be string, got number`,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(t, err)

			eval := vm.New(expr)

			rv := reflect.New(reflect.TypeOf([]map[string]any{}))
			err = eval.Evaluate(nil, rv.Interface())
			require.ErrorContains(t, err, tc.expectedErr)
		})
	}
}
