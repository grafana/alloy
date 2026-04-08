package runtime

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/runtime/internal/testcomponents"
	"github.com/grafana/alloy/internal/runtime/internal/worker"
	"github.com/stretchr/testify/require"
)

func TestController_Updates(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)

	// Simple pipeline with a minimal lag
	config := `
	testcomponents.count "inc" {
		frequency = "10ms"
		max = 10
	}

	testcomponents.passthrough "inc_dep_1" {
		input = testcomponents.count.inc.count
		lag = "1ms"
	}

	testcomponents.passthrough "inc_dep_2" {
		input = testcomponents.passthrough.inc_dep_1.output
		lag = "1ms"
	}

	testcomponents.summation "sum" {
		input = testcomponents.passthrough.inc_dep_2.output
	}
`

	ctrl := newTestController(t)

	// Use testUpdatesFile from graph_builder_test.go.
	f, err := ParseSource(t.Name(), []byte(config))
	require.NoError(t, err)
	require.NotNil(t, f)

	err = ctrl.LoadSource(f, nil, "")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		ctrl.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	// Wait for the updates to propagate
	require.Eventually(t, func() bool {
		_, out := getFields(t, ctrl.loader.Graph(), "testcomponents.summation.sum")
		return out.(testcomponents.SummationExports).LastAdded == 10
	}, 3*time.Second, 10*time.Millisecond)

	in, out := getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.inc_dep_1")
	require.Equal(t, "10", in.(testcomponents.PassthroughConfig).Input)
	require.Equal(t, "10", out.(testcomponents.PassthroughExports).Output)

	in, out = getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.inc_dep_2")
	require.Equal(t, "10", in.(testcomponents.PassthroughConfig).Input)
	require.Equal(t, "10", out.(testcomponents.PassthroughExports).Output)

	in, _ = getFields(t, ctrl.loader.Graph(), "testcomponents.summation.sum")
	require.Equal(t, 10, in.(testcomponents.SummationConfig).Input)
}

func TestController_Updates_WithQueueFull(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)

	// Simple pipeline with a minimal lag with one node having 3 direct dependencies and one misbehaving node.
	config := `
	testcomponents.count "inc" {
		frequency = "10ms"
		max = 10
	}

	testcomponents.passthrough "misbehaving_slow" {
		input = testcomponents.count.inc.count
		lag = "100ms"
	}

	testcomponents.passthrough "inc_dep_1" {
		input = testcomponents.count.inc.count
		lag = "1ms"
	}

	testcomponents.passthrough "inc_dep_2" {
		input = testcomponents.count.inc.count
		lag = "1ms"
	}

	testcomponents.passthrough "inc_dep_3" {
		input = testcomponents.count.inc.count
		lag = "1ms"
	}

	testcomponents.summation "sum" {
		input = testcomponents.passthrough.inc_dep_3.output
	}
`

	ctrl, err := newController(controllerOptions{
		Options:        testOptions(t),
		ModuleRegistry: newModuleRegistry(),
		IsModule:       false,
		// The small number of workers and small queue means that a lot of updates will need to be retried.
		WorkerPool: worker.NewFixedWorkerPool(1, 1),
	})
	require.NoError(t, err)

	// Use testUpdatesFile from graph_builder_test.go.
	f, err := ParseSource(t.Name(), []byte(config))
	require.NoError(t, err)
	require.NotNil(t, f)

	err = ctrl.LoadSource(f, nil, "")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())

	done := make(chan struct{})
	go func() {
		ctrl.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	// Wait for the updates to propagate
	require.Eventually(t, func() bool {
		_, out := getFields(t, ctrl.loader.Graph(), "testcomponents.summation.sum")
		return out.(testcomponents.SummationExports).LastAdded == 10
	}, 3*time.Second, 10*time.Millisecond)

	in, _ := getFields(t, ctrl.loader.Graph(), "testcomponents.summation.sum")
	require.Equal(t, 10, in.(testcomponents.SummationConfig).Input)

	in, out := getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.inc_dep_3")
	require.Equal(t, "10", in.(testcomponents.PassthroughConfig).Input)
	require.Equal(t, "10", out.(testcomponents.PassthroughExports).Output)

	// The dep_2 is independent of sum and dep_3, so we check for it with eventually.
	require.Eventually(t, func() bool {
		_, out := getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.inc_dep_2")
		return out.(testcomponents.PassthroughExports).Output == "10"
	}, 3*time.Second, 10*time.Millisecond)

	// Similar for dep_1
	require.Eventually(t, func() bool {
		_, out := getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.inc_dep_1")
		return out.(testcomponents.PassthroughExports).Output == "10"
	}, 3*time.Second, 10*time.Millisecond)
}

