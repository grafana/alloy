package validator

import (
	"github.com/grafana/alloy/internal/dag"
	"github.com/grafana/alloy/internal/featuregate"
	astutil "github.com/grafana/alloy/internal/util/ast"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/vm"
)

func findReferences(cn dag.Node, g astutil.Graph, scope *vm.Scope, minStability featuregate.Stability) ([]astutil.Reference, diag.Diagnostics) {
	var (
		traversals []astutil.Traversal

		diags diag.Diagnostics
	)

	switch cn := cn.(type) {
	case blockNode:
		if cn.Block() != nil {
			traversals = astutil.TraversalsFromBody(cn.Block().Body)
		}
	}

	refs := make([]astutil.Reference, 0, len(traversals))
	for _, t := range traversals {
		ref, resolveDiags := astutil.ResolveTraversal(t, g)
		componentRefMatch := !resolveDiags.HasErrors()

		_, scopeMatch := scope.Lookup(t[0].Name)
		if !componentRefMatch && !scopeMatch {
			diags.Merge(resolveDiags)
			continue
		}

		if componentRefMatch {
			if scope.IsStdlibIdentifiers(t[0].Name) {
				diags.Add(diag.Diagnostic{
					Severity: diag.SeverityLevelWarn,
					Message:  "a component is shadowing an existing stdlib name",
					StartPos: ast.StartPos(t[0]).Position(),
					EndPos:   ast.EndPos(t[0]).Position(),
				})
			}
			refs = append(refs, ref)
		} else if scope.IsStdlibDeprecated(t[0].Name) {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelWarn,
				Message:  "this stdlib function is deprecated; please refer to the documentation for updated usage and alternatives",
				StartPos: ast.StartPos(t[0]).Position(),
				EndPos:   ast.EndPos(t[len(t)-1]).Position(),
			})
		} else if funcName := t.String(); scope.IsStdlibExperimental(funcName) {
			if err := featuregate.CheckAllowed(featuregate.StabilityExperimental, minStability, funcName); err != nil {
				diags = append(diags, diag.Diagnostic{
					Severity: diag.SeverityLevelError,
					Message:  err.Error(),
					StartPos: ast.StartPos(t[0]).Position(),
					EndPos:   ast.EndPos(t[len(t)-1]).Position(),
				})
				continue
			}
		}
	}

	return refs, diags
}
