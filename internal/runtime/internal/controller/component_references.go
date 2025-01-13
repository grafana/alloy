package controller

import (
	"fmt"
	"strings"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/internal/dag"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/vm"
)

// Traversal describes accessing a sequence of fields relative to a component.
// Traversal only include uninterrupted sequences of field accessors; for an
// expression "component.field_a.field_b.field_c[0].inner_field", the Traversal
// will be (field_a, field_b, field_c).
type Traversal []*ast.Ident

// String returns a dot-separated string representation of the field names in the traversal.
// For example, a traversal of fields [field_a, field_b, field_c] returns "field_a.field_b.field_c".
// Returns an empty string if the traversal contains no fields.
func (t Traversal) String() string {
	var fieldNames []string
	for _, field := range t {
		fieldNames = append(fieldNames, field.Name)
	}
	return strings.Join(fieldNames, ".")
}

// Reference describes an Alloy expression reference to a BlockNode.
type Reference struct {
	Target BlockNode // BlockNode being referenced

	// Traversal describes which nested field relative to Target is being
	// accessed.
	Traversal Traversal
}

// ComponentReferences returns the list of references a component is making to
// other components.
func ComponentReferences(cn dag.Node, g *dag.Graph, l log.Logger, scope *vm.Scope, minStability featuregate.Stability) ([]Reference, diag.Diagnostics) {
	var (
		traversals []Traversal

		diags diag.Diagnostics
	)

	switch cn := cn.(type) {
	case BlockNode:
		if cn.Block() != nil {
			traversals = expressionsFromBody(cn.Block().Body)
		}
	}

	refs := make([]Reference, 0, len(traversals))
	for _, t := range traversals {
		ref, resolveDiags := resolveTraversal(t, g)
		componentRefMatch := !resolveDiags.HasErrors()

		// we look for a match in the provided scope and the stdlib
		_, scopeMatch := scope.Lookup(t[0].Name)

		if !componentRefMatch && !scopeMatch {
			// The traversal for the foreach node is used at the foreach level to access the references from outside of the foreach block.
			// This is quite handy but not perfect because:
			// - it fails with the var
			// - it fails at the root level to link two components that are inside of the template (because they are not evaluated at the root level)
			// Both cases should be ignored at the linking level, that's the diags are ignored here.
			// This is not super clean, but it should not create any problem since that the errors will be caught either during evaluation or while linking components
			// inside of the foreach.
			if _, ok := cn.(*ForeachConfigNode); !ok {
				diags = append(diags, resolveDiags...)
			}
			continue
		}

		if componentRefMatch {
			if scope.IsStdlibIdentifiers(t[0].Name) {
				level.Warn(l).Log("msg", "a component is shadowing an existing stdlib name", "component", strings.Join(ref.Target.Block().Name, "."), "stdlib name", t[0].Name)
			}
			refs = append(refs, ref)
		} else if scope.IsStdlibDeprecated(t[0].Name) {
			level.Warn(l).Log("msg", "this stdlib function is deprecated; please refer to the documentation for updated usage and alternatives", "function", t[0].Name)
		} else if funcName := t.String(); scope.IsStdlibExperimental(funcName) {
			if err := featuregate.CheckAllowed(featuregate.StabilityExperimental, minStability, funcName); err != nil {
				diags = append(diags, diag.Diagnostic{
					Severity: diag.SeverityLevelError,
					Message:  err.Error(),
					StartPos: ast.StartPos(t[0]).Position(),
					EndPos:   ast.StartPos(t[len(t)-1]).Position(),
				})
				continue
			}
		}
	}

	return refs, diags
}

// expressionsFromSyntaxBody recurses through body and finds all variable
// references.
func expressionsFromBody(body ast.Body) []Traversal {
	var w traversalWalker
	ast.Walk(&w, body)

	// Flush after the walk in case there was an in-progress traversal.
	w.flush()
	return w.traversals
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

func resolveTraversal(t Traversal, g *dag.Graph) (Reference, diag.Diagnostics) {
	var (
		diags diag.Diagnostics

		partial = ComponentID{t[0].Name}
		rem     = t[1:]
	)

	for {
		if n := g.GetByID(partial.String()); n != nil {
			return Reference{
				Target:    n.(BlockNode),
				Traversal: rem,
			}, nil
		}

		if len(rem) == 0 {
			// Stop: there's no more elements to look at in the traversal.
			break
		}

		// Append the next name in the traversal to our partial reference.
		partial = append(partial, rem[0].Name)
		rem = rem[1:]
	}

	diags = append(diags, diag.Diagnostic{
		Severity: diag.SeverityLevelError,
		Message:  fmt.Sprintf("component %q does not exist or is out of scope", partial),
		StartPos: ast.StartPos(t[0]).Position(),
		EndPos:   ast.StartPos(t[len(t)-1]).Position(),
	})
	return Reference{}, diags
}
