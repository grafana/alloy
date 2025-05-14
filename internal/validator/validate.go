package validator

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/typecheck"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/nodeconf/foreach"
	"github.com/grafana/alloy/internal/nodeconf/importsource"
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
	return v.validate(s.Declares(), s.Configs(), s.Components(), cr)
}

func (v *validator) validate(declares, configs, components []*ast.BlockStmt, cr *componentRegistry) diag.Diagnostics {
	var (
		diags diag.Diagnostics
		graph = newGraph()
	)

	// Need to validate declares first becuse we will register "custom" components.
	v.validateDeclares(declares, cr, graph)
	v.validateConfigs(configs, cr, graph)

	components, services := splitComponents(components, v.sm)
	v.validateComponents(components, cr, graph)
	v.validateServices(services, graph)

	for n := range graph.Nodes() {
		// Add any non type check diags
		diags.Merge(n.diags)
		if n.args != nil {
			diags.Merge(typecheck.Block(n.block, n.args))
		}
	}

	if diags.HasErrors() {
		return diags
	}

	return nil
}

// validateDeclares will perform validation on declare blocks and register them as "custom" component.
func (v *validator) validateDeclares(declares []*ast.BlockStmt, cr *componentRegistry, g *orderedGraph) {
	mem := make(map[string]*ast.BlockStmt, len(declares))

	for i, d := range declares {
		node := newBlockNode(d)

		// Declare blocks must have a label.
		if d.Label == "" {
			node.diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: d.NamePos.Position(),
				EndPos:   d.NamePos.Add(len(d.GetBlockName()) - 1).Position(),
				Message:  "declare block must have a label",
			})
		} else {
			// Only register custom component if we have a label
			// Without a label there is no way to create one.
			cr.registerCustomComponent(node.block)
		}

		// Declares need to be unique
		if diag, ok := blockAlreadyDefined(mem, node.block); ok {
			node.diags.Add(diag)
			// We need to generate a unique id for this duplicated node so we can still typecheck it.
			node.id = node.id + "-" + strconv.Itoa(i)
		}

		// Add declare to graph
		g.Add(node)
	}
}

// validateConfigs will perform validation on config blocks.
func (v *validator) validateConfigs(configs []*ast.BlockStmt, cr *componentRegistry, g *orderedGraph) {
	mem := make(map[string]*ast.BlockStmt, len(configs))

	for i, c := range configs {
		node := newBlockNode(c)
		// Config blocks needs to be unique.
		if diag, ok := blockAlreadyDefined(mem, node.block); ok {
			node.diags.Add(diag)
			// We need to generate a unique id for this duplicated node so we can still typecheck it.
			node.id = node.id + "-" + strconv.Itoa(i)
		} else if c.Name[0] == "import" {
			// We need to register import blocks as a custom component.
			cr.registerCustomComponent(node.block)
		}

		// In configs we store blocks for logging, tracing, argument, export, import.file,
		// import.string, import.http, import.git and foreach.
		switch c.GetBlockName() {
		case "logging":
			node.args = &logging.Options{}
			g.Add(node)
		case "tracing":
			node.args = &tracing.Options{}
			g.Add(node)
		case foreach.BlockName:
			node.args = &foreach.Arguments{}
			v.validateForeach(node, cr, g)
		case importsource.BlockNameFile:
			node.args = &importsource.FileArguments{}
			g.Add(node)
		case importsource.BlockNameString:
			node.args = &importsource.StringArguments{}
			g.Add(node)
		case importsource.BlockNameHTTP:
			node.args = &importsource.HTTPArguments{}
			g.Add(node)
		case importsource.BlockNameGit:
			node.args = &importsource.GitArguments{}
			g.Add(node)
		}
	}
}

