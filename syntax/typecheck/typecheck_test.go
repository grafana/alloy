package typecheck

import (
	"fmt"
	"testing"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/stretchr/testify/require"
)

type Args struct {
	Arg1   string `alloy:"arg1,attr,optional"`
	Arg2   string `alloy:"arg2,attr"`
	Block1 Block1 `alloy:"block1,block"`
}

type Block1 struct {
	BlockArg1 string `alloy:"block_arg1,attr,optional"`
	BlockArg2 string `alloy:"block_arg2,attr"`
}

func TestBlock(t *testing.T) {
	type TestCase struct {
		desc        string
		src         []byte
		expectedErr string
	}

	tests := []TestCase{
		{
			desc: "attributes ok",
			src: []byte(`
				test "name" {
					arg1 = "test"
					arg2 = "test"	
					block1 {
						block_arg1 = "test"
						block_arg2 = "test"
					}
				}
			`),
		},
		{
			desc: "missing optional attribute",
			src: []byte(`
				test "name" {
					arg2 = "test"	
					block1 {
						block_arg1 = "test"
						block_arg2 = "test"
					}
				}
			`),
		},
		{
			desc: "missing required attribute",
			src: []byte(`
				test "name" {
					arg1 = "test"
					block1 {
						block_arg1 = "test"
						block_arg2 = "test"
					}
				}
			`),
			expectedErr: `2:5: missing required attribute "arg2"`,
		},

		{
			desc: "duplicated attribute",
			src: []byte(`
				test "name" {
					arg1 = "test"
					arg1 = "test"
					arg2 = "test"
					block1 {
						block_arg1 = "test"
						block_arg2 = "test"
					}
				}
			`),
			expectedErr: `4:6: attribute "arg1" may only be provided once`,
		},
		{
			desc: "unknown attribute",
			src: []byte(`
					test "name" {
						unknown = "test"
						arg1 = "test"
						arg2 = "test"
						block1 {
							block_arg1 = "test"
							block_arg2 = "test"
						}
					}
				`),
			expectedErr: `3:7: unrecognized attribute name "unknown"`,
		},
		{
			desc: "missing block",
			src: []byte(`
				test "name" {
					arg1 = "test"
					arg2 = "test"
				}
			`),
			expectedErr: `2:5: missing required block "block1"`,
		},
		{
			desc: "missing required attribute in block",
			src: []byte(`
				test "name" {
					arg1 = "test"
					arg2 = "test"
					block1 {
						block_arg1 = "test"
					}
				}
			`),
			expectedErr: `5:6: missing required attribute "block_arg2"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			if tt.desc == "missing required attribute in block" {
				fmt.Println()
			}

			file, err := parser.ParseFile("", []byte(tt.src))
			require.NoError(t, err)
			diag := Block(file.Body[0].(*ast.BlockStmt), &Args{})
			if tt.expectedErr == "" {
				require.Len(t, diag, 0)
			} else {
				require.EqualError(t, diag, tt.expectedErr)
			}
		})
	}
}
