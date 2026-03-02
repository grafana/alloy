package discovery

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/Masterminds/goutils"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/equality"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/token/builder"
	"github.com/grafana/alloy/syntax/vm"
)

func TestUsingTargetCapsule(t *testing.T) {
	type testCase struct {
		name                  string
		inputTarget           map[string]string
		expression            string
		decodeInto            any
		expectedDecodedString string
		expectedEvalError     string
	}

	testCases := []testCase{
		{
			name:                  "target to map of string -> string",
			inputTarget:           map[string]string{"a1a": "beachfront avenue", "ice": "ice"},
			expression:            "t",
			decodeInto:            map[string]string{},
			expectedDecodedString: `{"a1a"="beachfront avenue", "ice"="ice"}`,
		},
		{
			name:                  "target to map of string -> any",
			inputTarget:           map[string]string{"a1a": "beachfront avenue", "ice": "ice"},
			expression:            "t",
			decodeInto:            map[string]any{},
			expectedDecodedString: `{"a1a"="beachfront avenue", "ice"="ice"}`,
		},
		{
			name:                  "target to map of any -> any",
			inputTarget:           map[string]string{"a1a": "beachfront avenue", "ice": "ice"},
			expression:            "t",
			decodeInto:            map[any]any{},
			expectedDecodedString: `{"a1a"="beachfront avenue", "ice"="ice"}`,
		},
		{
			name:                  "target to map of string -> syntax.Value",
			inputTarget:           map[string]string{"a1a": "beachfront avenue"},
			expression:            "t",
			decodeInto:            map[string]syntax.Value{},
			expectedDecodedString: `{"a1a"="beachfront avenue"}`,
		},
		{
			name:                  "target indexing a string value",
			inputTarget:           map[string]string{"a1a": "beachfront avenue", "hip": "hop"},
			expression:            `t["hip"]`,
			decodeInto:            "",
			expectedDecodedString: `hop`,
		},
		{
			name:                  "target indexing a non-existing string value",
			inputTarget:           map[string]string{"a1a": "beachfront avenue", "hip": "hop"},
			expression:            `t["boom"]`,
			decodeInto:            "",
			expectedDecodedString: "<nil>",
		},
		{
			name:              "target indexing a value like an object field",
			inputTarget:       map[string]string{"a1a": "beachfront avenue", "hip": "hop"},
			expression:        `t.boom`,
			decodeInto:        "",
			expectedEvalError: `field "boom" does not exist`,
		},
		{
			name:                  "targets passed to concat",
			inputTarget:           map[string]string{"boom": "bap", "hip": "hop"},
			expression:            `array.concat([t], [t])`,
			decodeInto:            []Target{},
			expectedDecodedString: `[{"boom"="bap", "hip"="hop"} {"boom"="bap", "hip"="hop"}]`,
		},
		{
			name:                  "coalesce an empty target",
			inputTarget:           map[string]string{},
			expression:            `coalesce(t, [], t, {}, t, 123, t)`,
			decodeInto:            []Target{},
			expectedDecodedString: `123`,
		},
		{
			name:                  "coalesce a non-empty target",
			inputTarget:           map[string]string{"big": "bang"},
			expression:            `coalesce([], {}, "", t, 321, [])`,
			decodeInto:            []Target{},
			expectedDecodedString: `{"big"="bang"}`,
		},
		{
			name:                  "array.combine_maps with targets",
			inputTarget:           map[string]string{"a": "a1", "b": "b1"},
			expression:            `array.combine_maps([t, t], [{"a" = "a1", "c" = "c1"}, {"a" = "a2", "c" = "c2"}], ["a"])`,
			decodeInto:            []Target{},
			expectedDecodedString: `[map[a:a1 b:b1 c:c1] map[a:a1 b:b1 c:c1]]`,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			target := NewTargetFromMap(tc.inputTarget)
			scope := vm.NewScope(map[string]any{"t": target})
			expr, err := parser.ParseExpression(tc.expression)
			require.NoError(t, err)
			eval := vm.New(expr)
			evalError := eval.Evaluate(scope, &tc.decodeInto)
			if tc.expectedEvalError != "" {
				require.ErrorContains(t, evalError, tc.expectedEvalError)
			} else {
				require.NoError(t, evalError)
			}
			require.Equal(t, tc.expectedDecodedString, fmt.Sprintf("%v", tc.decodeInto))
		})
	}
}

func TestNestedIndexing(t *testing.T) {
	targets := []Target{
		NewTargetFromMap(map[string]string{"foo": "bar", "boom": "bap"}),
		NewTargetFromMap(map[string]string{"hip": "hop", "dont": "stop"}),
	}
	scope := vm.NewScope(map[string]any{"targets": targets})

	expr, err := parser.ParseExpression(`targets[1]["dont"]`)
	require.NoError(t, err)
	eval := vm.New(expr)
	actual := ""
	err = eval.Evaluate(scope, &actual)
	require.NoError(t, err)
	require.Equal(t, "stop", actual)

	expr, err = parser.ParseExpression(`targets[0].boom`)
	require.NoError(t, err)
	eval = vm.New(expr)
	actual = ""
	err = eval.Evaluate(scope, &actual)
	require.NoError(t, err)
	require.Equal(t, "bap", actual)
}

