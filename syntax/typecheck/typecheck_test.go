package typecheck

import (
	"testing"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/stretchr/testify/require"
)

type Args struct {
	Arg1       string            `alloy:"arg1,attr,optional"`
	Arg2       string            `alloy:"arg2,attr"`
	Capsule    alloytypes.Secret `alloy:"capsule,attr,optional"`
	Block1     Block1            `alloy:"block1,block"`
	Block2     []Block1          `alloy:"block2,block,optional"`
	Block3     [2]Block1         `alloy:"block3,block,optional"`
	Block4     Block2            `alloy:"block4,block,optional"`
	InnerBlock InnerBlock        `alloy:",squash"`
	EnumBlock  []EnumBlock       `alloy:"enum,enum,optional"`
	Arr        []string          `alloy:"arr,attr,optional"`
	ArrArr     [][]string        `alloy:"arrarr,attr,optional"`
	Obj        map[string]string `alloy:"obj,attr,optional"`
}

type Block1 struct {
	Arg1 string `alloy:"arg1,attr,optional"`
	Arg2 string `alloy:"arg2,attr"`
}

type Block2 struct {
	Nested NestedBlock `alloy:"nested,block"`
}

type NestedBlock struct {
	Arg string `alloy:"nested_arg,attr"`
}

type InnerBlock struct {
	Arg3 bool `alloy:"arg3,attr"`
}

type EnumBlock struct {
	Block1 *Block1     `alloy:"block1,block,optional"`
	Block2 *InnerBlock `alloy:"block2,block,optional"`
}

