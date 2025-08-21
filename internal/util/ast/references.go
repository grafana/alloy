package ast

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/dag"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
)

type Reference struct {
	Target    dag.Node
	Traversal Traversal
}

type Graph interface {
	GetByID(string) dag.Node
}

// ResolveTraversal will step trough the traversal and try to resolve a node from the graph.
func ResolveTraversal(t Traversal, g Graph) (Reference, diag.Diagnostics) {
	var (
		diags diag.Diagnostics

		partial = Traversal{t[0]}
		rem     = t[1:]
	)

	for {
		if n := g.GetByID(partial.String()); n != nil {
			return Reference{
				Target:    n,
				Traversal: rem,
			}, nil
		}

		if len(rem) == 0 {
			// Stop: there's no more elements to look at in the traversal.
			break
		}

		// Append the next name in the traversal to our partial reference.
		partial = append(partial, rem[0])
		rem = rem[1:]
	}

	diags = append(diags, diag.Diagnostic{
		Severity: diag.SeverityLevelError,
		Message:  fmt.Sprintf("component %q does not exist or is out of scope", partial),
		StartPos: ast.StartPos(t[0]).Position(),
		EndPos:   ast.EndPos(t[len(t)-1]).Position(),
	})

	return Reference{}, diags
}

// TraversalsFromBody recurses through body and finds all variable
// potential references.
func TraversalsFromBody(body ast.Body) []Traversal {
	var w traversalWalker
	ast.Walk(&w, body)

	// Flush after the walk in case there was an in-progress traversal.
	w.flush()
	return w.traversals
}

// Traversal describes accessing a sequence of fields relative to a component.
// Traversal only include uninterrupted sequences of field accessors; for an
// expression "component.field_a.field_b.field_c[0].inner_field", the Traversal
// will be (field_a, field_b, field_c).
type Traversal []*ast.Ident

func (t Traversal) String() string {
	var fieldNames []string
	for _, field := range t {
		fieldNames = append(fieldNames, field.Name)
	}
	return strings.Join(fieldNames, ".")
}

type traversalWalker struct {
	traversals []Traversal

	buildTraversal   bool      // Whether
	currentTraversal Traversal // currentTraversal being built.
}

func (tw *traversalWalker) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.IdentifierExpr:
		// Identifiers always start new traversals. Pop the last one.
		tw.flush()
		tw.buildTraversal = true
		tw.currentTraversal = append(tw.currentTraversal, n.Ident)

	case *ast.AccessExpr:
		ast.Walk(tw, n.Value)

		// Fields being accessed should get only added to the traversal if one is
		// being built. This will be false for accesses like a().foo.
		if tw.buildTraversal {
			tw.currentTraversal = append(tw.currentTraversal, n.Name)
		}
		return nil

	case *ast.IndexExpr:
		// Indexing interrupts traversals so we flush after walking the value.
		ast.Walk(tw, n.Value)
		tw.flush()
		ast.Walk(tw, n.Index)
		return nil

	case *ast.CallExpr:
		// Calls interrupt traversals so we flush after walking the value.
		ast.Walk(tw, n.Value)
		tw.flush()
		for _, arg := range n.Args {
			ast.Walk(tw, arg)
		}
		return nil
	}

	return tw
}

// flush will flush the in-progress traversal to the traversals list and unset
// the buildTraversal state.
func (tw *traversalWalker) flush() {
	if tw.buildTraversal && len(tw.currentTraversal) > 0 {
		tw.traversals = append(tw.traversals, tw.currentTraversal)
	}
	tw.buildTraversal = false
	tw.currentTraversal = nil
}