func TestDecodeMap(t *testing.T) {
	type testCase struct {
		name     string
		input    string
		expected map[string]string
	}

	tests := []testCase{
		{
			name:     "empty",
			input:    `{}`,
			expected: map[string]string{},
		},
		{
			name:     "simple decode",
			input:    `{ "a" = "5", "b" = "10" }`,
			expected: map[string]string{"a": "5", "b": "10"},
		},
		{
			name:     "decode no quotes on keys",
			input:    `{ a = "5", b = "10" }`,
			expected: map[string]string{"a": "5", "b": "10"},
		},
		{
			name:     "decode no quotes",
			input:    `{ a = 5, b = 10 }`,
			expected: map[string]string{"a": "5", "b": "10"},
		},
		{
			name:     "decode mixed quoting",
			input:    `{ a = "5", "b" = "10", "c" = 15, d = 20 }`,
			expected: map[string]string{"a": "5", "b": "10", "c": "15", "d": "20"},
		},
		{
			name:     "decode different order",
			input:    `{ "b" = "10", "a" = "5" }`,
			expected: map[string]string{"a": "5", "b": "10"},
		},
		{
			name:     "decode with string concat",
			input:    `{ "b" = "1"+"0", "a" = "5" }`,
			expected: map[string]string{"a": "5", "b": "10"},
		},
		{
			name:     "decode with std function",
			input:    `{ "b" = string.format("%x", 31337), "a" = string.join(["2", "0"], ".") }`,
			expected: map[string]string{"a": "2.0", "b": "7a69"},
		},
		{
			name:     "decode with encoding.from_json function",
			input:    "encoding.from_json(`{ \"__address__\": \"localhost:8080\", \"x\": 123 }`)",
			expected: map[string]string{"__address__": "localhost:8080", "x": "123"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scope := vm.NewScope(map[string]any{})
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(t, err)
			eval := vm.New(expr)
			actual := Target{}
			require.NoError(t, eval.Evaluate(scope, &actual))
			require.Equal(t, NewTargetFromMap(tc.expected), actual)
		})
	}
}

