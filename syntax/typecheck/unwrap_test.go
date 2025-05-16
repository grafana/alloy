package typecheck

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/internal/value"
	"github.com/grafana/alloy/syntax/parser"
)

func TestTryUnwrapBlockAttr(t *testing.T) {
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
			v := TryUnwrapBlockAttr(file.Body[0].(*ast.BlockStmt), tt.attr, tt.dv)

			require.Equal(t, v.Type(), tt.expected.Type())
			require.Equal(t, v.Reflect(), tt.expected.Reflect())
		})
	}

}
