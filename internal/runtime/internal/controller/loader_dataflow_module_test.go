package controller

import (
	"errors"
	"os"
	"strconv"
	"testing"

	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
)

// newReproLoader builds a Loader for the issue #4062 regression test. It
// reuses the in-package test facilities (NewModuleControllerMock backs custom
// component instantiation); MinStability must be below GenerallyAvailable for
// testcomponents to load.
func newReproLoader(t *testing.T) *Loader {
	t.Helper()

	logger, _ := logging.New(os.Stderr, logging.DefaultOptions)
	l, err := NewLoader(LoaderOptions{
		ComponentGlobals: ComponentGlobals{
			Logger:            logger,
			TraceProvider:     noop.NewTracerProvider(),
			DataPath:          t.TempDir(),
			MinStability:      featuregate.StabilityPublicPreview,
			OnBlockNodeUpdate: func(BlockNode) {},
			Registerer:        prometheus.NewRegistry(),
			NewModuleController: func(_ ModuleControllerOpts) ModuleController {
				return NewModuleControllerMock()
			},
		},
	})
	require.NoError(t, err)
	return l
}

// reproFileToBlock parses Alloy configuration text into a list of block
// statements. It mirrors fileToBlock from the external test package, which is
// not visible from the in-package tests.
func reproFileToBlock(t *testing.T, bytes []byte) ([]*ast.BlockStmt, diag.Diagnostics) {
	t.Helper()
	var diags diag.Diagnostics
	file, err := parser.ParseFile(t.Name(), bytes)

	var parseDiags diag.Diagnostics
	if errors.As(err, &parseDiags); parseDiags.HasErrors() {
		return nil, parseDiags
	}

	var blocks []*ast.BlockStmt
	for _, stmt := range file.Body {
		switch stmt := stmt.(type) {
		case *ast.BlockStmt:
			blocks = append(blocks, stmt)
		default:
			diags = append(diags, diag.Diagnostic{
				Severity: diag.SeverityLevelError,
				Message:  "unexpected statement",
				StartPos: ast.StartPos(stmt).Position(),
				EndPos:   ast.EndPos(stmt).Position(),
			})
		}
	}

	return blocks, diags
}

// reproApply assembles Alloy configuration text into the Loader graph and
// returns the diagnostics produced by Apply so callers can assert a successful
// load. Note: import.* blocks are config-level blocks and must be passed via
// the config parameter so that registerImport registers the namespace.
func reproApply(t *testing.T, l *Loader, components, config, declares []byte) diag.Diagnostics {
	t.Helper()
	var diags diag.Diagnostics

	componentBlocks, parseDiags := reproFileToBlock(t, components)
	diags = append(diags, parseDiags...)
	if parseDiags.HasErrors() {
		return diags
	}
	configBlocks, parseDiags := reproFileToBlock(t, config)
	diags = append(diags, parseDiags...)
	if parseDiags.HasErrors() {
		return diags
	}
	declareBlocks, parseDiags := reproFileToBlock(t, declares)
	diags = append(diags, parseDiags...)
	if parseDiags.HasErrors() {
		return diags
	}

	l.Apply(ApplyOptions{
		ComponentBlocks: componentBlocks,
		ConfigBlocks:    configBlocks,
		DeclareBlocks:   declareBlocks,
	})
	return diags
}

// moduleDataFlowFixture is the module body: an inner passthrough component
// whose output is exposed through an export block. It is a minimized form of
// the reporter's declare "filter" module from issue #4062.
const moduleDataFlowFixture = `declare "filter" {
	testcomponents.passthrough "inner" {
		input = "from-module"
	}

	export "output" {
		value = testcomponents.passthrough.inner.output
	}
}`

// TestDataFlowEdgesCustomComponent verifies that a data flow edge is created
// between a custom component instance and its upstream consumer (regression
// test for grafana/alloy#4062).
//
// Instances of custom components defined with declare blocks or imported with
// import.* blocks were missing the data flow edge towards their upstream
// consumers, so the module instance only showed its downstream connections in
// the graph view. The module instance is the exporting side, so the correct
// edge direction is: instance -> consumer.
func TestDataFlowEdgesCustomComponent(t *testing.T) {
	tests := []struct {
		name         string
		namespaceRef string // reference path used in the consumer's input expression (also the graph node ID)
	}{
		{name: "imported module", namespaceRef: "arcm.filter.inst"},
		{name: "local declare", namespaceRef: "filter.inst"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var (
				cfg    []byte
				config []byte
			)
			if tc.name == "imported module" {
				config = []byte(`
					import.string "arcm" {
						content = ` + strconv.Quote(moduleDataFlowFixture) + `
					}
				`)
				cfg = []byte(`
					arcm.filter "inst" {}

					testcomponents.passthrough "upstream" {
						input = ` + tc.namespaceRef + `.output
					}
				`)
			} else {
				cfg = []byte(`
					filter "inst" {}

					testcomponents.passthrough "upstream" {
						input = ` + tc.namespaceRef + `.output
					}
				`)
			}

			l := newReproLoader(t)
			d := reproApply(t, l, cfg, config, []byte(moduleDataFlowFixture))
			require.False(t, d.HasErrors(), "Apply should succeed, got diagnostics: %v", d.Error())

			g := l.Graph()
			inst, ok := g.GetByID(tc.namespaceRef).(ComponentNode)
			require.True(t, ok, "node %q should exist and be a ComponentNode (got %v)", tc.namespaceRef, g.GetByID(tc.namespaceRef))
			upstream, ok := g.GetByID("testcomponents.passthrough.upstream").(ComponentNode)
			require.True(t, ok, "node testcomponents.passthrough.upstream should exist and be a ComponentNode")

			require.Contains(t, inst.GetDataFlowEdgesTo(), upstream.NodeID(),
				"module component instance should hold a data flow edge to its upstream consumer")
		})
	}
}
