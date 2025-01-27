package controller

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/vm"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestCreateCustomComponents(t *testing.T) {
	config := `foreach "default" {
		collection = [1, 2, 3]
		var = "num"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t))
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]interface{}))))
	customComponentIds := foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	require.ElementsMatch(t, customComponentIds, []string{"foreach_1_1", "foreach_2_1", "foreach_3_1"})
	keys := make([]string, 0, len(foreachConfigNode.customComponents))
	for key := range foreachConfigNode.customComponents {
		keys = append(keys, key)
	}
	require.ElementsMatch(t, keys, []string{"foreach_1_1", "foreach_2_1", "foreach_3_1"})
}

func TestCreateCustomComponentsDuplicatedIds(t *testing.T) {
	config := `foreach "default" {
		collection = [1, 2, 1]
		var = "num"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t))
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]interface{}))))
	customComponentIds := foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	require.ElementsMatch(t, customComponentIds, []string{"foreach_1_1", "foreach_2_1", "foreach_1_2"})
	keys := make([]string, 0, len(foreachConfigNode.customComponents))
	for key := range foreachConfigNode.customComponents {
		keys = append(keys, key)
	}
	require.ElementsMatch(t, keys, []string{"foreach_1_1", "foreach_2_1", "foreach_1_2"})
}

func TestCreateCustomComponentsWithUpdate(t *testing.T) {
	config := `foreach "default" {
		collection = [1, 2, 3]
		var = "num"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t))
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]interface{}))))
	customComponentIds := foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	require.ElementsMatch(t, customComponentIds, []string{"foreach_1_1", "foreach_2_1", "foreach_3_1"})
	keys := make([]string, 0, len(foreachConfigNode.customComponents))
	for key := range foreachConfigNode.customComponents {
		keys = append(keys, key)
	}
	require.ElementsMatch(t, keys, []string{"foreach_1_1", "foreach_2_1", "foreach_3_1"})

	newConfig := `foreach "default" {
		collection = [2, 1, 1]
		var = "num"
		template {
		}
	}`
	foreachConfigNode.moduleController.(*ModuleControllerMock).Reset()
	foreachConfigNode.UpdateBlock(getBlockFromConfig(t, newConfig))
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]interface{}))))
	customComponentIds = foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents

	// Only the 2nd "1" item in the collection is created because the two others were already created.
	require.ElementsMatch(t, customComponentIds, []string{"foreach_1_2"})

	// "foreach31" was removed, "foreach12" was added
	keys = make([]string, 0, len(foreachConfigNode.customComponents))
	for key := range foreachConfigNode.customComponents {
		keys = append(keys, key)
	}
	require.ElementsMatch(t, keys, []string{"foreach_1_1", "foreach_2_1", "foreach_1_2"})
}

func TestRunCustomComponents(t *testing.T) {
	config := `foreach "default" {
		collection = [1, 2, 3]
		var = "num"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t))
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]interface{}))))
	ctx, cancel := context.WithCancel(context.Background())
	go foreachConfigNode.Run(ctx)

	// check that all custom components are running correctly
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		for _, cc := range foreachConfigNode.customComponents {
			assert.True(c, cc.(*CustomComponentMock).IsRunning.Load())
		}
	}, 1*time.Second, 5*time.Millisecond)

	cancel()
	// check that all custom components are stopped
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		for _, cc := range foreachConfigNode.customComponents {
			assert.False(c, cc.(*CustomComponentMock).IsRunning.Load())
		}
	}, 1*time.Second, 5*time.Millisecond)
}

