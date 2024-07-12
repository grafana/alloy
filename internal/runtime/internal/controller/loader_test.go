package controller_test

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
	"github.com/grafana/alloy/internal/runtime/internal/dag"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	_ "github.com/grafana/alloy/internal/runtime/internal/testcomponents" // Include test components
)

func TestLoader(t *testing.T) {
	testFile := `
		testcomponents.tick "ticker" {
			frequency = "1s"
		}

		testcomponents.passthrough "static" {
			input = "hello, world!"
		}

		testcomponents.passthrough "ticker" {
			input = testcomponents.tick.ticker.tick_time
		}

		testcomponents.passthrough "forwarded" {
			input = testcomponents.passthrough.ticker.output
		}
	`

	testFileCommunity := `
		testcomponents.community "com" {}
	`

	testConfig := `
		logging {
			level = "debug"
			format = "logfmt"
		}

		tracing {
			sampling_fraction = 1
		}
	`

	// corresponds to testFile
	testGraphDefinition := graphDefinition{
		Nodes: []string{
			"testcomponents.tick.ticker",
			"testcomponents.passthrough.static",
			"testcomponents.passthrough.ticker",
			"testcomponents.passthrough.forwarded",
			"logging",
			"tracing",
		},
		OutEdges: []edge{
			{From: "testcomponents.passthrough.ticker", To: "testcomponents.tick.ticker"},
			{From: "testcomponents.passthrough.forwarded", To: "testcomponents.passthrough.ticker"},
		},
	}

	newLoaderOptionsWithStability := func(stability featuregate.Stability) controller.LoaderOptions {
		l, _ := logging.New(os.Stderr, logging.DefaultOptions)
		return controller.LoaderOptions{
			ComponentGlobals: controller.ComponentGlobals{
				Logger:            l,
				TraceProvider:     noop.NewTracerProvider(),
				DataPath:          t.TempDir(),
				MinStability:      stability,
				OnBlockNodeUpdate: func(cn controller.BlockNode) { /* no-op */ },
				Registerer:        prometheus.NewRegistry(),
				NewModuleController: func(id string) controller.ModuleController {
					return nil
				},
			},
		}
	}

	newLoaderOptions := func() controller.LoaderOptions {
		return newLoaderOptionsWithStability(featuregate.StabilityPublicPreview)
	}

	t.Run("New Graph", func(t *testing.T) {
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, []byte(testFile), []byte(testConfig), nil)
		require.NoError(t, diags.ErrorOrNil())
		requireGraph(t, l.Graph(), testGraphDefinition)
	})

	t.Run("Reload Graph New Config", func(t *testing.T) {
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, []byte(testFile), []byte(testConfig), nil)
		require.NoError(t, diags.ErrorOrNil())
		requireGraph(t, l.Graph(), testGraphDefinition)
		updatedTestConfig := `
			tracing {
				sampling_fraction = 2
			}
		`
		diags = applyFromContent(t, l, []byte(testFile), []byte(updatedTestConfig), nil)
		require.NoError(t, diags.ErrorOrNil())
		// Expect the same graph because tracing is still there and logging will be added by default.
		requireGraph(t, l.Graph(), testGraphDefinition)
	})

	t.Run("New Graph No Config", func(t *testing.T) {
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, []byte(testFile), nil, nil)
		require.NoError(t, diags.ErrorOrNil())
		requireGraph(t, l.Graph(), testGraphDefinition)
	})

	t.Run("Copy existing components and delete stale ones", func(t *testing.T) {
		startFile := `
			// Component that should be copied over to the new graph
			testcomponents.tick "ticker" {
				frequency = "1s"
			}

			// Component that will not exist in the new graph
			testcomponents.tick "remove_me" {
				frequency = "1m"
			}
		`
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, []byte(startFile), []byte(testConfig), nil)
		origGraph := l.Graph()
		require.NoError(t, diags.ErrorOrNil())

		diags = applyFromContent(t, l, []byte(testFile), []byte(testConfig), nil)
		require.NoError(t, diags.ErrorOrNil())
		newGraph := l.Graph()

		// Ensure that nodes were copied over and not recreated
		require.Equal(t, origGraph.GetByID("testcomponents.tick.ticker"), newGraph.GetByID("testcomponents.tick.ticker"))
		require.Nil(t, newGraph.GetByID("testcomponents.tick.remove_me")) // The new graph shouldn't have the old node
	})

	t.Run("Load with invalid components", func(t *testing.T) {
		invalidFile := `
			doesnotexist "bad_component" {
			}
		`
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, []byte(invalidFile), nil, nil)
		require.ErrorContains(t, diags.ErrorOrNil(), `cannot find the definition of component name "doesnotexist`)
	})

	t.Run("Load with component with empty label", func(t *testing.T) {
		invalidFile := `
			testcomponents.tick "" {
				frequency = "1s"
			}
		`
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, []byte(invalidFile), nil, nil)
		require.ErrorContains(t, diags.ErrorOrNil(), `component "testcomponents.tick" must have a label`)
	})

	t.Run("Load with correct stability level", func(t *testing.T) {
		l := controller.NewLoader(newLoaderOptionsWithStability(featuregate.StabilityPublicPreview))
		diags := applyFromContent(t, l, []byte(testFile), nil, nil)
		require.NoError(t, diags.ErrorOrNil())
	})

	t.Run("Load with below minimum stability level", func(t *testing.T) {
		l := controller.NewLoader(newLoaderOptionsWithStability(featuregate.StabilityGenerallyAvailable))
		diags := applyFromContent(t, l, []byte(testFile), nil, nil)
		require.ErrorContains(t, diags.ErrorOrNil(), "component \"testcomponents.tick\" is at stability level \"public-preview\", which is below the minimum allowed stability level \"generally-available\"")
	})

	t.Run("Load with undefined minimum stability level", func(t *testing.T) {
		l := controller.NewLoader(newLoaderOptionsWithStability(featuregate.StabilityUndefined))
		diags := applyFromContent(t, l, []byte(testFile), nil, nil)
		require.ErrorContains(t, diags.ErrorOrNil(), "stability levels must be defined: got \"public-preview\" as stability of component \"testcomponents.tick\" and <invalid_stability_level> as the minimum stability level")
	})

	t.Run("Load community component with community enabled", func(t *testing.T) {
		options := newLoaderOptions()
		options.ComponentGlobals.EnableCommunityComps = true
		l := controller.NewLoader(options)
		diags := applyFromContent(t, l, []byte(testFileCommunity), nil, nil)
		require.NoError(t, diags.ErrorOrNil())
	})

	t.Run("Load community component with community enabled and undefined stability level", func(t *testing.T) {
		options := newLoaderOptionsWithStability(featuregate.StabilityUndefined)
		options.ComponentGlobals.EnableCommunityComps = true
		l := controller.NewLoader(options)
		diags := applyFromContent(t, l, []byte(testFileCommunity), nil, nil)
		require.NoError(t, diags.ErrorOrNil())
	})

	t.Run("Load community component with community disabled", func(t *testing.T) {
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, []byte(testFileCommunity), nil, nil)
		require.ErrorContains(t, diags.ErrorOrNil(), "the component \"testcomponents.community\" is a community component. Use the --feature.community-components.enabled command-line flag to enable community components")
	})

	t.Run("Partial load with invalid reference", func(t *testing.T) {
		invalidFile := `
			testcomponents.tick "ticker" {
				frequency = "1s"
			}

			testcomponents.passthrough "valid" {
				input = testcomponents.tick.ticker.tick_time
			}

			testcomponents.passthrough "invalid" {
				input = testcomponents.tick.doesnotexist.tick_time
			}
		`
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, []byte(invalidFile), nil, nil)
		require.Error(t, diags.ErrorOrNil())

		requireGraph(t, l.Graph(), graphDefinition{
			Nodes:    nil,
			OutEdges: nil,
		})
	})

	t.Run("File has cycles", func(t *testing.T) {
		invalidFile := `
			testcomponents.tick "ticker" {
				frequency = "1s"
			}

			testcomponents.passthrough "static" {
				input = testcomponents.passthrough.forwarded.output
			}

			testcomponents.passthrough "ticker" {
				input = testcomponents.passthrough.static.output
			}

			testcomponents.passthrough "forwarded" {
				input = testcomponents.passthrough.ticker.output
			}
		`
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, []byte(invalidFile), nil, nil)
		require.Error(t, diags.ErrorOrNil())
	})

	t.Run("Config block redefined", func(t *testing.T) {
		invalidFile := `
			logging {}
			logging {}
		`
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, nil, []byte(invalidFile), nil)
		require.ErrorContains(t, diags.ErrorOrNil(), `block logging already declared at TestLoader/Config_block_redefined:2:4`)
	})

	t.Run("Config block redefined after reload", func(t *testing.T) {
		file := `
			logging {}
		`
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, nil, []byte(file), nil)
		require.NoError(t, diags.ErrorOrNil())
		invalidFile := `
			logging {}
			logging {}
		`
		diags = applyFromContent(t, l, nil, []byte(invalidFile), nil)
		require.ErrorContains(t, diags.ErrorOrNil(), `block logging already declared at TestLoader/Config_block_redefined_after_reload:2:4`)
	})

	t.Run("Component block redefined", func(t *testing.T) {
		invalidFile := `
			testcomponents.tick "ticker" {
				frequency = "1s"
			}
			testcomponents.tick "ticker" {
				frequency = "1s"
			}
		`
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, []byte(invalidFile), nil, nil)
		require.ErrorContains(t, diags.ErrorOrNil(), `block testcomponents.tick.ticker already declared at TestLoader/Component_block_redefined:2:4`)
	})

	t.Run("Component block redefined after reload", func(t *testing.T) {
		file := `
			testcomponents.tick "ticker" {
				frequency = "1s"
			}
		`
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, []byte(file), nil, nil)
		require.NoError(t, diags.ErrorOrNil())
		invalidFile := `
			testcomponents.tick "ticker" {
				frequency = "1s"
			}
			testcomponents.tick "ticker" {
				frequency = "1s"
			}
		`
		diags = applyFromContent(t, l, []byte(invalidFile), nil, nil)
		require.ErrorContains(t, diags.ErrorOrNil(), `block testcomponents.tick.ticker already declared at TestLoader/Component_block_redefined_after_reload:2:4`)
	})

	t.Run("Declare block redefined", func(t *testing.T) {
		invalidFile := `
			declare "a" {}
			declare "a" {}
		`
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, nil, nil, []byte(invalidFile))
		require.ErrorContains(t, diags.ErrorOrNil(), `block declare.a already declared at TestLoader/Declare_block_redefined:2:4`)
	})

	t.Run("Declare block redefined after reload", func(t *testing.T) {
		file := `
			declare "a" {}
		`
		l := controller.NewLoader(newLoaderOptions())
		diags := applyFromContent(t, l, nil, nil, []byte(file))
		require.NoError(t, diags.ErrorOrNil())
		invalidFile := `
			declare "a" {}
			declare "a" {}
		`
		diags = applyFromContent(t, l, nil, nil, []byte(invalidFile))
		require.ErrorContains(t, diags.ErrorOrNil(), `block declare.a already declared at TestLoader/Declare_block_redefined_after_reload:2:4`)
	})
}