func TestEncode_Decode_Targets(t *testing.T) {
	type testCase struct {
		name     string
		input    map[string]string
		expected string
	}

	tests := []testCase{
		{
			name:     "empty",
			input:    map[string]string{},
			expected: `target = {}`,
		},
		{
			name:  "simple",
			input: map[string]string{"banh": "mi", "char": "siu"},
			expected: `target = {
	banh = "mi",
	char = "siu",
}`,
		},
		{
			name:  "simple order change",
			input: map[string]string{"char": "siu", "banh": "mi", "bun": "cha"},
			expected: `target = {
	banh = "mi",
	bun  = "cha",
	char = "siu",
}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("encode into text", func(t *testing.T) {
				f := builder.NewFile()
				f.Body().SetAttributeValue("target", NewTargetFromMap(tc.input))
				encoded := string(f.Bytes())
				require.Equal(t, tc.expected, encoded)
			})

			var toDecode string
			t.Run("encode target", func(t *testing.T) {
				f := builder.NewFile()
				f.Body().SetAttributeValue("target", NewTargetFromMap(tc.input))
				encoded := string(f.Bytes())
				require.Equal(t, tc.expected, encoded)
				// store toDecode for other tests
				toDecode = strings.TrimPrefix(encoded, "target = ")
			})

			t.Run("decode into target", func(t *testing.T) {
				expr, err := parser.ParseExpression(toDecode)
				require.NoError(t, err)
				eval := vm.New(expr)
				actual := Target{}
				require.NoError(t, eval.Evaluate(vm.NewScope(map[string]any{}), &actual))
				require.Equal(t, NewTargetFromMap(tc.input), actual)
			})

			t.Run("decode into a map", func(t *testing.T) {
				expr, err := parser.ParseExpression(toDecode)
				require.NoError(t, err)
				eval := vm.New(expr)
				actualMap := map[string]string{}
				require.NoError(t, eval.Evaluate(vm.NewScope(map[string]any{}), &actualMap))
				require.Equal(t, tc.input, actualMap)
			})

			t.Run("decode into a map pointer", func(t *testing.T) {
				expr, err := parser.ParseExpression(toDecode)
				require.NoError(t, err)
				eval := vm.New(expr)
				actualMap := map[string]string{}
				require.NoError(t, eval.Evaluate(vm.NewScope(map[string]any{}), &actualMap))
				require.Equal(t, &tc.input, &actualMap)
			})

			t.Run("decode from target into map via scope", func(t *testing.T) {
				// If not supported, this would lead to error: target::ConvertInto: conversion to '*map[string]string' is not supported
				scope := vm.NewScope(map[string]any{"export": NewTargetFromMap(tc.input)})
				expr, err := parser.ParseExpression("export")
				require.NoError(t, err)
				eval := vm.New(expr)
				actualMap := map[string]string{}
				require.NoError(t, eval.Evaluate(scope, &actualMap))
				require.Equal(t, tc.input, actualMap)
			})

			t.Run("decode from map into target via scope", func(t *testing.T) {
				scope := vm.NewScope(map[string]any{"map": tc.input})
				expr, err := parser.ParseExpression("map")
				require.NoError(t, err)
				eval := vm.New(expr)
				actual := Target{}
				require.NoError(t, eval.Evaluate(scope, &actual))
				require.Equal(t, NewTargetFromMap(tc.input), actual)
			})
		})
	}
}

func TestEncode_Decode_TargetArrays(t *testing.T) {
	type testCase struct {
		name     string
		input    []map[string]string
		expected string
	}

	tests := []testCase{
		{
			name:     "nil",
			input:    nil,
			expected: `target = []`,
		},
		{
			name:     "empty",
			input:    []map[string]string{},
			expected: `target = []`,
		},
		{
			name: "simple two targets",
			input: []map[string]string{
				{"a": "5", "b": "10"},
				{"c": "5", "d": "10"},
			},
			expected: `target = [{
	a = "5",
	b = "10",
}, {
	c = "5",
	d = "10",
}]`,
		},
		{
			name: "nil target",
			input: []map[string]string{
				{"a": "5", "b": "10"},
				nil,
			},
			expected: `target = [{
	a = "5",
	b = "10",
}, {}]`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Set as map first to verify it acts the same as targets
			f := builder.NewFile()
			f.Body().SetAttributeValue("target", tc.input)
			require.Equal(t, tc.expected, string(f.Bytes()), "compliance check using a map")

			// Set as targets and check it's the same
			targets := make([]Target, 0)
			for _, m := range tc.input {
				targets = append(targets, NewTargetFromMap(m))
			}
			f = builder.NewFile()
			f.Body().SetAttributeValue("target", targets)
			encoded := string(f.Bytes())
			require.Equal(t, tc.expected, encoded, "using a target")

			// Try decoding now
			toDecode := strings.TrimPrefix(encoded, "target = ")
			scope := vm.NewScope(map[string]any{})
			expr, err := parser.ParseExpression(toDecode)
			require.NoError(t, err)
			eval := vm.New(expr)
			var actual []Target
			require.NoError(t, eval.Evaluate(scope, &actual))
			require.Equal(t, targets, actual)
		})
	}
}

func TestDecode_TargetArrays(t *testing.T) {
	type testCase struct {
		name     string
		input    string
		expected []map[string]string
	}

	tests := []testCase{
		{
			name:     "empty array",
			input:    `[]`,
			expected: []map[string]string{},
		},
		{
			name:  "simple two targets",
			input: `[{a = "5", b = "10"}, {c = "5",	d = "10"}]`,
			expected: []map[string]string{
				{"a": "5", "b": "10"},
				{"c": "5", "d": "10"},
			},
		},
		{
			name:  "concat targets",
			input: `array.concat([{a = "5", b = "10"}], [{c = "5",	d = "10"}])`,
			expected: []map[string]string{
				{"a": "5", "b": "10"},
				{"c": "5", "d": "10"},
			},
		},
		{
			name:  "concat nested targets",
			input: `array.concat(array.concat([{a = "5", b = "10"}], [{c = "5",	d = "10"}]), [{e = "5",	f = "10"}])`,
			expected: []map[string]string{
				{"a": "5", "b": "10"},
				{"c": "5", "d": "10"},
				{"e": "5", "f": "10"},
			},
		},
		{
			name:  "from_json",
			input: "encoding.from_json(`[ { \"__address__\": \"localhost:8080\", \"foo\": 123 }, {}, {\"bap\": \"boom\"} ]`)",
			expected: []map[string]string{
				{"__address__": "localhost:8080", "foo": "123"},
				{},
				{"bap": "boom"},
			},
		},
		{
			name:  "from_json one by one",
			input: "[encoding.from_json(`{ \"__address__\": \"localhost:8080\", \"foo\": 123 }`), encoding.from_json(`{ \"boom\": \"bap\", \"foo\": 321 }`)]",
			expected: []map[string]string{
				{"__address__": "localhost:8080", "foo": "123"},
				{"boom": "bap", "foo": "321"},
			},
		},
		{
			name:  "from_yaml",
			input: "encoding.from_yaml(`[ { __address__: localhost:8080, foo: 123 }, {}, {bap: boom} ]`)",
			expected: []map[string]string{
				{"__address__": "localhost:8080", "foo": "123"},
				{},
				{"bap": "boom"},
			},
		},
		{
			name:  "combine_maps",
			input: `array.combine_maps([{"a" = "a1", "b" = "b1"}, {"a" = "a1", "b" = "b1"}], [{"a" = "a1", "c" = "c1"}], ["a"])`,
			expected: []map[string]string{
				{"a": "a1", "b": "b1", "c": "c1"},
				{"a": "a1", "b": "b1", "c": "c1"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expectedTargets := make([]Target, 0)
			for _, m := range tc.expected {
				expectedTargets = append(expectedTargets, NewTargetFromMap(m))
			}

			scope := vm.NewScope(map[string]any{})
			expr, err := parser.ParseExpression(tc.input)
			require.NoError(t, err)
			eval := vm.New(expr)
			var actual []Target
			require.NoError(t, eval.Evaluate(scope, &actual))
			require.Equal(t, expectedTargets, actual)
		})
	}
}

func TestTargetMisc(t *testing.T) {
	target := NewTargetFromMap(map[string]string{"a": "5", "b": "10"})
	// Test can iterate over it
	var seen []string
	target.ForEachLabel(func(k string, v string) bool {
		seen = append(seen, fmt.Sprintf("%s=%s", k, v))
		return true
	})
	slices.Sort(seen)
	require.Equal(t, []string{"a=5", "b=10"}, seen)

	// Some loggers print targets out, check it's all good.
	require.Equal(t, `{"a"="5", "b"="10"}`, target.String())
}

func TestConvertFromNative(t *testing.T) {
	var nativeTargets = []model.LabelSet{
		{model.LabelName("hip"): model.LabelValue("hop")},
		{model.LabelName("nae"): model.LabelValue("nae")},
	}

	nativeGroup := &targetgroup.Group{
		Targets: nativeTargets,
		Labels: model.LabelSet{
			model.LabelName("boom"): model.LabelValue("bap"),
		},
		Source: "test",
	}

	expected := []Target{
		NewTargetFromMap(map[string]string{"hip": "hop", "boom": "bap"}),
		NewTargetFromMap(map[string]string{"nae": "nae", "boom": "bap"}),
	}

	require.True(t, equality.DeepEqual(expected, toAlloyTargets(map[string]*targetgroup.Group{"test": nativeGroup})))
}

func TestEquals_Custom(t *testing.T) {
	eq1 := NewTargetFromSpecificAndBaseLabelSet(
		model.LabelSet{"foo": "bar"},
		model.LabelSet{"hip": "hop"},
	)
	eq2 := NewTargetFromSpecificAndBaseLabelSet(
		nil,
		model.LabelSet{"hip": "hop", "foo": "bar"},
	)
	eq3 := NewTargetFromSpecificAndBaseLabelSet(
		model.LabelSet{"hip": "hop", "foo": "bar"},
		nil,
	)
	eq4 := NewTargetFromSpecificAndBaseLabelSet(
		model.LabelSet{"hip": "hop", "foo": "bar"},
		model.LabelSet{"foo": "baz"}, // overwritten by own set
	)

	equalTargets := []Target{eq1, eq2, eq3, eq4}

	ne1 := NewTargetFromSpecificAndBaseLabelSet(
		model.LabelSet{"foo": "bar"},
		nil,
	)
	ne2 := NewTargetFromSpecificAndBaseLabelSet(
		nil,
		model.LabelSet{"foo": "bar"},
	)
	ne3 := NewTargetFromSpecificAndBaseLabelSet(
		model.LabelSet{"boom": "bap"},
		model.LabelSet{"hip": "hop", "foo": "bar"},
	)
	ne4 := NewTargetFromSpecificAndBaseLabelSet(
		model.LabelSet{"hip": "hop", "foo": "bar"},
		model.LabelSet{"boom": "bap"},
	)
	ne5 := NewTargetFromSpecificAndBaseLabelSet(
		model.LabelSet{"foo": "baz"}, // takes precedence over the group
		model.LabelSet{"hip": "hop", "foo": "bar"},
	)
	notEqualTargets := []Target{ne1, ne2, ne3, ne4, ne5}

	for _, t1 := range equalTargets {
		for _, t2 := range equalTargets {
			require.True(t, t1.Equals(&t2), "should be equal: %v = %v", t1, t2)
			require.True(t, t1.EqualsTarget(&t2), "should be equal: %v = %v", t1, t2)
			require.True(t, t2.Equals(&t1), "should be equal: %v = %v", t1, t2)
			require.True(t, t2.EqualsTarget(&t1), "should be equal: %v = %v", t1, t2)
		}
	}

	for _, t1 := range notEqualTargets {
		for _, t2 := range equalTargets {
			require.False(t, t1.Equals(&t2), "should not be equal: %v <> %v", t1, t2)
			require.False(t, t1.EqualsTarget(&t2), "should not be equal: %v <> %v", t1, t2)
			require.False(t, t2.Equals(&t1), "should not be equal: %v <> %v", t1, t2)
			require.False(t, t2.EqualsTarget(&t1), "should not be equal: %v <> %v", t1, t2)
		}
	}
}

func TestHashing(t *testing.T) {
	labelsPerGenerator := 10
	targetsPerTestCase := 10
	type testCase struct {
		name               string
		labelGenerators    []func(targetInd, labelInd int) (string, string)
		hashOp             func(target Target) uint64
		expectedHash       uint64
		expectAllDifferent bool
	}

	testCases := []testCase{
		{
			name: "labels names different",
			labelGenerators: []func(targetInd, labelInd int) (string, string){
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("label_%d_%d", labelInd, targetInd), fmt.Sprintf("value_%d", labelInd)
				},
			},
			hashOp: func(target Target) uint64 {
				return target.HashLabelsWithPredicate(func(key string) bool {
					return true
				})
			},
			expectAllDifferent: true,
		},
		{
			name: "label values different",
			labelGenerators: []func(targetInd, labelInd int) (string, string){
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("label_%d", labelInd), fmt.Sprintf("value_%d_%d", labelInd, targetInd)
				},
			},
			hashOp: func(target Target) uint64 {
				return target.HashLabelsWithPredicate(func(key string) bool {
					return true
				})
			},
			expectAllDifferent: true,
		},
		{
			name: "all labels same for all targets",
			labelGenerators: []func(targetInd, labelInd int) (string, string){
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("label_%d", labelInd), fmt.Sprintf("value_%d", labelInd)
				},
			},
			hashOp: func(target Target) uint64 {
				return target.HashLabelsWithPredicate(func(key string) bool {
					return true
				})
			},
			expectedHash: 0xa28155048ff30d6f,
		},
		{
			name: "all labels same for all targets - non meta labels hash",
			labelGenerators: []func(targetInd, labelInd int) (string, string){
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("label_%d", labelInd), fmt.Sprintf("value_%d", labelInd)
				},
			},
			hashOp:       func(target Target) uint64 { return target.NonMetaLabelsHash() },
			expectedHash: 0xa28155048ff30d6f,
		},

		{
			name: "specific labels hash equal",
			labelGenerators: []func(targetInd, labelInd int) (string, string){
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("t%d_l%d", targetInd, labelInd), fmt.Sprintf("%d_%d", targetInd, labelInd)
				},
				// some const labels same for all to use in SpecificLabelsHash
				func(targetInd, labelInd int) (string, string) { return "foo", "bar" },
				func(targetInd, labelInd int) (string, string) { return "bin", "baz" },
			},
			hashOp:       func(target Target) uint64 { return target.SpecificLabelsHash([]string{"foo", "bin"}) },
			expectedHash: 0xbbbe498586b668f3,
		},
		{
			name: "specific labels hash different",
			labelGenerators: []func(targetInd, labelInd int) (string, string){
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("t%d_l%d", targetInd, labelInd), fmt.Sprintf("%d_%d", targetInd, labelInd)
				},
				// some const labels same for all to use in SpecificLabelsHash
				func(targetInd, labelInd int) (string, string) {
					return "foo", fmt.Sprintf("%d_%d", targetInd, labelInd)
				},
				func(targetInd, labelInd int) (string, string) {
					return "bin", fmt.Sprintf("%d_%d", targetInd, labelInd)
				},
			},
			hashOp:             func(target Target) uint64 { return target.SpecificLabelsHash([]string{"foo", "bin"}) },
			expectAllDifferent: true,
		},
		{
			name: "labels with predicate equal",
			labelGenerators: []func(targetInd, labelInd int) (string, string){
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("t%d_l%d", targetInd, labelInd), fmt.Sprintf("%d_%d", targetInd, labelInd)
				},
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("label_%d", labelInd), fmt.Sprintf("val_%d", labelInd)
				},
			},
			hashOp: func(target Target) uint64 {
				return target.HashLabelsWithPredicate(func(key string) bool {
					return strings.HasPrefix(key, "label_")
				})
			},
			expectedHash: 0x77c5d28715ca6a11,
		},
		{
			name: "labels with predicate different values",
			labelGenerators: []func(targetInd, labelInd int) (string, string){
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("t%d_l%d", targetInd, labelInd), fmt.Sprintf("%d_%d", targetInd, labelInd)
				},
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("label_%d", labelInd), fmt.Sprintf("val_%d_%d", labelInd, targetInd)
				},
			},
			hashOp: func(target Target) uint64 {
				return target.HashLabelsWithPredicate(func(key string) bool {
					return strings.HasPrefix(key, "label_")
				})
			},
			expectAllDifferent: true,
		},
		{
			name: "meta labels equal",
			labelGenerators: []func(targetInd, labelInd int) (string, string){
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("__meta_t%d_l%d", targetInd, labelInd), fmt.Sprintf("%d_%d", targetInd, labelInd)
				},
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("label_%d", labelInd), fmt.Sprintf("val_%d", labelInd)
				},
			},
			hashOp:       func(target Target) uint64 { return target.NonMetaLabelsHash() },
			expectedHash: 0x77c5d28715ca6a11,
		},
		{
			name: "meta labels different",
			labelGenerators: []func(targetInd, labelInd int) (string, string){
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("__meta_t%d_l%d", targetInd, labelInd), fmt.Sprintf("%d_%d", targetInd, labelInd)
				},
				func(targetInd, labelInd int) (string, string) {
					return fmt.Sprintf("label_%d", labelInd), fmt.Sprintf("val_%d_%d", labelInd, targetInd)
				},
			},
			hashOp:             func(target Target) uint64 { return target.NonMetaLabelsHash() },
			expectAllDifferent: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// verifies that all hashes are equal to each other and the expected hash (if it's specified)
			verifyCollectionEqual := func(hashes []uint64) {
				require.Greater(t, len(hashes), 0)
				firstActual := hashes[0]
				for _, hash := range hashes {
					require.Equal(t, firstActual, hash, "returned hashes are different between each other: %v", hashes)
				}
				if tc.expectedHash != 0 { // if specified, check the expected hash
					require.Equal(t, tc.expectedHash, firstActual, "returned hashes don't match the expected hash: %v vs %v", tc.expectedHash, hashes)
				}
			}

			verifyAllDifferent := func(hashes []uint64) {
				require.Greater(t, len(hashes), 0)
				unique := map[uint64]struct{}{}
				for _, hash := range hashes {
					unique[hash] = struct{}{}
				}
				require.Equal(t, len(unique), len(hashes), "hashes are not all unique: %v vs. unique: %v", hashes, unique)
			}

			var allHashes []uint64
			// create targetsPerTestCase targets
			for targetInd := 0; targetInd < targetsPerTestCase; targetInd++ {
				tb := NewTargetBuilder()
				// for each create bunch of labels using generators
				for _, generator := range tc.labelGenerators {
					for labelInd := 0; labelInd < labelsPerGenerator; labelInd++ {
						l, v := generator(targetInd, labelInd)
						tb.Set(l, v)
					}
				}

				// get the hashes
				actual := tc.hashOp(tb.Target())
				allHashes = append(allHashes, actual)
			}
			// verify all targets hashes together
			if tc.expectAllDifferent {
				verifyAllDifferent(allHashes)
			} else {
				verifyCollectionEqual(allHashes)
			}
		})
	}
}

