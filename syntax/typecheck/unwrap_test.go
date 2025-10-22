package typecheck

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/internal/value"
	"github.com/grafana/alloy/syntax/parser"
)

func TestUnwrapBlockAttr(t *testing.T) {
	type TestCase struct {
		desc     string
		attr     string
		dv       syntax.Value
		src      []byte
		expected syntax.Value
	}

	tests := []TestCase{
		{
			desc:     "unwrap value",
			attr:     "optional",
			dv:       value.Bool(false),
			expected: value.Bool(true),
			src: []byte(`
				argument "test" {
					optional = true
				}
			`),
		},
		{
			desc:     "fallback to default for wrong type",
			attr:     "optional",
			dv:       value.Bool(false),
			expected: value.Bool(false),
			src: []byte(`
				argument "test" {
					optional = "test"
				}
			`),
		},
		{
			desc:     "fallback to default for missing",
			attr:     "optional",
			dv:       value.Bool(false),
			expected: value.Bool(false),
			src: []byte(`
				argument "test" {}
			`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			file, err := parser.ParseFile("", tt.src)
			require.NoError(t, err)
			v := UnwrapBlockAttr(file.Body[0].(*ast.BlockStmt), tt.attr, tt.dv)

			require.Equal(t, v.Type(), tt.expected.Type())
			require.Equal(t, v.Reflect(), tt.expected.Reflect())
		})
	}
}

func TestTryUnwrapBlockAttr(t *testing.T) {
	type TestCase struct {
		desc       string
		attr       string
		kind       reflect.Kind
		src        []byte
		expected   syntax.Value
		expectedOk bool
	}

	tests := []TestCase{
		{
			desc:       "unwrap value",
			attr:       "optional",
			kind:       reflect.Bool,
			expectedOk: true,
			expected:   value.Bool(true),
			src: []byte(`
				argument "test" {
					optional = true
				}
			`),
		},
		{
			desc:       "false for wrong type",
			attr:       "optional",
			kind:       reflect.Bool,
			expectedOk: false,
			expected:   value.Null,
			src: []byte(`
				argument "test" {
					optional = "test"
				}
			`),
		},
		{
			desc:       "false for default for missing",
			attr:       "optional",
			kind:       reflect.Bool,
			expectedOk: false,
			expected:   value.Null,
			src: []byte(`
				argument "test" {}
			`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			file, err := parser.ParseFile("", tt.src)
			require.NoError(t, err)
			v, ok := TryUnwrapBlockAttr(file.Body[0].(*ast.BlockStmt), tt.attr, tt.kind)

			require.Equal(t, tt.expected.Type(), v.Type())
			require.Equal(t, tt.expectedOk, ok)
		})
	}
}
