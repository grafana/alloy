package print

import (
	"bytes"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/printer"
)

func PrintNode(node ast.Node) (str string, err error) {
	var buf bytes.Buffer
	err = printer.Fprint(&buf, node)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}
