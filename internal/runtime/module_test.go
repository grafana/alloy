package runtime

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
	"github.com/grafana/alloy/internal/runtime/internal/worker"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service"
)

const loggingConfig = `
	logging {}`

const tracingConfig = `
	tracing {}`

const argumentConfig = `
	argument "username" {} 
	argument "defaulted" {
		optional = true
		default = "default_value"
	}`

const argumentWithFullOptsConfig = `
	argument "foo" {
		comment = "description of foo"
		optional = true	
		default = "default_value"
	}`

const exportStringConfig = `
	export "username" {
		value = "bob"
	}`

const exportDummy = `
	export "dummy" {
		value = "bob"
	}`

const serviceConfig = `
	testservice {}`

func TestModule(t *testing.T) {
	tt := []struct {
		name                  string
		argumentModuleContent string
		args                  map[string]any
		exportModuleContent   string
		expectedExports       []string
		expectedErrorContains string
	}{
		{
			name: "Empty Content Allowed",
		},
		{
			name:                  "Bad Module",
			argumentModuleContent: `this isn't a valid module config`,
			expectedErrorContains: `expected block label, got IDENT`,
		},
		{
			name:                  "Logging blocks not allowed in module config",
			argumentModuleContent: argumentConfig + loggingConfig,
			exportModuleContent:   exportStringConfig,
			expectedErrorContains: "logging block not allowed inside a module",
		},
		{
			name:                  "Tracing blocks not allowed in module config",
			argumentModuleContent: argumentConfig + tracingConfig,
			exportModuleContent:   exportStringConfig,
			expectedErrorContains: "tracing block not allowed inside a module",
		},
		{
			name:                  "Service blocks not allowed in module config",
			argumentModuleContent: argumentConfig + serviceConfig,
			exportModuleContent:   exportStringConfig,
			expectedErrorContains: "service blocks not allowed inside a module: \"testservice\"",
		},
		{
			name:                  "Argument not defined in module source",
			argumentModuleContent: `argument "different_argument" {}`,
			exportModuleContent:   exportStringConfig,
			args:                  map[string]any{"different_argument": "test", "username": "bad"},
			expectedErrorContains: "Provided argument \"username\" is not defined in the module",
		},

		{
			name:                  "Missing required argument",
			argumentModuleContent: argumentConfig,
			exportModuleContent:   exportStringConfig,
			expectedErrorContains: "Failed to evaluate node for config block: missing required argument \"username\" to module",
		},

		{
			name:                  "Duplicate argument config",
			argumentModuleContent: argumentConfig + argumentConfig,
			exportModuleContent:   exportStringConfig,
			expectedErrorContains: "block argument.username already declared at t1:2:2",
		},
		{
			name:                  "Duplicate export config",
			argumentModuleContent: argumentConfig,
			exportModuleContent:   exportStringConfig + exportStringConfig,
			expectedErrorContains: "block export.username already declared at t1:7:2",
		},
		{
			name:                "Multiple exports but none are used but still exported",
			exportModuleContent: exportStringConfig + exportDummy,
			expectedExports:     []string{"username", "dummy"},
		},
		{
			name:                "Argument block with comment is parseable",
			exportModuleContent: argumentWithFullOptsConfig,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			defer verifyNoGoroutineLeaks(t)
			mc := newModuleController(testModuleControllerOptions(t)).(*moduleController)
			// modules do not clean up their own worker pool as we normally use a shared one from the root controller
			defer mc.o.WorkerPool.Stop(5 * time.Second)

			tm := &testModule{
				content: tc.argumentModuleContent + tc.exportModuleContent,
				args:    tc.args,
				opts:    component.Options{ModuleController: mc},
			}
			ctx, cnc := context.WithTimeout(t.Context(), 1*time.Second)
			defer cnc()
			err := tm.Run(ctx)
			if tc.expectedErrorContains == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, tc.expectedErrorContains)
			}
			for _, e := range tc.expectedExports {
				_, found := tm.exports[e]
				require.True(t, found)
			}
		})
	}
}

func TestArgsNotInModules(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	f := New(testOptions(t))
	defer cleanUpController(t.Context(), f)
	fl, err := ParseSource("test", []byte("argument \"arg\"{}"))
	require.NoError(t, err)
	err = f.LoadSource(fl, nil, "")
	require.ErrorContains(t, err, "argument blocks only allowed inside a module")
}

func TestExportsNotInModules(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	f := New(testOptions(t))
	defer cleanUpController(t.Context(), f)
	fl, err := ParseSource("test", []byte("export \"arg\"{ value = 1}"))
	require.NoError(t, err)
	err = f.LoadSource(fl, nil, "")
	require.ErrorContains(t, err, "export blocks only allowed inside a module")
}

