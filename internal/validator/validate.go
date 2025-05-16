package validator

import (
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/typecheck"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/nodeconf/argument"
	"github.com/grafana/alloy/internal/nodeconf/export"
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

	components, services := splitComponents(s.Components(), v.sm)

	rootState := &state{
		root:       true,
		graph:      newGraph(),
		declares:   s.Declares(),
		configs:    s.Configs(),
		components: components,
		services:   services,
		cr:         cr,
	}

	diags := validateGraph(v.validate(rootState))
	if diags.HasErrors() {
		return diags
	}

	return nil
}

type state struct {
	root       bool
	foreach    bool
	graph      *orderedGraph
	declares   []*ast.BlockStmt
	configs    []*ast.BlockStmt
	components []*ast.BlockStmt
	services   []*ast.BlockStmt
	cr         *componentRegistry
	// arguments registered by module
	arguments []*ast.BlockStmt
}

func (v *validator) validate(s *state) *state {
	// Need to validate declares first because we will register "custom" components.
	v.validateDeclares(s)
	v.validateConfigs(s)

	v.validateComponents(s)
	v.validateServices(s)
	return s
}

// validateDeclares will perform validation on declare blocks and register them as "custom" component.
func (v *validator) validateDeclares(s *state) {
	mem := make(map[string]*ast.BlockStmt, len(s.declares))

	for i, d := range s.declares {
		node := newBlockNode(d)

		// Declare blocks must have a label.
		if node.block.Label == "" {
			node.diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: d.NamePos.Position(),
				EndPos:   d.NamePos.Add(len(d.GetBlockName()) - 1).Position(),
				Message:  "declare block must have a label",
			})
		}

		// Declares need to be unique
		if diag, ok := blockAlreadyDefined(mem, node.block); ok {
			node.diags.Add(diag)
			// We need to generate a unique id for this duplicated node so we can still typecheck it.
			node.id = node.id + "-" + strconv.Itoa(i)
		}

		// Add declare to graph
		s.graph.Add(node)

		configs, declares, services, components := extractBlocks(node, node.block.Body, v.sm)

		moduleState := &state{
			root:       false,
			graph:      newGraph(),
			declares:   declares,
			configs:    configs,
			services:   services,
			components: components,
			cr:         newComponentRegistry(s.cr),
		}

		// Add module state as node to graph
		s.graph.Add(newSubNode(node, v.validate(moduleState)))

		if node.block.Label != "" {
			s.cr.registerCustomComponent(node.block, generateArgumentsStruct(moduleState.arguments))
		}
	}
}

// validateConfigs will perform validation on config blocks.
func (v *validator) validateConfigs(s *state) {
	mem := make(map[string]*ast.BlockStmt, len(s.configs))

	for i, c := range s.configs {
		node := newBlockNode(c)
		// Config blocks needs to be unique.
		if diag, ok := blockAlreadyDefined(mem, node.block); ok {
			node.diags.Add(diag)
			// We need to generate a unique id for this duplicated node so we can still typecheck it.
			node.id = node.id + "-" + strconv.Itoa(i)
		} else if c.Name[0] == "import" {
			// We need to register import blocks as a custom component.
			s.cr.registerCustomComponent(node.block, nil)
		}

		name := node.block.GetBlockName()

		if name != "logging" && name != "tracing" {
			if node.block.Label == "" {
				node.diags.Add(diag.Diagnostic{
					Severity: diag.SeverityLevelError,
					StartPos: node.block.NamePos.Position(),
					EndPos:   node.block.NamePos.Add(len(name) - 1).Position(),
					Message:  fmt.Sprintf("%s block must have a label", name),
				})
			}
		}

		// In configs we store blocks for logging, tracing, argument, export, import.file,
		// import.string, import.http, import.git and foreach.
		switch name {
		case "logging":
			node.args = &logging.Options{}
			if diag, ok := blockDisallowed(s, node.block); ok {
				node.diags.Add(diag)
			}
			s.graph.Add(node)
		case "tracing":
			node.args = &tracing.Options{}
			if diag, ok := blockDisallowed(s, node.block); ok {
				node.diags.Add(diag)
			}
			s.graph.Add(node)
		case foreach.BlockName:
			node.args = &foreach.Arguments{}
			v.validateForeach(node, s)
		case importsource.BlockNameFile:
			node.args = &importsource.FileArguments{}
			s.graph.Add(node)
		case importsource.BlockNameString:
			node.args = &importsource.StringArguments{}
			s.graph.Add(node)
		case importsource.BlockNameHTTP:
			node.args = &importsource.HTTPArguments{}
			s.graph.Add(node)
		case importsource.BlockNameGit:
			node.args = &importsource.GitArguments{}
			s.graph.Add(node)
		case argument.BlockName:
			node.args = &argument.Arguments{}
			if s.root {
				node.diags.Add(diag.Diagnostic{
					Severity: diag.SeverityLevelError,
					Message:  "argument blocks only allowed inside a module",
					StartPos: ast.StartPos(node.block).Position(),
					EndPos:   ast.EndPos(node.block).Position(),
				})
			}
			if node.block.Label != "" {
				s.arguments = append(s.arguments, node.block)
			}
			s.graph.Add(node)
		case export.BlockName:
			node.args = &export.Arguments{}
			if s.root {
				node.diags.Add(diag.Diagnostic{
					Severity: diag.SeverityLevelError,
					Message:  "export blocks only allowed inside a module",
					StartPos: ast.StartPos(node.block).Position(),
					EndPos:   ast.EndPos(node.block).Position(),
				})
			}
			s.graph.Add(node)
		}
	}
}

