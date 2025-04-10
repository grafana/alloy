package validator

import (
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
)

// validateComponents will perform validation on component blocks.
func validateComponents(components []*ast.BlockStmt) diag.Diagnostics {
	var diags diag.Diagnostics

	for _, c := range components {
		name := c.GetBlockName()

		// 1. All components must have a label.
		if c.Label == "" {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: c.NamePos.Position(),
				EndPos:   c.NamePos.Add(len(name) - 1).Position(),
				Message:  fmt.Sprintf("component %q must have a label", name),
			})
		}

		// FIXME
		reg, ok := component.Get(name)
		fmt.Println(reg, ok)
	}

	return diags
}