func TestLoader_Services(t *testing.T) {
	testFile := `
		testsvc { }
	`

	testService := &fakeService{
		DefinitionFunc: func() service.Definition {
			return service.Definition{
				Name: "testsvc",
				ConfigType: struct {
					Name string `alloy:"name,attr,optional"`
				}{},
				Stability: featuregate.StabilityPublicPreview,
			}
		},
	}

	newLoaderOptionsWithStability := func(stability featuregate.Stability) controller.LoaderOptions {
		l, _ := logging.New(os.Stderr, logging.DefaultOptions)
		return controller.LoaderOptions{
			ComponentGlobals: controller.ComponentGlobals{
				Logger:            l,
				TraceProvider:     noop.NewTracerProvider(),
				DataPath:          t.TempDir(),
				MinStability:      stability,
				OnBlockNodeUpdate: func(cn controller.BlockNode) { /* no-op */ },
				Registerer:        prometheus.NewRegistry(),
				NewModuleController: func(id string) controller.ModuleController {
					return nil
				},
			},
			Services: []service.Service{testService},
		}
	}

	t.Run("Load with service at correct stability level", func(t *testing.T) {
		l := controller.NewLoader(newLoaderOptionsWithStability(featuregate.StabilityPublicPreview))
		diags := applyFromContent(t, l, []byte(testFile), nil, nil)
		require.NoError(t, diags.ErrorOrNil())
	})

	t.Run("Load with service below minimum stabilty level", func(t *testing.T) {
		l := controller.NewLoader(newLoaderOptionsWithStability(featuregate.StabilityGenerallyAvailable))
		diags := applyFromContent(t, l, []byte(testFile), nil, nil)
		require.ErrorContains(t, diags.ErrorOrNil(), `block "testsvc" is at stability level "public-preview", which is below the minimum allowed stability level "generally-available"`)
	})

	t.Run("Load with undefined minimum stability level", func(t *testing.T) {
		l := controller.NewLoader(newLoaderOptionsWithStability(featuregate.StabilityUndefined))
		diags := applyFromContent(t, l, []byte(testFile), nil, nil)
		require.ErrorContains(t, diags.ErrorOrNil(), `stability levels must be defined: got "public-preview" as stability of block "testsvc" and <invalid_stability_level> as the minimum stability level`)
	})
}