func (v *validator) validateForeach(node *blockNode, s *state) {
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

	// Require label for foreach block.
	if node.block.Label == "" {
		node.diags.Add(diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			StartPos: node.block.NamePos.Position(),
			EndPos:   node.block.NamePos.Add(len(name) - 1).Position(),
			Message:  "foreach block must have a label",
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
		s.graph.Add(node)
		return
	}

	s.graph.Add(node)
	// We extract all blocks from template body and evaluate them as components.
	configs, declares, services, components := extractBlocks(node, template.Body, v.sm)

	// Add foreach state as node to the graph
	s.graph.Add(newSubNode(node, v.validate(&state{
		root:       s.root,
		foreach:    true,
		graph:      newGraph(),
		declares:   declares,
		configs:    configs,
		services:   services,
		components: components,
		cr:         newComponentRegistry(s.cr),
	})))
}

// validateComponents will perform validation on component blocks.
func (v *validator) validateComponents(s *state) {
	mem := make(map[string]*ast.BlockStmt, len(s.components))

	for i, c := range s.components {
		var (
			node = newComponentNode(c)
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

		s.graph.Add(node)
	}
}

func (v *validator) validateServices(s *state) {
	mem := make(map[string]*ast.BlockStmt, len(s.services))

	for i, c := range s.services {
		var (
			node = newBlockNode(c)
			def  = v.sm[c.GetBlockName()]
		)

		if diag, ok := blockAlreadyDefined(mem, node.block); ok {
			node.diags.Add(diag)
			// We need to generate a unique id for this duplicated node so we can still typecheck it.
			node.id = node.id + "-" + strconv.Itoa(i)
		}

		if diag, ok := blockDisallowed(s, node.block); ok {
			node.diags.Add(diag)
		}

		if def.ConfigType == nil {
			node.diags.Add(diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				StartPos: ast.StartPos(c).Position(),
				EndPos:   ast.EndPos(c).Position(),
				Message:  fmt.Sprintf("service %q does not support being configured", def.Name),
			})
		} else {
			node.args = def.CloneConfig()
		}

		s.graph.Add(node)
	}
}

var configBlockNames = [...]string{
	foreach.BlockName, argument.BlockName, export.BlockName, "logging", "tracing",
	importsource.BlockNameFile, importsource.BlockNameString, importsource.BlockNameHTTP, importsource.BlockNameGit,
}

// extractBlocks extracts configs, declares and components blocks from body
func extractBlocks(node *blockNode, body ast.Body, sm map[string]service.Definition) ([]*ast.BlockStmt, []*ast.BlockStmt, []*ast.BlockStmt, []*ast.BlockStmt) {
	var (
		configs    = make([]*ast.BlockStmt, 0, len(body))
		declares   = make([]*ast.BlockStmt, 0, len(body))
		services   = make([]*ast.BlockStmt, 0, len(body))
		components = make([]*ast.BlockStmt, 0, len(body))
	)

	for _, stmt := range body {
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

		if slices.Contains(configBlockNames[:], b.GetBlockName()) {
			configs = append(configs, b)
			continue
		}

		if b.GetBlockName() == "declare" {
			declares = append(declares, b)
			continue
		}

		if _, ok := sm[blockID(b)]; ok {
			services = append(services, b)
			continue
		}

		components = append(components, b)
	}

	return configs, declares, services, components
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

func blockDisallowed(s *state, b *ast.BlockStmt) (diag.Diagnostic, bool) {
	id := blockID(b)
	if !s.root {
		return diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			Message:  fmt.Sprintf("%s not allowed in module", id),
			StartPos: b.NamePos.Position(),
			EndPos:   b.NamePos.Add(len(id) - 1).Position(),
		}, true
	}

	if s.foreach {
		return diag.Diagnostic{
			Severity: diag.SeverityLevelError,
			Message:  fmt.Sprintf("%s not allowed in foreach", id),
			StartPos: b.NamePos.Position(),
			EndPos:   b.NamePos.Add(len(id) - 1).Position(),
		}, true
	}

	return diag.Diagnostic{}, false
}

func generateArgumentsStruct(args []*ast.BlockStmt) any {
	mem := make(map[string]struct{})
	fields := make([]reflect.StructField, 0, len(args))
	for _, a := range args {
		if _, ok := mem[a.Label]; ok {
			continue
		}
		mem[a.Label] = struct{}{}

		optional := typecheck.TryUnwrapBlockAttr(a, "optional", syntax.ValueFromBool(false))

		var tag string
		if optional.Bool() {
			tag = fmt.Sprintf(`alloy:"%s,attr,optional"`, a.Label)
		} else {
			tag = fmt.Sprintf(`alloy:"%s,attr"`, a.Label)
		}

		f := reflect.StructField{
			Name: cases.Title(language.English).String(a.Label),
			Type: reflect.TypeFor[any](),
			Tag:  reflect.StructTag(tag),
		}
		fields = append(fields, f)
	}

	return reflect.New(reflect.StructOf(fields)).Interface()
}