func TestRunCustomComponentsAfterUpdate(t *testing.T) {
	config := `foreach "default" {
		collection = [1, 2, 3]
		var = "num"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t))
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]interface{}))))
	ctx, cancel := context.WithCancel(context.Background())
	go foreachConfigNode.Run(ctx)

	// check that all custom components are running correctly
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		for _, cc := range foreachConfigNode.customComponents {
			assert.True(c, cc.(*CustomComponentMock).IsRunning.Load())
		}
	}, 1*time.Second, 5*time.Millisecond)

	newConfig := `foreach "default" {
		collection = [2, 1, 1]
		var = "num"
		template {
		}
	}`
	foreachConfigNode.moduleController.(*ModuleControllerMock).Reset()
	foreachConfigNode.UpdateBlock(getBlockFromConfig(t, newConfig))
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]interface{}))))

	newComponentIds := []string{"foreach_1_1", "foreach_2_1", "foreach_1_2"}
	// check that all new custom components are running correctly
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		for id, cc := range foreachConfigNode.customComponents {
			assert.Contains(c, newComponentIds, id)
			assert.True(c, cc.(*CustomComponentMock).IsRunning.Load())
		}
	}, 1*time.Second, 5*time.Millisecond)

	cancel()
	// check that all custom components are stopped
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		for _, cc := range foreachConfigNode.customComponents {
			assert.False(c, cc.(*CustomComponentMock).IsRunning.Load())
		}
	}, 1*time.Second, 5*time.Millisecond)
}

func TestCreateCustomComponentsCollectionObjectsWithUpdate(t *testing.T) {
	config := `foreach "default" {
		collection = [obj1, obj2]
		var = "num"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t))
	vars := map[string]interface{}{
		"obj1": map[string]string{
			"label1": "a",
			"label2": "b",
		},
		"obj2": map[string]string{
			"label3": "c",
		},
	}
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(vars)))
	customComponentIds := foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	require.ElementsMatch(t, customComponentIds, []string{"foreach_be19d02a2ccb2cbc2c47e90d_1", "foreach_b335d50e2e8490eb8bf5f51b_1"})
	keys := make([]string, 0, len(foreachConfigNode.customComponents))
	for key := range foreachConfigNode.customComponents {
		keys = append(keys, key)
	}
	require.ElementsMatch(t, keys, []string{"foreach_be19d02a2ccb2cbc2c47e90d_1", "foreach_b335d50e2e8490eb8bf5f51b_1"})

	newConfig := `foreach "default" {
		collection = [obj1, obj3]
		var = "num"
		template {
		}
	}`
	vars2 := map[string]interface{}{
		"obj1": map[string]string{
			"label1": "a",
			"label2": "b",
		},
		"obj3": map[string]string{
			"label3": "d",
		},
	}
	foreachConfigNode.moduleController.(*ModuleControllerMock).Reset()
	foreachConfigNode.UpdateBlock(getBlockFromConfig(t, newConfig))
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(vars2)))
	customComponentIds = foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents

	// Create only the custom component for the obj3 because the one for obj1 was already created
	require.ElementsMatch(t, customComponentIds, []string{"foreach_1464766cf9c8fd1095d0f7a2_1"})

	// "foreachb335d50e2e8490eb8bf5f51b1" was removed, "foreach1464766cf9c8fd1095d0f7a21" was added
	keys = make([]string, 0, len(foreachConfigNode.customComponents))
	for key := range foreachConfigNode.customComponents {
		keys = append(keys, key)
	}
	require.ElementsMatch(t, keys, []string{"foreach_be19d02a2ccb2cbc2c47e90d_1", "foreach_1464766cf9c8fd1095d0f7a2_1"})
}

func getBlockFromConfig(t *testing.T, config string) *ast.BlockStmt {
	file, err := parser.ParseFile("", []byte(config))
	require.NoError(t, err)
	return file.Body[0].(*ast.BlockStmt)
}

func getComponentGlobals(t *testing.T) ComponentGlobals {
	l, _ := logging.New(os.Stderr, logging.DefaultOptions)
	return ComponentGlobals{
		Logger:            l,
		TraceProvider:     noop.NewTracerProvider(),
		DataPath:          t.TempDir(),
		MinStability:      featuregate.StabilityGenerallyAvailable,
		OnBlockNodeUpdate: func(cn BlockNode) { /* no-op */ },
		Registerer:        prometheus.NewRegistry(),
		NewModuleController: func(id string) ModuleController {
			return NewModuleControllerMock()
		},
	}
}

type ModuleControllerMock struct {
	CustomComponents []string
}

func NewModuleControllerMock() ModuleController {
	return &ModuleControllerMock{
		CustomComponents: make([]string, 0),
	}
}

func (m *ModuleControllerMock) NewModule(id string, export component.ExportFunc) (component.Module, error) {
	return nil, nil
}

func (m *ModuleControllerMock) ModuleIDs() []string {
	return nil
}

func (m *ModuleControllerMock) NewCustomComponent(id string, export component.ExportFunc) (CustomComponent, error) {
	m.CustomComponents = append(m.CustomComponents, id)
	return &CustomComponentMock{}, nil
}

func (m *ModuleControllerMock) Reset() {
	m.CustomComponents = make([]string, 0)
}

type CustomComponentMock struct {
	IsRunning atomic.Bool
}

func (c *CustomComponentMock) LoadBody(body ast.Body, args map[string]any, customComponentRegistry *CustomComponentRegistry) error {
	return nil
}

func (c *CustomComponentMock) Run(ctx context.Context) error {
	c.IsRunning.Store(true)
	<-ctx.Done()
	c.IsRunning.Store(false)
	return nil
}
