package runtime

import (
	"context"
	"os"
	"testing"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/internal/controller"
	"github.com/grafana/alloy/internal/runtime/internal/dag"
	"github.com/grafana/alloy/internal/runtime/internal/testcomponents"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

var testFile = `
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

func TestController_LoadSource_Evaluation(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	ctrl := New(testOptions(t))
	defer cleanUpController(ctrl)

	// Use testFile from graph_builder_test.go.
	f, err := ParseSource(t.Name(), []byte(testFile))
	require.NoError(t, err)
	require.NotNil(t, f)

	err = ctrl.LoadSource(f, nil, "")
	require.NoError(t, err)
	require.Len(t, ctrl.loader.Components(), 4)

	// Check the inputs and outputs of things that should be immediately resolved
	// without having to run the components.
	in, out := getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.static")
	require.Equal(t, "hello, world!", in.(testcomponents.PassthroughConfig).Input)
	require.Equal(t, "hello, world!", out.(testcomponents.PassthroughExports).Output)
}

var modulePathTestFile = `
	testcomponents.tick "ticker" {
		frequency = "1s"
	}
	testcomponents.passthrough "static" {
		input = module_path
	}
	testcomponents.passthrough "ticker" {
		input = testcomponents.tick.ticker.tick_time
	}
	testcomponents.passthrough "forwarded" {
		input = testcomponents.passthrough.ticker.output
	}
`

func TestController_LoadSource_WithModulePath_Evaluation(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	ctrl := New(testOptions(t))
	defer cleanUpController(ctrl)

	f, err := ParseSource(t.Name(), []byte(modulePathTestFile))
	require.NoError(t, err)
	require.NotNil(t, f)

	filePath := "tmp_modulePath_test/test/main.alloy"
	require.NoError(t, os.Mkdir("tmp_modulePath_test", 0700))
	require.NoError(t, os.Mkdir("tmp_modulePath_test/test", 0700))
	defer os.RemoveAll("tmp_modulePath_test")
	require.NoError(t, os.WriteFile(filePath, []byte(""), 0664))

	err = ctrl.LoadSource(f, nil, filePath)
	require.NoError(t, err)
	require.Len(t, ctrl.loader.Components(), 4)

	// Check the inputs and outputs of things that should be immediately resolved
	// without having to run the components.
	in, out := getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.static")
	require.Equal(t, "tmp_modulePath_test/test", in.(testcomponents.PassthroughConfig).Input)
	require.Equal(t, "tmp_modulePath_test/test", out.(testcomponents.PassthroughExports).Output)
}

func TestController_LoadSource_WithModulePathWithoutFileExtension_Evaluation(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)
	ctrl := New(testOptions(t))
	defer cleanUpController(ctrl)

	f, err := ParseSource(t.Name(), []byte(modulePathTestFile))
	require.NoError(t, err)
	require.NotNil(t, f)

	filePath := "tmp_modulePath_test/test/main"
	require.NoError(t, os.Mkdir("tmp_modulePath_test", 0700))
	require.NoError(t, os.Mkdir("tmp_modulePath_test/test", 0700))
	defer os.RemoveAll("tmp_modulePath_test")
	require.NoError(t, os.WriteFile(filePath, []byte(""), 0664))

	err = ctrl.LoadSource(f, nil, filePath)
	require.NoError(t, err)
	require.Len(t, ctrl.loader.Components(), 4)

	// Check the inputs and outputs of things that should be immediately resolved
	// without having to run the components.
	in, out := getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.static")
	require.Equal(t, "tmp_modulePath_test/test", in.(testcomponents.PassthroughConfig).Input)
	require.Equal(t, "tmp_modulePath_test/test", out.(testcomponents.PassthroughExports).Output)
}

func getFields(t *testing.T, g *dag.Graph, nodeID string) (component.Arguments, component.Exports) {
	t.Helper()

	n := g.GetByID(nodeID)
	require.NotNil(t, n, "couldn't find node %q in graph", nodeID)

	uc := n.(*controller.BuiltinComponentNode)
	return uc.Arguments(), uc.Exports()
}

func testOptions(t *testing.T) Options {
	t.Helper()

	s, err := logging.New(os.Stderr, logging.DefaultOptions)
	require.NoError(t, err)

	return Options{
		Logger:       s,
		DataPath:     t.TempDir(),
		MinStability: featuregate.StabilityPublicPreview,
		Reg:          nil,
	}
}

func cleanUpController(ctrl *Runtime) {
	// To avoid leaking goroutines and clean-up, we need to run and shut down the controller.
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ctrl.Run(ctx)
		close(done)
	}()
	cancel()
	<-done
}

func verifyNoGoroutineLeaks(t *testing.T) {
	t.Helper()
	goleak.VerifyNone(
		t,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("go.opentelemetry.io/otel/sdk/trace.(*batchSpanProcessor).processQueue"),
	)
}
