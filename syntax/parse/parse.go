package parse

import (
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
)

func ParseString(str string) (*ast.File, error) {
	return parser.ParseFile("", []byte(str))
}