func TestController_Updates_WithLag(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)

	// Simple pipeline with some lag
	config := `
	testcomponents.count "inc" {
		frequency = "10ms"
		max = 10
	}

	testcomponents.passthrough "inc_dep_1" {
		input = testcomponents.count.inc.count
		lag = "10ms"
	}

	testcomponents.passthrough "inc_dep_2" {
		input = testcomponents.passthrough.inc_dep_1.output
		lag = "10ms"
	}

	testcomponents.summation "sum" {
		input = testcomponents.passthrough.inc_dep_2.output
	}
`

	ctrl := newTestController(t)

	// Use testUpdatesFile from graph_builder_test.go.
	f, err := ParseSource(t.Name(), []byte(config))
	require.NoError(t, err)
	require.NotNil(t, f)

	err = ctrl.LoadSource(f, nil, "")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		ctrl.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	// Wait for the updates to propagate
	require.Eventually(t, func() bool {
		_, out := getFields(t, ctrl.loader.Graph(), "testcomponents.summation.sum")
		return out.(testcomponents.SummationExports).LastAdded == 10
	}, 3*time.Second, 10*time.Millisecond)

	in, out := getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.inc_dep_1")
	require.Equal(t, "10", in.(testcomponents.PassthroughConfig).Input)
	require.Equal(t, "10", out.(testcomponents.PassthroughExports).Output)

	in, out = getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.inc_dep_2")
	require.Equal(t, "10", in.(testcomponents.PassthroughConfig).Input)
	require.Equal(t, "10", out.(testcomponents.PassthroughExports).Output)

	in, _ = getFields(t, ctrl.loader.Graph(), "testcomponents.summation.sum")
	require.Equal(t, 10, in.(testcomponents.SummationConfig).Input)
}

func TestController_Updates_WithOtherLaggingPipeline(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)

	// Another pipeline exists with a significant lag.
	config := `
	testcomponents.count "inc" {
		frequency = "10ms"
		max = 10
	}

	testcomponents.passthrough "inc_dep_1" {
		input = testcomponents.count.inc.count
		lag = "1ms"
	}

	testcomponents.passthrough "inc_dep_2" {
		input = testcomponents.passthrough.inc_dep_1.output
		lag = "1ms"
	}

	testcomponents.summation "sum" {
		input = testcomponents.passthrough.inc_dep_2.output
	}

	testcomponents.count "inc_2" {
		frequency = "10ms"
		max = 10
	}

	testcomponents.passthrough "inc_dep_slow" {
		input = testcomponents.count.inc_2.count
		lag = "500ms"
	}
`

	ctrl := newTestController(t)

	// Use testUpdatesFile from graph_builder_test.go.
	f, err := ParseSource(t.Name(), []byte(config))
	require.NoError(t, err)
	require.NotNil(t, f)

	err = ctrl.LoadSource(f, nil, "")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		ctrl.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	// Wait for the updates to propagate
	require.Eventually(t, func() bool {
		_, out := getFields(t, ctrl.loader.Graph(), "testcomponents.summation.sum")
		return out.(testcomponents.SummationExports).LastAdded == 10
	}, 2*time.Second, 10*time.Millisecond)

	in, out := getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.inc_dep_1")
	require.Equal(t, "10", in.(testcomponents.PassthroughConfig).Input)
	require.Equal(t, "10", out.(testcomponents.PassthroughExports).Output)

	in, out = getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.inc_dep_2")
	require.Equal(t, "10", in.(testcomponents.PassthroughConfig).Input)
	require.Equal(t, "10", out.(testcomponents.PassthroughExports).Output)

	in, _ = getFields(t, ctrl.loader.Graph(), "testcomponents.summation.sum")
	require.Equal(t, 10, in.(testcomponents.SummationConfig).Input)
}

func TestController_Updates_WithLaggingComponent(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)

	// Part of the pipeline has a significant lag.
	config := `
	testcomponents.count "inc" {
		frequency = "10ms"
		max = 10
	}

	testcomponents.passthrough "inc_dep_1" {
		input = testcomponents.count.inc.count
		lag = "1ms"
	}

	testcomponents.passthrough "inc_dep_2" {
		input = testcomponents.passthrough.inc_dep_1.output
		lag = "1ms"
	}

	testcomponents.summation "sum" {
		input = testcomponents.passthrough.inc_dep_2.output
	}

	testcomponents.passthrough "inc_dep_slow" {
		input = testcomponents.count.inc.count
		lag = "500ms"
	}
`

	ctrl := newTestController(t)

	// Use testUpdatesFile from graph_builder_test.go.
	f, err := ParseSource(t.Name(), []byte(config))
	require.NoError(t, err)
	require.NotNil(t, f)

	err = ctrl.LoadSource(f, nil, "")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		ctrl.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	// Wait for the updates to propagate
	require.Eventually(t, func() bool {
		_, out := getFields(t, ctrl.loader.Graph(), "testcomponents.summation.sum")
		return out.(testcomponents.SummationExports).LastAdded == 10
	}, 2*time.Second, 10*time.Millisecond)

	in, out := getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.inc_dep_1")
	require.Equal(t, "10", in.(testcomponents.PassthroughConfig).Input)
	require.Equal(t, "10", out.(testcomponents.PassthroughExports).Output)

	in, out = getFields(t, ctrl.loader.Graph(), "testcomponents.passthrough.inc_dep_2")
	require.Equal(t, "10", in.(testcomponents.PassthroughConfig).Input)
	require.Equal(t, "10", out.(testcomponents.PassthroughExports).Output)

	in, _ = getFields(t, ctrl.loader.Graph(), "testcomponents.summation.sum")
	require.Equal(t, 10, in.(testcomponents.SummationConfig).Input)
}

func newTestController(t *testing.T) *Runtime {
	ctrl, err := newController(controllerOptions{
		Options:        testOptions(t),
		ModuleRegistry: newModuleRegistry(),
		IsModule:       false,
		// Make sure that we have consistent number of workers for tests to make them deterministic.
		WorkerPool: worker.NewFixedWorkerPool(4, 100),
	})
	require.NoError(t, err)
	return ctrl
}