func TestHashLabelsWithPredicateClearsStringSlicePool(t *testing.T) {
	var (
		target = NewTargetFromMap(map[string]string{
			"job": "hash-test",
			"env": "prod",
		})
		recordedLens    []int
		scratch         []string
		originalBorrow  = borrowLabelsSlice
		originalRelease = releaseLabelsSlice
	)

	t.Cleanup(func() {
		borrowLabelsSlice = originalBorrow
		releaseLabelsSlice = originalRelease
	})

	borrowLabelsSlice = func() []string {
		if scratch == nil {
			scratch = make([]string, 0, 8)
		}
		return scratch
	}
	releaseLabelsSlice = func(labels []string) {
		recordedLens = append(recordedLens, len(labels))
		scratch = labels
	}

	target.HashLabelsWithPredicate(func(string) bool {
		return true
	})
	target.HashLabelsWithPredicate(func(string) bool {
		return false
	})

	require.GreaterOrEqual(t, len(recordedLens), 2)
	require.Equal(t, 0, recordedLens[len(recordedLens)-1], "pool slice must be cleared before returning")
}

func TestHashLargeLabelSets(t *testing.T) {
	sharedLabels := 50
	ownLabels := 100
	labelsLength := 100 // large labels to verify the "slow" code path
	metaLabelsCount := 5

	chars := "abcdefghijklmnopqrstuvwxyz"

	genLabel := func(id, length int) string {
		sb := strings.Builder{}
		for i := 0; i < length; i++ {
			sb.WriteByte(chars[(i+id)%len(chars)])
		}
		return sb.String()
	}

	genLabelSet := func(size int) model.LabelSet {
		ls := model.LabelSet{}
		for i := 0; i < size; i++ {
			name := genLabel(i, labelsLength)
			value := genLabel(i, labelsLength)
			ls[model.LabelName(name)] = model.LabelValue(value)
		}
		for i := 0; i < metaLabelsCount; i++ {
			name := "__meta_" + genLabel(i, labelsLength)
			value := genLabel(i, labelsLength)
			ls[model.LabelName(name)] = model.LabelValue(value)
		}
		return ls
	}

	target := NewTargetFromSpecificAndBaseLabelSet(genLabelSet(ownLabels), genLabelSet(sharedLabels))
	expectedNonMetaLabelsHash := 0x374005f6a622f4d8
	expectedAllLabelsHash := 0x174c789bf3b783a7

	require.Equal(t, uint64(expectedNonMetaLabelsHash), target.NonMetaLabelsHash())
	require.Equal(t, uint64(expectedNonMetaLabelsHash), target.HashLabelsWithPredicate(func(key string) bool {
		return !strings.HasPrefix(key, "__meta_")
	}))
	require.Equal(t, uint64(expectedAllLabelsHash), target.HashLabelsWithPredicate(func(key string) bool {
		return true
	}))
	require.Equal(t, uint64(expectedAllLabelsHash), labels.StableHash(target.PromLabels())) // check it matches Prometheus algo

	var allNonMetaLabels []string
	target.ForEachLabel(func(k string, v string) bool {
		if !strings.HasPrefix(k, "__meta_") {
			allNonMetaLabels = append(allNonMetaLabels, k)
		}
		return true
	})

	require.Equal(t, uint64(expectedNonMetaLabelsHash), target.SpecificLabelsHash(allNonMetaLabels))
}