// TestScopeWithFailingComponent is used to ensure that the scope is filled out, even if the component
// fails to properly start.
func TestScopeWithFailingComponent(t *testing.T) {
	testFile := `
		testcomponents.tick "ticker" {
			frequenc = "1s"
		}

		testcomponents.passthrough "static" {
			input = "hello, world!"
		}

		testcomponents.passthrough "ticker" {
			input = testcomponents.tick.ticker.tick_time
		}

		testcomponents.passthrough "forwarded" {
			input = testcomponents.passthrough.ticker.output
		}
	`
	newLoaderOptions := func() controller.LoaderOptions {
		l, _ := logging.New(os.Stderr, logging.DefaultOptions)
		return controller.LoaderOptions{
			ComponentGlobals: controller.ComponentGlobals{
				Logger:            l,
				TraceProvider:     noop.NewTracerProvider(),
				DataPath:          t.TempDir(),
				MinStability:      featuregate.StabilityPublicPreview,
				OnBlockNodeUpdate: func(cn controller.BlockNode) { /* no-op */ },
				Registerer:        prometheus.NewRegistry(),
				NewModuleController: func(id string) controller.ModuleController {
					return fakeModuleController{}
				},
			},
		}
	}

	l := controller.NewLoader(newLoaderOptions())
	diags := applyFromContent(t, l, []byte(testFile), nil, nil)
	require.Error(t, diags.ErrorOrNil())
	require.Len(t, diags, 1)
	require.True(t, strings.Contains(diags.Error(), `unrecognized attribute name "frequenc"`))
}