func TestExportsWhenNotUsed(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	f := New(testOptions(t))
	content := " export \\\"username\\\"  { value  = 1 } \\n export \\\"dummy\\\" { value = 2 } "
	fullContent := "test.module \"t1\" { content = \"" + content + "\" }"
	fl, err := ParseSource("test", []byte(fullContent))
	require.NoError(t, err)
	err = f.LoadSource(fl, nil, "")
	require.NoError(t, err)
	ctx, cnc := context.WithTimeout(t.Context(), 1*time.Second)
	defer cnc()
	f.Run(ctx)
	exps := f.loader.Components()[0].Exports().(TestExports)
	for _, x := range []string{"username", "dummy"} {
		_, found := exps.Exports[x]
		require.True(t, found)
	}
}

func TestIDList(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	o := testModuleControllerOptions(t)
	defer o.WorkerPool.Stop(5 * time.Second)
	nc := newModuleController(o)
	require.Len(t, nc.ModuleIDs(), 0)

	mod1, err := nc.NewModule("t1", nil)
	require.NoError(t, err)
	ctx, cncl := context.WithCancel(t.Context())
	go func() {
		m1err := mod1.Run(ctx)
		require.NoError(t, m1err)
	}()
	require.Eventually(t, func() bool {
		return len(nc.ModuleIDs()) == 1
	}, 1*time.Second, 100*time.Millisecond)

	mod2, err := nc.NewModule("t2", nil)
	require.NoError(t, err)
	go func() {
		m2err := mod2.Run(ctx)
		require.NoError(t, m2err)
	}()
	require.Eventually(t, func() bool {
		return len(nc.ModuleIDs()) == 2
	}, 1*time.Second, 100*time.Millisecond)
	// Call cncl which will stop the run methods and remove the ids from the module controller
	cncl()
	require.Eventually(t, func() bool {
		return len(nc.ModuleIDs()) == 0
	}, 1*time.Second, 100*time.Millisecond)
}

func TestDuplicateIDList(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	o := testModuleControllerOptions(t)
	defer o.WorkerPool.Stop(5 * time.Second)
	nc := newModuleController(o)
	require.Len(t, nc.ModuleIDs(), 0)

	mod1, err := nc.NewModule("t1", nil)
	require.NoError(t, err)
	ctx, cncl := context.WithCancel(t.Context())
	defer cncl()
	go func() {
		m1err := mod1.Run(ctx)
		require.NoError(t, m1err)
	}()
	require.Eventually(t, func() bool {
		return len(nc.ModuleIDs()) == 1
	}, 5*time.Second, 100*time.Millisecond)

	// This should panic with duplicate registration.
	require.PanicsWithError(t, "duplicate metrics collector registration attempted", func() {
		_, _ = nc.NewModule("t1", nil)
	})
}

func testModuleControllerOptions(t *testing.T) *moduleControllerOptions {
	t.Helper()

	s, err := logging.New(io.Discard, logging.DefaultOptions)
	require.NoError(t, err)

	services := []service.Service{
		&testService{},
	}

	serviceMap := controller.NewServiceMap(services)

	return &moduleControllerOptions{
		Logger:         s,
		DataPath:       t.TempDir(),
		MinStability:   featuregate.StabilityPublicPreview,
		Reg:            prometheus.NewRegistry(),
		ModuleRegistry: newModuleRegistry(),
		WorkerPool:     worker.NewFixedWorkerPool(1, 100),
		ServiceMap:     serviceMap,
	}
}

func init() {
	component.Register(component.Registration{
		Name:      "test.module",
		Stability: featuregate.StabilityPublicPreview,
		Args:      TestArguments{},
		Exports:   TestExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return &testModule{
				content: args.(TestArguments).Content,
				opts:    opts,
			}, nil
		},
	})
}

type TestArguments struct {
	Content string `alloy:"content,attr"`
}

type TestExports struct {
	Exports map[string]any `alloy:"exports,attr"`
}

type testModule struct {
	content string
	args    map[string]any
	exports map[string]any
	opts    component.Options
}

func (t *testModule) Run(ctx context.Context) error {
	m, err := t.opts.ModuleController.NewModule("t1", func(exports map[string]any) {
		t.exports = exports
		if t.opts.OnStateChange == nil {
			return
		}
		t.opts.OnStateChange(TestExports{Exports: exports})
	})
	if err != nil {
		return err
	}

	err = m.LoadConfig([]byte(t.content), t.args)
	if err != nil {
		return err
	}
	m.Run(ctx)
	return nil
}

func (t *testModule) Update(_ component.Arguments) error {
	return nil
}

type testService struct{}

func (t *testService) Definition() service.Definition {
	return service.Definition{
		Name: "testservice",
	}
}

func (t *testService) Run(ctx context.Context, host service.Host) error {
	return nil
}

func (t *testService) Update(newConfig any) error {
	return nil
}

func (t *testService) Data() any {
	return nil
}