func TestBlock(t *testing.T) {
	type TestCase struct {
		desc        string
		src         []byte
		expectedErr []string
	}

	tests := []TestCase{
		{
			desc: "attributes ok",
			src: []byte(`
				test "name" {
					arg1 = "test"
					arg2 = "test"	
					arg3 = true
					capsule = "secret"
					block1 {
						arg1 = "test"
						arg2 = "test"
					}

					block2 {
						arg2 = "test"
					}
					
					block2 {
						arg1 = "test"
						arg2 = "test"
					}
				}
			`),
		},
		{
			desc: "missing optional attribute",
			src: []byte(`
				test "name" {
					arg2 = "test"	
					arg3 = true

					block1 {
						arg1 = "test"
						arg2 = "test"
					}
				}
			`),
		},
		{
			desc: "missing required attribute",
			src: []byte(`
				test "name" {
					arg1 = "test"
					arg3 = true

					block1 {
						arg1 = "test"
						arg2 = "test"
					}
				}
			`),
			expectedErr: []string{`2:5: missing required attribute "arg2"`},
		},

		{
			desc: "duplicated attribute",
			src: []byte(`
				test "name" {
					arg1 = "test"
					arg1 = "test"
					arg2 = "test"
					arg3 = true

					block1 {
						arg1 = "test"
						arg2 = "test"
					}
				}
			`),
			expectedErr: []string{`4:6: attribute "arg1" may only be provided once`},
		},
		{
			desc: "unknown attribute",
			src: []byte(`
					test "name" {
						unknown = "test"
						arg1 = "test"
						arg2 = "test"
						arg3 = true

						block1 {
							arg1 = "test"
							arg2 = "test"
						}
					}
				`),
			expectedErr: []string{`3:7: unrecognized attribute name "unknown"`},
		},
		{
			desc: "missing block",
			src: []byte(`
				test "name" {
					arg1 = "test"
					arg2 = "test"
					arg3 = true
				}
			`),
			expectedErr: []string{`2:5: missing required block "block1"`},
		},
		{
			desc: "missing required attribute in block",
			src: []byte(`
				test "name" {
					arg1 = "test"
					arg2 = "test"
					arg3 = true

					block1 {
						arg1 = "test"
					}
				}
			`),
			expectedErr: []string{`7:6: missing required attribute "arg2"`},
		},
		{
			desc: "missing required attribute in slice block",
			src: []byte(`
				test "name" {
					arg1 = "test"
					arg2 = "test"
					arg3 = true

					block1 {
						arg1 = "test"
						arg2 = "test"
					}
		
					block2 {
						arg2 = "test"
					}
					
					block2 {
						arg1 = "test"
					}
				}
			`),
			expectedErr: []string{`16:6: missing required attribute "arg2"`},
		},
		{
			desc: "to many blocks when type is array with 2 elements",
			src: []byte(`
				test "name" {
					arg1 = "test"
					arg2 = "test"
					arg3 = true

					block1 {
						arg1 = "test"
						arg2 = "test"
					}
		
					block3 {}
		
					block3 {}
					
					block3 {}
				}
			`),
			expectedErr: []string{
				`12:6: block "block3" must be specified exactly 2 times, but was specified 3 times`,
				`14:6: block "block3" must be specified exactly 2 times, but was specified 3 times`,
				`16:6: block "block3" must be specified exactly 2 times, but was specified 3 times`,
			},
		},
		{
			desc: "enum block",
			src: []byte(`
				test "name" {
					arg1 = "test"
					arg2 = "test"	
					arg3 = true
					block1 {
						arg1 = "test"
						arg2 = "test"
					}

					block2 {
						arg2 = "test"
					}
					
					block2 {
						arg1 = "test"
						arg2 = "test"
					}

					enum.block1 {
						arg2 = "test"
					}
	
					enum.block2 {
						arg3 = true
					}
				}
			`),
		},
		{
			desc: "missing required attribute in enum",
			src: []byte(`
				test "name" {
					arg1 = "test"
					arg2 = "test"	
					arg3 = true
					block1 {
						arg1 = "test"
						arg2 = "test"
					}

					block2 {
						arg2 = "test"
					}
					
					block2 {
						arg1 = "test"
						arg2 = "test"
					}

					enum.block1 {
						arg2 = "test"
					}
	
					enum.block2 {}
				}
			`),
			expectedErr: []string{`24:6: missing required attribute "arg3"`},
		},
		{
			desc: "missing required attribute nested block",
			src: []byte(`
				test "name" {
					arg2 = "test"
					arg3 = false

					block1 {
						arg1 = "test"
						arg2 = "test"
					}

					block4 {
						nested {}
					}
				}
			`),
			expectedErr: []string{`12:7: missing required attribute "nested_arg"`},
		},
		{
			desc: "wrong literal",
			src: []byte(`
				test "name" {
					arg1 = true
					arg2 = "test"	
					arg3 = true
					arr = ["test", 1]
					obj = { "key" = 1 }
					capsule = 1

					block1 {
						arg1 = "test"
						arg2 = "test"
					}

					block2 {
						arg2 = "test"
					}
					
					block2 {
						arg1 = "test"
						arg2 = "test"
					}
				}
			`),
			expectedErr: []string{
				`3:13: "arg1" should be string, got bool`,
				`6:21: "arr" should be string, got number`,
				`7:22: "obj" should be string, got number`,
				`8:16: "capsule" should be capsule, got number`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			file, err := parser.ParseFile("", tt.src)
			require.NoError(t, err)
			diags := Block(file.Body[0].(*ast.BlockStmt), &Args{})

			require.Equal(t, len(tt.expectedErr), len(diags))
			for i := range diags {
				require.EqualError(t, diags[i], tt.expectedErr[i])
			}
		})
	}
}

func TestBlockMap(t *testing.T) {
	type Args struct {
		Map map[string]any `alloy:"map,block"`
	}

	src := []byte(`
		test "name" {
			map {
				a = 1
				b = "str"
			}
		}
	`)

	file, err := parser.ParseFile("", src)
	require.NoError(t, err)
	diag := Block(file.Body[0].(*ast.BlockStmt), &Args{})
	require.Len(t, diag, 0)
}
