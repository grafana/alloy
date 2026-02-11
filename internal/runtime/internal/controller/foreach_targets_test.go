// controller_test package helps avoid a circular dependency when testing using discovery.Target capsules.
package controller_test

import (
	"context"
	"os"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/vm"
)

func TestForeachCollectionTargetsUsesId(t *testing.T) {
	config := `foreach "default" {
		collection = targets
		var = "each"
		id = "selected_id"
		template {
		}
	}`
	moduleController := &moduleControllerStub{}
	foreachConfigNode := controller.NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t, moduleController), nil)
	vars := map[string]any{
		"targets": []discovery.Target{
			discovery.NewTargetFromMap(map[string]string{
				"__address__": "192.0.2.10",
				"selected_id": "8201",
				"instance":    "192.0.2.10",
			}),
			discovery.NewTargetFromMap(map[string]string{
				"__address__": "198.51.100.24",
				"selected_id": "8202",
				"instance":    "198.51.100.24",
			}),
		},
	}
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(vars)))
	require.ElementsMatch(t, []string{"foreach_8201_1", "foreach_8202_1"}, moduleController.customComponents)
}

func getBlockFromConfig(t *testing.T, config string) *ast.BlockStmt {
	file, err := parser.ParseFile("", []byte(config))
	require.NoError(t, err)
	return file.Body[0].(*ast.BlockStmt)
}

func getComponentGlobals(t *testing.T, moduleController controller.ModuleController) controller.ComponentGlobals {
	l, _ := logging.New(os.Stderr, logging.DefaultOptions)
	return controller.ComponentGlobals{
		Logger:            l,
		TraceProvider:     noop.NewTracerProvider(),
		DataPath:          t.TempDir(),
		MinStability:      featuregate.StabilityGenerallyAvailable,
		OnBlockNodeUpdate: func(cn controller.BlockNode) { /* no-op */ },
		Registerer:        prometheus.NewRegistry(),
		NewModuleController: func(opts controller.ModuleControllerOpts) controller.ModuleController {
			return moduleController
		},
	}
}

type moduleControllerStub struct {
	customComponents []string
}

func (m *moduleControllerStub) NewModule(id string, export component.ExportFunc) (component.Module, error) {
	return nil, nil
}

func (m *moduleControllerStub) ModuleIDs() []string {
	return nil
}

func (m *moduleControllerStub) NewCustomComponent(id string, export component.ExportFunc) (controller.CustomComponent, error) {
	m.customComponents = append(m.customComponents, id)
	return &customComponentStub{}, nil
}

type customComponentStub struct{}

func (c *customComponentStub) LoadBody(body ast.Body, args map[string]any, customComponentRegistry *controller.CustomComponentRegistry) error {
	return nil
}

func (c *customComponentStub) Run(ctx context.Context) error {
	return nil
}
