package validator

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"

	"github.com/grafana/alloy/internal/component"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/service"
)

type Options struct {
	// ServiceDefinitions is used to validate service config.
	ServiceDefinitions []service.Definition
	// ComponentRegistry is used to validate component config.
	ComponentRegistry component.Registry
}

func Validate(sources map[string][]byte, opts Options) error {
	source, err := alloy_runtime.ParseSources(sources)
	if err != nil {
		return err
	}

	sm := make(map[string]service.Definition)
	for _, def := range opts.ServiceDefinitions {
		sm[def.Name] = def
	}

	components, _ := splitComponents(source.Components(), sm)

	diags := validateComponents(components, opts.ComponentRegistry)
	if diags != nil {
		return diags
	}

	return nil
}

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
	}

	return diags
}

func splitComponents(blocks []*ast.BlockStmt, sm map[string]service.Definition) ([]*ast.BlockStmt, []*ast.BlockStmt) {
	components := make([]*ast.BlockStmt, 0, len(blocks))
	services := make([]*ast.BlockStmt, 0, len(sm))

	for _, b := range blocks {
		if _, isService := sm[blockID(b)]; isService {
			services = append(services, b)
		} else {
			components = append(components, b)
		}
	}

	return components, services
}

func blockID(b *ast.BlockStmt) string {
	id := make([]string, 0, len(b.Name)+1)
	id = append(id, b.Name...)
	if b.Label != "" {
		id = append(id, b.Label)
	}
	return strings.Join(id, ".")
}

func Report(w io.Writer, err error, sources map[string][]byte) {
	var diags diag.Diagnostics
	if errors.As(err, &diags) {
		p := diag.NewPrinter(diag.PrinterConfig{
			Color:              !color.NoColor,
			ContextLinesBefore: 1,
			ContextLinesAfter:  1,
		})
		_ = p.Fprint(w, sources, diags)

		// Print newline after the diagnostics.
		fmt.Println()
		return
	}

	_, _ = fmt.Fprintf(w, "validation failed: %s", err)
}
