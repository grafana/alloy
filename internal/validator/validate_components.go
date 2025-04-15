package validator

import (
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
)

// validateComponents will perform validation on component blocks.
func validateComponents(components []*ast.BlockStmt, registry component.Registry) diag.Diagnostics {
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

		// 2. Check if component exists and can be used.
		_, err := registry.Get(name)
		if err != nil {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: c.NamePos.Position(),
				EndPos:   c.NamePos.Add(len(name) - 1).Position(),
				Message:  err.Error(),
			})

			// We cannot do further validation if the component don't exist.
			continue
		}

		// FIXME(kalleep): implement validation / type checking or arguments.
	}

	return diags
}