func applyFromContent(t *testing.T, l *controller.Loader, componentBytes []byte, configBytes []byte, declareBytes []byte) diag.Diagnostics {
	t.Helper()

	var (
		diags           diag.Diagnostics
		componentBlocks []*ast.BlockStmt
		configBlocks    []*ast.BlockStmt = nil
		declareBlocks   []*ast.BlockStmt = nil
	)

	componentBlocks, diags = fileToBlock(t, componentBytes)
	if diags.HasErrors() {
		return diags
	}

	if string(configBytes) != "" {
		configBlocks, diags = fileToBlock(t, configBytes)
		if diags.HasErrors() {
			return diags
		}
	}

	if string(declareBytes) != "" {
		declareBlocks, diags = fileToBlock(t, declareBytes)
		if diags.HasErrors() {
			return diags
		}
	}

	applyOptions := controller.ApplyOptions{
		ComponentBlocks: componentBlocks,
		ConfigBlocks:    configBlocks,
		DeclareBlocks:   declareBlocks,
	}

	applyDiags := l.Apply(applyOptions)
	diags = append(diags, applyDiags...)

	return diags
}

func fileToBlock(t *testing.T, bytes []byte) ([]*ast.BlockStmt, diag.Diagnostics) {
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

type graphDefinition struct {
	Nodes    []string
	OutEdges []edge
}

type edge struct{ From, To string }

func requireGraph(t *testing.T, g *dag.Graph, expect graphDefinition) {
	t.Helper()

	var (
		actualNodes []string
		actualEdges []edge
	)

	for _, n := range g.Nodes() {
		actualNodes = append(actualNodes, n.NodeID())
	}
	require.ElementsMatch(t, expect.Nodes, actualNodes, "List of nodes do not match")

	for _, e := range g.Edges() {
		actualEdges = append(actualEdges, edge{
			From: e.From.NodeID(),
			To:   e.To.NodeID(),
		})
	}
	require.ElementsMatch(t, expect.OutEdges, actualEdges, "List of edges do not match")
}

type fakeModuleController struct{}

func (f fakeModuleController) NewModule(id string, export component.ExportFunc) (component.Module, error) {
	return nil, nil
}

func (f fakeModuleController) ModuleIDs() []string {
	return nil
}

func (f fakeModuleController) ClearModuleIDs() {
}

func (f fakeModuleController) NewCustomComponent(id string, export component.ExportFunc) (controller.CustomComponent, error) {
	return nil, nil
}

type fakeService struct {
	DefinitionFunc func() service.Definition // Required.
	RunFunc        func(ctx context.Context, host service.Host) error
	UpdateFunc     func(newConfig any) error
	DataFunc       func() any
}

func (fs *fakeService) Definition() service.Definition {
	return fs.DefinitionFunc()
}

func (fs *fakeService) Run(ctx context.Context, host service.Host) error {
	if fs.RunFunc != nil {
		return fs.RunFunc(ctx, host)
	}

	<-ctx.Done()
	return nil
}

func (fs *fakeService) Update(newConfig any) error {
	if fs.UpdateFunc != nil {
		return fs.UpdateFunc(newConfig)
	}
	return nil
}

func (fs *fakeService) Data() any {
	if fs.DataFunc != nil {
		return fs.DataFunc()
	}
	return nil
}
