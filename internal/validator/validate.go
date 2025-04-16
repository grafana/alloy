package validator

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"

	"github.com/grafana/alloy/internal/component"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/service"
)

type Options struct {
	// Sources are all source files to validate.
	Sources map[string][]byte
	// ServiceDefinitions is used to validate service config.
	ServiceDefinitions []service.Definition
	// ComponentRegistry is used to validate component config.
	ComponentRegistry component.Registry
}

func Validate(opts Options) error {
	v := newValidator(opts)
	return v.run()
}

type validator struct {
	sources map[string][]byte
	sm      map[string]service.Definition
	cr      *componentRegistry
}

func newValidator(opts Options) *validator {
	sm := make(map[string]service.Definition)
	for _, def := range opts.ServiceDefinitions {
		sm[def.Name] = def
	}

	return &validator{
		sources: opts.Sources,
		sm:      sm,
		cr:      newComponentRegistry(opts.ComponentRegistry),
	}
}

func (v *validator) run() error {
	s, err := alloy_runtime.ParseSources(v.sources)
	if err != nil {
		return err
	}
	var diags diag.Diagnostics

	declareDiags := v.validateDeclares(s.Declares())
	diags = append(diags, declareDiags...)

	components, _ := splitComponents(s.Components(), v.sm)

	componentDiags := v.validateComponents(components)
	diags = append(diags, componentDiags...)

	if diags.HasErrors() {
		return diags
	}

	return nil
}

// validateDeclares will perform validation on declare blocks and register them as "custom" component.
func (v *validator) validateDeclares(declares []*ast.BlockStmt) diag.Diagnostics {
	var (
		diags diag.Diagnostics
		mem   = make(map[string]*ast.BlockStmt, len(declares))
	)

	for _, d := range declares {
		name := d.GetBlockName()

		// 1. Declare blocks must have a label.
		if d.Label == "" {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: d.NamePos.Position(),
				EndPos:   d.NamePos.Add(len(name) - 1).Position(),
				Message:  "declare block must have a label",
			})
		}

		v.cr.registerCustomComponent(d)

		// 2. Two of the same declare blocks cannot share label.
		if diag, ok := blockAlreadyDefined(mem, d); ok {
			diags.Add(diag)
		}
	}

	return diags
}

// validateComponents will perform validation on component blocks.
func (v *validator) validateComponents(components []*ast.BlockStmt) diag.Diagnostics {
	var (
		diags diag.Diagnostics
		mem   = make(map[string]*ast.BlockStmt, len(components))
	)

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
		_, err := v.cr.Get(name)
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

		// 2. Two of the same component cannot share label.
		if diag, ok := blockAlreadyDefined(mem, c); ok {
			diags.Add(diag)
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

func blockAlreadyDefined(mem map[string]*ast.BlockStmt, b *ast.BlockStmt) (diag.Diagnostic, bool) {
	id := blockID(b)
	if orig, redefined := mem[id]; redefined {
		return diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			Message:  fmt.Sprintf("block %s already declared at %s", id, ast.StartPos(orig).Position()),
			StartPos: b.NamePos.Position(),
			EndPos:   b.NamePos.Add(len(id) - 1).Position(),
		}, true
	}
	mem[id] = b
	return diag.Diagnostic{}, false
}