func TestComponentTargetsToPromTargetGroups(t *testing.T) {
	type testTarget struct {
		own   map[string]string
		group map[string]string
	}
	type args struct {
		jobName string
		tgs     []testTarget
	}
	tests := []struct {
		name                string
		args                args
		mockLabelSetEqualFn func(l1, l2 model.LabelSet) bool
		expected            map[string][]*targetgroup.Group
	}{
		{
			name:     "empty targets",
			args:     args{jobName: "job"},
			expected: map[string][]*targetgroup.Group{"job": {}},
		},
		{
			name: "targets all in same group",
			args: args{
				jobName: "job",
				tgs: []testTarget{
					{group: map[string]string{"hip": "hop"}, own: map[string]string{"boom": "bap"}},
					{group: map[string]string{"hip": "hop"}, own: map[string]string{"tiki": "ta"}},
				},
			},
			expected: map[string][]*targetgroup.Group{"job": {
				{
					Source: "job_part_9994420383135092995",
					Labels: mapToLabelSet(map[string]string{"hip": "hop"}),
					Targets: []model.LabelSet{
						mapToLabelSet(map[string]string{"boom": "bap"}),
						mapToLabelSet(map[string]string{"tiki": "ta"}),
					},
				},
			}},
		},
		{
			name: "two groups",
			args: args{
				jobName: "job",
				tgs: []testTarget{
					{group: map[string]string{"hip": "hop"}, own: map[string]string{"boom": "bap"}},
					{group: map[string]string{"kung": "foo"}, own: map[string]string{"tiki": "ta"}},
					{group: map[string]string{"hip": "hop"}, own: map[string]string{"hoo": "rey"}},
					{group: map[string]string{"kung": "foo"}, own: map[string]string{"bibim": "bap"}},
				},
			},
			expected: map[string][]*targetgroup.Group{"job": {
				{
					Source: "job_part_9994420383135092995",
					Labels: mapToLabelSet(map[string]string{"hip": "hop"}),
					Targets: []model.LabelSet{
						mapToLabelSet(map[string]string{"boom": "bap"}),
						mapToLabelSet(map[string]string{"hoo": "rey"}),
					},
				},
				{
					Source: "job_part_13313558424202542889",
					Labels: mapToLabelSet(map[string]string{"kung": "foo"}),
					Targets: []model.LabelSet{
						mapToLabelSet(map[string]string{"tiki": "ta"}),
						mapToLabelSet(map[string]string{"bibim": "bap"}),
					},
				},
			}},
		},
		{
			name: "two groups with hash conflict",
			mockLabelSetEqualFn: func(l1, l2 model.LabelSet) bool {
				if _, ok := l1[model.LabelName("hip")]; ok {
					return false
				}
				return l1.Equal(l2)
			},
			args: args{
				jobName: "job",
				tgs: []testTarget{
					{group: map[string]string{"hip": "hop"}, own: map[string]string{"boom": "bap"}},
					{group: map[string]string{"kung": "foo"}, own: map[string]string{"tiki": "ta"}},
					{group: map[string]string{"hip": "hop"}, own: map[string]string{"hoo": "rey"}},
					{group: map[string]string{"kung": "foo"}, own: map[string]string{"bibim": "bap"}},
				},
			},
			expected: map[string][]*targetgroup.Group{"job": {
				{
					Source: "job_part_13313558424202542889",
					Labels: mapToLabelSet(map[string]string{"kung": "foo"}),
					Targets: []model.LabelSet{
						mapToLabelSet(map[string]string{"tiki": "ta"}),
						mapToLabelSet(map[string]string{"bibim": "bap"}),
					},
				},
				{
					Source: "job_rest",
					Targets: []model.LabelSet{
						mapToLabelSet(map[string]string{"boom": "bap", "hip": "hop"}),
						mapToLabelSet(map[string]string{"hoo": "rey", "hip": "hop"}),
					},
				},
			}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockLabelSetEqualFn != nil {
				prev := labelSetEqualsFn
				labelSetEqualsFn = tt.mockLabelSetEqualFn
				defer func() {
					labelSetEqualsFn = prev
				}()
			}

			targets := make([]Target, 0, len(tt.args.tgs))
			for _, tg := range tt.args.tgs {
				targets = append(targets, NewTargetFromSpecificAndBaseLabelSet(mapToLabelSet(tg.own), mapToLabelSet(tg.group)))
			}
			actual := ComponentTargetsToPromTargetGroups(tt.args.jobName, targets)
			assert.Contains(t, actual, tt.args.jobName)
			assert.Equal(t, tt.expected, actual, "actual:\n%+v\nexpected:\n%+v\n", actual, tt.expected)
		})
	}
}

