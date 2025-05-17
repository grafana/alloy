package typecheck

import (
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/internal/transform"
)

// TryUnwrapBlockAttr tries to unwrap the attribute value.
// If the attribute are not found or cannot be unwraped defaultValue it returned.
// The value returned is guaranteed to be of the same kind as defaultValue.
func TryUnwrapBlockAttr(b *ast.BlockStmt, name string, defaultValue syntax.Value) syntax.Value {
	aw := &attrWalker{target: name, value: defaultValue}
	ast.Walk(aw, b)

	return aw.value
}

type attrWalker struct {
	target string
	value  syntax.Value
}

func (aw *attrWalker) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.AttributeStmt:
		if n.Name.Name == aw.target {
			switch expr := n.Value.(type) {
			case *ast.LiteralExpr:
				v, err := transform.ValueFromLiteral(expr.Value, expr.Kind)
				if err == nil && v.Type() == aw.value.Type() {
					aw.value = v
				}
			}
			return nil
		}
	}
	return aw
}
