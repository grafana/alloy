package validator

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/typecheck"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/dynamic/foreach"
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

	// Register all "import" blocks as custom component.
	for _, c := range s.Configs() {
		if c.Name[0] == "import" {
			v.cr.registerCustomComponent(c)
		}
	}

	var diags diag.Diagnostics
	// Need to validate declares first becuse we will register "custom" components.
	diags.Merge(v.validateDeclares(s.Declares()))
	diags.Merge(v.validateConfigs(s.Configs()))

	components, services := splitComponents(s.Components(), v.sm)
	diags.Merge(v.validateComponents(components))
	diags.Merge(v.validateServices(services))

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
		} else {
			// Only register custom component if we have a label
			// Without a label there is no way to create one.
			v.cr.registerCustomComponent(d)
		}

		// 2. Declares need to be unique
		if diag, ok := blockAlreadyDefined(mem, d); ok {
			diags.Add(diag)
		}
	}

	return diags
}

// validateConfigs will perform validation on config blocks.
func (v *validator) validateConfigs(configs []*ast.BlockStmt) diag.Diagnostics {
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
			v.cr.registerCustomComponent(c)
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
		case "foreach":
			diags.Merge(v.validateForeach(c))
		}
	}

	return diags
}

func (v *validator) validateForeach(block *ast.BlockStmt) diag.Diagnostics {
	var diags diag.Diagnostics

	if block.Label == "" {

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

	if template == nil {
		diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: ast.StartPos(block).Position(),
			EndPos:   ast.EndPos(block).Position(),
			Message:  fmt.Sprintf("missing required block %q", foreach.TypeTemplate),
		})
		return diags
	}

	components := make([]*ast.BlockStmt, 0, len(template.Body))
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
		components = append(components, b)
	}

	diags.Merge(v.validateComponents(components))
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
		reg, custom, err := v.cr.Get(name)
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

		// 3. Components need to be unique
		if diag, ok := blockAlreadyDefined(mem, c); ok {
			diags.Add(diag)
		}

		// For now we are skipping typecheking for custom components (modules and declares)
		if custom {
			continue
		}

		// 4. Perform typecheck on component
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