/*
	Recent run:

goos: darwin goarch: arm64 cpu: Apple M2
Benchmark_Targets_TypicalPipeline-8   	      36	  32868159 ns/op	 6022494 B/op	  100544 allocs/op
Benchmark_Targets_TypicalPipeline-8   	      34	  34562724 ns/op	 6109322 B/op	  100543 allocs/op
Benchmark_Targets_TypicalPipeline-8   	      34	  35662420 ns/op	 6022429 B/op	  100545 allocs/op
Benchmark_Targets_TypicalPipeline-8   	      36	  33446308 ns/op	 6021909 B/op	  100541 allocs/op
Benchmark_Targets_TypicalPipeline-8   	      34	  33537419 ns/op	 6022333 B/op	  100543 allocs/op
Benchmark_Targets_TypicalPipeline-8   	      34	  33687083 ns/op	 6109172 B/op	  100543 allocs/op
*/
func Benchmark_Targets_TypicalPipeline(b *testing.B) {
	sharedLabels := 5
	labelsPerTarget := 5
	labelsLength := 10
	targetsCount := 20_000
	numPeers := 10

	genLabelSet := func(size int) model.LabelSet {
		ls := model.LabelSet{}
		for i := 0; i < size; i++ {
			name, _ := goutils.RandomAlphaNumeric(labelsLength)
			value, _ := goutils.RandomAlphaNumeric(labelsLength)
			ls[model.LabelName(name)] = model.LabelValue(value)
		}
		return ls
	}

	var labelSets []model.LabelSet
	for i := 0; i < targetsCount; i++ {
		labelSets = append(labelSets, genLabelSet(labelsPerTarget))
	}

	cache := map[string]*targetgroup.Group{}
	cache["test"] = &targetgroup.Group{
		Targets: labelSets,
		Labels:  genLabelSet(sharedLabels),
		Source:  "test",
	}

	peers := make([]peer.Peer, 0, numPeers)
	for i := 0; i < numPeers; i++ {
		peerName := fmt.Sprintf("peer_%d", i)
		peers = append(peers, peer.Peer{Name: peerName, Addr: peerName, Self: i == 0, State: peer.StateParticipant})
	}

	cluster := &randomCluster{
		peers:        peers,
		peersByIndex: make(map[int][]peer.Peer, len(peers)),
	}

	b.ResetTimer()

	var prevDistTargets *DistributedTargets
	for i := 0; i < b.N; i++ {
		// Creating the targets in discovery
		targets := toAlloyTargets(cache)

		// Relabel of targets in discovery.relabel
		for ind := range targets {
			tb := NewTargetBuilderFrom(targets[ind])
			// would do alloy_relabel.ProcessBuilder here to relabel
			targets[ind] = tb.Target()
		}

		// prometheus.scrape: distributing targets for clustering
		dt := NewDistributedTargets(true, cluster, targets)
		_ = dt.LocalTargets()
		_ = dt.MovedToRemoteInstance(prevDistTargets)
		// Sending LabelSet to Prometheus library for scraping
		_ = ComponentTargetsToPromTargetGroups("test", targets)

		// Remote write happens on a sample level and largely outside Alloy's codebase, so skipping here.

		prevDistTargets = dt
	}
}

type randomCluster struct {
	peers []peer.Peer
	// stores results in a map to reduce the allocation noise in the benchmark
	peersByIndex map[int][]peer.Peer
}

func (f *randomCluster) Lookup(key shard.Key, _ int, _ shard.Op) ([]peer.Peer, error) {
	ind := int(key)
	if ind < 0 {
		ind = -ind
	}
	peerIndex := ind % len(f.peers)
	if _, ok := f.peersByIndex[peerIndex]; !ok {
		f.peersByIndex[peerIndex] = []peer.Peer{f.peers[peerIndex]}
	}
	return f.peersByIndex[peerIndex], nil
}

func (f *randomCluster) Peers() []peer.Peer {
	return f.peers
}

func (f *randomCluster) Ready() bool {
	return true
}

func mapToLabelSet(m map[string]string) model.LabelSet {
	r := make(model.LabelSet, len(m))
	for k, v := range m {
		r[model.LabelName(k)] = model.LabelValue(v)
	}
	return r
}
