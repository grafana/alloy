package validator

import (
	"fmt"
	"slices"
	"strings"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/typecheck"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/dynamic/foreach"
	"github.com/grafana/alloy/internal/featuregate"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/tracing"
	"github.com/grafana/alloy/internal/service"
)

type Options struct {
	// Sources are all source files to validate.
	Sources map[string][]byte
	// ServiceDefinitions is used to validate service config.
	ServiceDefinitions []service.Definition
	// ComponentRegistry is used to validate component config.
	ComponentRegistry component.Registry
	// MinStability is the minimum stability level of features that can be used by the collector. It is defined by
	// the user, for example, via command-line flags.
	MinStability featuregate.Stability
}

func Validate(opts Options) error {
	v := newValidator(opts)
	return v.run(newComponentRegistry(opts.ComponentRegistry))
}

type validator struct {
	minStability featuregate.Stability
	sources      map[string][]byte
	sm           map[string]service.Definition
}

func newValidator(opts Options) *validator {
	sm := make(map[string]service.Definition)
	for _, def := range opts.ServiceDefinitions {
		sm[def.Name] = def
	}

	return &validator{
		minStability: opts.MinStability,
		sources:      opts.Sources,
		sm:           sm,
	}
}

func (v *validator) run(cr *componentRegistry) error {
	s, err := alloy_runtime.ParseSources(v.sources)
	if err != nil {
		return err
	}

	// Register all "import" blocks as custom component.
	for _, c := range s.Configs() {
		if c.Name[0] == "import" {
			cr.registerCustomComponent(c)
		}
	}

	var diags diag.Diagnostics
	// Need to validate declares first becuse we will register "custom" components.
	diags.Merge(v.validateDeclares(s.Declares(), cr))
	diags.Merge(v.validateConfigs(s.Configs(), cr))

	components, services := splitComponents(s.Components(), v.sm)
	diags.Merge(v.validateComponents(components, cr))
	diags.Merge(v.validateServices(services))

	if diags.HasErrors() {
		return diags
	}

	return nil
}

// validateDeclares will perform validation on declare blocks and register them as "custom" component.
func (v *validator) validateDeclares(declares []*ast.BlockStmt, cr *componentRegistry) diag.Diagnostics {
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
		} else {
			// Only register custom component if we have a label
			// Without a label there is no way to create one.
			cr.registerCustomComponent(d)
		}

		// 2. Declares need to be unique
		if diag, ok := blockAlreadyDefined(mem, d); ok {
			diags.Add(diag)
		}
	}

	return diags
}

// validateConfigs will perform validation on config blocks.
func (v *validator) validateConfigs(configs []*ast.BlockStmt, cr *componentRegistry) diag.Diagnostics {
	var (
		diags diag.Diagnostics
		mem   = make(map[string]*ast.BlockStmt, len(configs))
	)

	for _, c := range configs {
		// 1. Config blocks needs to be unique.
		if diag, ok := blockAlreadyDefined(mem, c); ok {
			diags.Add(diag)
		}

		if c.Name[0] == "import" {
			// We need to register import blocks as a custom component.
			cr.registerCustomComponent(c)
		}

		// In config we store blocks for logging, tracing, argument, export, import.file,
		// import.string, import.http, import.git and foreach.
		// For now we only typecheck logging and tracing and ignore the rest.
		switch c.GetBlockName() {
		case "logging":
			args := &logging.Options{}
			diags.Merge(typecheck.Block(c, args))
		case "tracing":
			args := &tracing.Options{}
			diags.Merge(typecheck.Block(c, args))
		case foreach.Name:
			diags.Merge(v.validateForeach(c, cr))
		}
	}

	return diags
}

func (v *validator) validateForeach(block *ast.BlockStmt, cr *componentRegistry) diag.Diagnostics {
	var diags diag.Diagnostics

	name := block.GetBlockName()
	// Check required stability level.
	if err := featuregate.CheckAllowed(foreach.StabilityLevel, v.minStability, fmt.Sprintf("foreach block %q", name)); err != nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: block.NamePos.Position(),
			EndPos:   block.NamePos.Add(len(name) - 1).Position(),
			Message:  err.Error(),
		})
	}

	// Require label for all foreach blocks.
	if block.Label == "" {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: block.NamePos.Position(),
			EndPos:   block.NamePos.Add(len(name) - 1).Position(),
			Message:  "declare block must have a label",
		})
	}

	var (
		body     ast.Body
		template *ast.BlockStmt
	)

	for _, stmt := range block.Body {
		if b, ok := stmt.(*ast.BlockStmt); ok && b.GetBlockName() == foreach.TypeTemplate {
			template = b
			continue
		}
		body = append(body, stmt)
	}

	// Set the body of block to all non template properties.
	block.Body = body
	diags.Merge(typecheck.Block(block, &foreach.Arguments{}))

	// Foreach blocks must have a template.
	if template == nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: ast.StartPos(block).Position(),
			EndPos:   ast.EndPos(block).Position(),
			Message:  fmt.Sprintf("missing required block %q", foreach.TypeTemplate),
		})
		return diags
	}

	// We extract all blocks from template body and evaluate them as components.
	var (
		configs    = make([]*ast.BlockStmt, 0, len(template.Body))
		components = make([]*ast.BlockStmt, 0, len(template.Body))
	)

	for _, stmt := range template.Body {
		b, ok := stmt.(*ast.BlockStmt)
		if !ok {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: ast.StartPos(stmt).Position(),
				EndPos:   ast.EndPos(stmt).Position(),
				Message:  fmt.Sprintf("unsupported statement type %T", stmt),
			})
			continue
		}

		var validNames = [...]string{foreach.Name, "import.file", "import.string", "import.http", "import.git"}
		if slices.Contains(validNames[:], b.GetBlockName()) {
			configs = append(configs, b)
			continue
		}

		components = append(components, b)
	}

	foreachCr := newComponentRegistry(cr)

	// We can reuse validateConfigs here since we know that all block in nested
	// are foreach.
	diags.Merge(v.validateConfigs(configs, foreachCr))
	// Validate all other blocks as components.
	diags.Merge(v.validateComponents(components, foreachCr))
	return diags
}

// validateComponents will perform validation on component blocks.
func (v *validator) validateComponents(components []*ast.BlockStmt, cr component.Registry) diag.Diagnostics {
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
		reg, err := cr.Get(name)
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

		// 3. Components need to be unique.
		if diag, ok := blockAlreadyDefined(mem, c); ok {
			diags.Add(diag)
		}

		// Skip components without any arguments.
		if reg.Args == nil {
			continue
		}

		// 4. Perform typecheck on component.
		diags.Merge(typecheck.Block(c, reg.CloneArguments()))
	}

	return diags
}

func (v *validator) validateServices(services []*ast.BlockStmt) diag.Diagnostics {
	var (
		diags diag.Diagnostics
		mem   = make(map[string]*ast.BlockStmt, len(services))
	)

	for _, s := range services {
		def := v.sm[s.GetBlockName()]

		if diag, ok := blockAlreadyDefined(mem, s); ok {
			diags.Add(diag)
		}

		if def.ConfigType == nil {
			diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: ast.StartPos(s).Position(),
				EndPos:   ast.EndPos(s).Position(),
				Message:  fmt.Sprintf("service %q does not support being configured", def.Name),
			})
			continue
		}

		diags.Merge(typecheck.Block(s, def.CloneConfig()))
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
