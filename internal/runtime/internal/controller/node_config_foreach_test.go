package controller

import (
	"context"
	"os"
	"testing"
	"time"

	"go.uber.org/atomic"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/trace/noop"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/parser"
	"github.com/grafana/alloy/syntax/vm"
)

func TestCreateCustomComponents(t *testing.T) {
	config := `foreach "default" {
		collection = [1, 2, 3]
		var = "num"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))))
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
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))))
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
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))))
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
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))))
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
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))))
	ctx, cancel := context.WithCancel(t.Context())
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
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))))
	ctx, cancel := context.WithCancel(t.Context())
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
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))))

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
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	vars := map[string]any{
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
	require.ElementsMatch(t, customComponentIds, []string{"foreach_be19d02a2ccb2cbc2c47e90dcad8446a50459577449624176398d1f2aa6cd23a_1", "foreach_b335d50e2e8490eb8bf5f51b3ca8b1599d811514ca40d28ada5214294d49752d_1"})
	keys := make([]string, 0, len(foreachConfigNode.customComponents))
	for key := range foreachConfigNode.customComponents {
		keys = append(keys, key)
	}
	require.ElementsMatch(t, keys, []string{"foreach_be19d02a2ccb2cbc2c47e90dcad8446a50459577449624176398d1f2aa6cd23a_1", "foreach_b335d50e2e8490eb8bf5f51b3ca8b1599d811514ca40d28ada5214294d49752d_1"})

	newConfig := `foreach "default" {
		collection = [obj1, obj3]
		var = "num"
		template {
		}
	}`
	vars2 := map[string]any{
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
	require.ElementsMatch(t, customComponentIds, []string{"foreach_1464766cf9c8fd1095d0f7a22abe0632b6a6d44c3eeae65766086350eef3ac33_1"})

	// "foreach_b335d50e2e8490eb8bf5f51b3ca8b1599d811514ca40d28ada5214294d49752d_1" was removed, "foreach_1464766cf9c8fd1095d0f7a22abe0632b6a6d44c3eeae65766086350eef3ac33_1" was added
	keys = make([]string, 0, len(foreachConfigNode.customComponents))
	for key := range foreachConfigNode.customComponents {
		keys = append(keys, key)
	}
	require.ElementsMatch(t, keys, []string{"foreach_be19d02a2ccb2cbc2c47e90dcad8446a50459577449624176398d1f2aa6cd23a_1", "foreach_1464766cf9c8fd1095d0f7a22abe0632b6a6d44c3eeae65766086350eef3ac33_1"})
}

func TestNonAlphaNumericString(t *testing.T) {
	config := `foreach "default" {
		collection = ["123./st%4$"]
		var = "num"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))))
	customComponentIds := foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	require.ElementsMatch(t, customComponentIds, []string{"foreach_123__st_4__1"})
}

func TestNonAlphaNumericString2(t *testing.T) {
	// All non-alphanumeric characters are replaced with "_".
	// This test uses two different strings that will be normalized to the same string.
	// Both "123./s4" and "123/.s4" will become "123__s4".
	// We expect this to be ok - the controller will name one of them "123__s4_1", and the other "123__s4_2"
	config := `foreach "default" {
		collection = ["123./s4", "123/.s4"]
		var = "num"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))))
	customComponentIds := foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	require.ElementsMatch(t, customComponentIds, []string{"foreach_123__s4_1", "foreach_123__s4_2"})
}

func TestNonAlphaNumericString3(t *testing.T) {
	// The "123./s4" non-alphanumeric string should normally be converted into "foreach_123__s4_1".
	// However, there is already a "foreach_123__s4_1".
	// We expect the controller to avoid such name collisions.
	config := `foreach "default" {
		collection = ["123./s4", "123__s4_1"]
		var = "num"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))))
	customComponentIds := foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	// TODO: It's not very clear which item became "foreach_123__s4_1_1".
	// To avoid confusion, maybe we should log a mapping?
	require.ElementsMatch(t, customComponentIds, []string{"foreach_123__s4_1", "foreach_123__s4_1_1"})
}