func (v *validator) validateForeach(node *blockNode, cr *componentRegistry, g *orderedGraph) {
	name := node.block.GetBlockName()
	// Check required stability level.
	if err := featuregate.CheckAllowed(foreach.StabilityLevel, v.minStability, fmt.Sprintf("foreach block %q", name)); err != nil {
		node.diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: node.block.NamePos.Position(),
			EndPos:   node.block.NamePos.Add(len(name) - 1).Position(),
			Message:  err.Error(),
		})
	}

	// Require label for all foreach blocks.
	if node.block.Label == "" {
		node.diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: node.block.NamePos.Position(),
			EndPos:   node.block.NamePos.Add(len(name) - 1).Position(),
			Message:  "declare block must have a label",
		})
	}

	var (
		body     ast.Body
		template *ast.BlockStmt
	)

	for _, stmt := range node.block.Body {
		if b, ok := stmt.(*ast.BlockStmt); ok && b.GetBlockName() == foreach.TypeTemplate {
			template = b
			continue
		}
		body = append(body, stmt)
	}

	// Set the body of block to all non template properties.
	node.block.Body = body

	// Foreach blocks must have a template.
	if template == nil {
		node.diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: ast.StartPos(node.block).Position(),
			EndPos:   ast.EndPos(node.block).Position(),
			Message:  fmt.Sprintf("missing required block %q", foreach.TypeTemplate),
		})
		g.Add(node)
		return
	}

	// We extract all blocks from template body and evaluate them as components.
	var (
		configs    = make([]*ast.BlockStmt, 0, len(template.Body))
		declares   = make([]*ast.BlockStmt, 0, len(template.Body))
		components = make([]*ast.BlockStmt, 0, len(template.Body))
	)

	for _, stmt := range template.Body {
		b, ok := stmt.(*ast.BlockStmt)
		if !ok {
			node.diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: ast.StartPos(stmt).Position(),
				EndPos:   ast.EndPos(stmt).Position(),
				Message:  fmt.Sprintf("unsupported statement type %T", stmt),
			})
			continue
		}

		var validNames = [...]string{
			foreach.BlockName, importsource.BlockNameFile,
			importsource.BlockNameString, importsource.BlockNameHTTP, importsource.BlockNameGit,
		}

		if slices.Contains(validNames[:], b.GetBlockName()) {
			configs = append(configs, b)
			continue
		}

		if b.GetBlockName() == "declare" {
			declares = append(declares, b)
		}

		components = append(components, b)
	}

	node.diags.Merge(v.validate(declares, configs, components, newComponentRegistry(cr)))
	g.Add(node)
}

// validateComponents will perform validation on component blocks.
func (v *validator) validateComponents(components []*ast.BlockStmt, cr component.Registry, g *orderedGraph) {
	mem := make(map[string]*ast.BlockStmt, len(components))

	for i, c := range components {
		var (
			node = newBlockNode(c)
			name = node.block.GetBlockName()
		)
		// All components must have a label.
		if c.Label == "" {
			node.diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: node.block.NamePos.Position(),
				EndPos:   node.block.NamePos.Add(len(name) - 1).Position(),
				Message:  fmt.Sprintf("component %q must have a label", name),
			})
		}

		// Components need to be unique.
		if diag, ok := blockAlreadyDefined(mem, node.block); ok {
			node.diags.Add(diag)
			// We need to generate a unique id for this duplicated node so we can still typecheck it.
			node.id = node.id + "-" + strconv.Itoa(i)
		}

		// Check if component exists and can be used.
		reg, err := cr.Get(name)
		if err != nil {
			node.diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: c.NamePos.Position(),
				EndPos:   c.NamePos.Add(len(name) - 1).Position(),
				Message:  err.Error(),
			})
			g.Add(node)
			// We cannot do further validation if the component don't exist.
			continue
		}

		if reg.Args != nil {
			node.args = reg.CloneArguments()
		}

		g.Add(node)
	}
}

func (v *validator) validateServices(services []*ast.BlockStmt, g *orderedGraph) {
	mem := make(map[string]*ast.BlockStmt, len(services))

	for i, s := range services {
		var (
			node = newBlockNode(s)
			def  = v.sm[s.GetBlockName()]
		)

		if diag, ok := blockAlreadyDefined(mem, node.block); ok {
			node.diags.Add(diag)
			// We need to generate a unique id for this duplicated node so we can still typecheck it.
			node.id = node.id + "-" + strconv.Itoa(i)
		}

		if def.ConfigType == nil {
			node.diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: ast.StartPos(s).Position(),
				EndPos:   ast.EndPos(s).Position(),
				Message:  fmt.Sprintf("service %q does not support being configured", def.Name),
			})
		} else {
			node.args = def.CloneConfig()
		}

		g.Add(node)
	}

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
