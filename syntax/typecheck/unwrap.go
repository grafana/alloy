package typecheck

import (
	"reflect"

	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/internal/transform"
)

// UnwrapBlockAttr tries to unwrap the attribute value.
// If the attribute are not found or cannot be unwraped defaultValue it returned.
// The value returned is guaranteed to be of the same kind as defaultValue.
func UnwrapBlockAttr(b *ast.BlockStmt, name string, defaultValue syntax.Value) syntax.Value {
	aw := &attrWalker{target: name, targetKind: defaultValue.Reflect().Kind(), value: defaultValue}
	ast.Walk(aw, b)

	return aw.value
}

// TryUnwrapBlockAttr tries to unwrap the attribute value.
// If the attribute are not found or cannot be unwrapped
// the second return argument is set to false.
func TryUnwrapBlockAttr(b *ast.BlockStmt, name string, kind reflect.Kind) (syntax.Value, bool) {
	aw := &attrWalker{target: name, targetKind: kind}
	ast.Walk(aw, b)
	return aw.value, aw.found
}

type attrWalker struct {
	target     string
	targetKind reflect.Kind
	found      bool
	value      syntax.Value
}

func (aw *attrWalker) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.AttributeStmt:
		if n.Name.Name == aw.target {
			switch expr := n.Value.(type) {
			case *ast.LiteralExpr:
				v, err := transform.ValueFromLiteral(expr.Value, expr.Kind)
				if err == nil && v.Reflect().Kind() == aw.targetKind {
					aw.value = v
					aw.found = true
				}
			}
			return nil
		}
	}
	return aw
}
