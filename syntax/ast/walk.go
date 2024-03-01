package ast

import "fmt"

// A Visitor has its Visit method invoked for each node encountered by Walk. If
// the resulting visitor w is not nil, Walk visits each of the children of node
// with the visitor w, followed by a call of w.Visit(nil).
type Visitor interface {
	Visit(node Node) (w Visitor)
}

// Walk traverses an AST in depth-first order: it starts by calling
// v.Visit(node); node must not be nil. If the visitor w returned by
// v.Visit(node) is not nil, Walk is invoked recursively with visitor w for
// each of the non-nil children of node, followed by a call of w.Visit(nil).
func Walk(v Visitor, node Node) {
	if v = v.Visit(node); v == nil {
		return
	}

	// Walk children. The order of the cases matches the declared order of nodes
	// in ast.go.
	switch n := node.(type) {
	case *File:
		Walk(v, n.Body)
	case Body:
		for _, s := range n {
			Walk(v, s)
		}
	case *AttributeStmt:
		Walk(v, n.Name)
		Walk(v, n.Value)
	case *BlockStmt:
		Walk(v, n.Body)
	case *Ident:
		// Nothing to do
	case *IdentifierExpr:
		Walk(v, n.Ident)
	case *LiteralExpr:
		// Nothing to do
	case *ArrayExpr:
		for _, e := range n.Elements {
			Walk(v, e)
		}
	case *ObjectExpr:
		for _, f := range n.Fields {
			Walk(v, f.Name)
			Walk(v, f.Value)
		}
	case *AccessExpr:
		Walk(v, n.Value)
		Walk(v, n.Name)
	case *IndexExpr:
		Walk(v, n.Value)
		Walk(v, n.Index)
	case *CallExpr:
		Walk(v, n.Value)
		for _, a := range n.Args {
			Walk(v, a)
		}
	case *UnaryExpr:
		Walk(v, n.Value)
	case *BinaryExpr:
		Walk(v, n.Left)
		Walk(v, n.Right)
	case *ParenExpr:
		Walk(v, n.Inner)
	default:
		panic(fmt.Sprintf("river/ast: unexpected node type %T", n))
	}

	v.Visit(nil)
}