func TestStringIDHash(t *testing.T) {
	config := `foreach "default" {
		collection = ["123./st%4$"]
		var = "num"
		hash_string_id = true
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))))
	customComponentIds := foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	require.ElementsMatch(t, customComponentIds, []string{"foreach_1951d330e1267d082c816bfb3f40cce6eb9a8da9f6a6b9da09ace3c6514361cd_1"})
}

func TestStringIDHashWithKey(t *testing.T) {
	config := `foreach "default" {
		collection = [obj1, obj2]
		var = "num"
		hash_string_id = true
		id = "label1"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	vars := map[string]any{
		"obj1": map[string]string{
			"label1": "123./st%4$",
			"label2": "b",
		},
		"obj2": map[string]string{
			"label1": "aaaaaaaaaaaaaaabbbbbbbbbcccccccccdddddddddeeeeeeeeeffffffffffgggggggggggghhhhhhhhhhiiiiiiiiiiijjjjjjjjjjjkkkkkkkkkklllll",
		},
	}
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(vars)))
	customComponentIds := foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	require.ElementsMatch(t, customComponentIds, []string{"foreach_1951d330e1267d082c816bfb3f40cce6eb9a8da9f6a6b9da09ace3c6514361cd_1", "foreach_986cb398ec7d2d70a69bab597e8525ccc5c67594765a82ee7d0f011937cdec25_1"})
}

func TestStringIDHashWithKeySameValue(t *testing.T) {
	config := `foreach "default" {
		collection = [obj1, obj2]
		var = "num"
		hash_string_id = true
		id = "label1"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	vars := map[string]any{
		"obj1": map[string]string{
			"label1": "123./st%4$",
			"label2": "b",
		},
		"obj2": map[string]string{
			"label1": "123./st%4$",
		},
	}
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(vars)))
	customComponentIds := foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	require.ElementsMatch(t, customComponentIds, []string{"foreach_1951d330e1267d082c816bfb3f40cce6eb9a8da9f6a6b9da09ace3c6514361cd_1", "foreach_1951d330e1267d082c816bfb3f40cce6eb9a8da9f6a6b9da09ace3c6514361cd_2"})
}

func TestForeachCollectionMapAnyUsesId(t *testing.T) {
	config := `foreach "default" {
		collection = [obj1, obj2]
		var = "each"
		id = "selected_id"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	vars := map[string]any{
		"obj1": map[string]any{
			"selected_id": "9101",
		},
		"obj2": map[string]any{
			"selected_id": "9102",
		},
	}
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(vars)))
	customComponentIds := foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	require.ElementsMatch(t, customComponentIds, []string{"foreach_9101_1", "foreach_9102_1"})
}

func TestForeachCollectionSyntaxValueUsesId(t *testing.T) {
	config := `foreach "default" {
		collection = [obj1]
		var = "each"
		id = "selected_id"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	vars := map[string]any{
		"obj1": map[string]syntax.Value{
			"selected_id": syntax.ValueFromString("9103"),
		},
	}
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(vars)))
	customComponentIds := foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	require.ElementsMatch(t, customComponentIds, []string{"foreach_9103_1"})
}

func TestCollectionNonArrayValue(t *testing.T) {
	config := `foreach "default" {
		collection = "aaa"
		var = "num"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	require.ErrorContains(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))), `"aaa" should be array, got string`)
}

func TestModuleControllerUpdate(t *testing.T) {
	config := `foreach "default" {
		collection = [1, 2, 3]
		var = "num"
		template {
		}
	}`
	foreachConfigNode := NewForeachConfigNode(getBlockFromConfig(t, config), getComponentGlobals(t), nil)
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))))
	customComponentIds := foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	require.ElementsMatch(t, customComponentIds, []string{"foreach_1_1", "foreach_2_1", "foreach_3_1"})

	// Re-evaluate, the module controller should still contain the same custom components
	require.NoError(t, foreachConfigNode.Evaluate(vm.NewScope(make(map[string]any))))
	customComponentIds = foreachConfigNode.moduleController.(*ModuleControllerMock).CustomComponents
	require.ElementsMatch(t, customComponentIds, []string{"foreach_1_1", "foreach_2_1", "foreach_3_1"})
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
		NewModuleController: func(opts ModuleControllerOpts) ModuleController {
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
