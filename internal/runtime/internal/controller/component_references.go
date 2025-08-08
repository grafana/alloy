package controller

import (
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/dag"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	astutil "github.com/grafana/alloy/internal/util/ast"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/vm"
)

// ComponentReferences returns the list of references a component is making to
// other components.
func ComponentReferences(cn dag.Node, g *dag.Graph, l log.Logger, scope *vm.Scope, minStability featuregate.Stability) ([]astutil.Reference, diag.Diagnostics) {
	var (
		traversals []astutil.Traversal

		diags diag.Diagnostics
	)

	switch cn := cn.(type) {
	case BlockNode:
		if cn.Block() != nil {
			traversals = astutil.TraversalsFromBody(cn.Block().Body)
		}
	}

	refs := make([]astutil.Reference, 0, len(traversals))
	for _, t := range traversals {
		ref, resolveDiags := astutil.ResolveTraversal(t, g)
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
				level.Warn(l).Log("msg", "a component is shadowing an existing stdlib name", "component", ref.Target.NodeID(), "stdlib name", t[0].Name)
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
