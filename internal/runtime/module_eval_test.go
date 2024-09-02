package runtime_test

// This file contains tests which verify that the Alloy controller correctly
// evaluates and updates modules, including the module's arguments and exports.

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/internal/testcomponents"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service"
	cluster_service "github.com/grafana/alloy/internal/service/cluster"
	http_service "github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	otel_service "github.com/grafana/alloy/internal/service/otel"
	remotecfg_service "github.com/grafana/alloy/internal/service/remotecfg"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	_ "github.com/grafana/alloy/internal/runtime/internal/testcomponents/module/string"
)

func TestUpdates_EmptyModule(t *testing.T) {
	defer verifyNoGoroutineLeaks(t)

	// There's an empty module in the config below, but the pipeline we test for propagation is not affected by it.
	config := `
	module.string "test" {
		content = ""
	}

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

	ctrl := runtime.New(testOptions(t))
	f, err := runtime.ParseSource(t.Name(), []byte(config))
	require.NoError(t, err)
	require.NotNil(t, f)

	err = ctrl.LoadSource(f, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ctrl.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	require.Eventually(t, func() bool {
		export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum")
		return export.LastAdded == 10
	}, 3*time.Second, 10*time.Millisecond)
}

func TestUpdates_ThroughModule(t *testing.T) {
	// We use this module in an Alloy config below.
	module := `
	argument "input" {
		optional = false
	}

	testcomponents.passthrough "pt" {
		input = argument.input.value
		lag = "1ms"
	}

	export "output" {
		value = testcomponents.passthrough.pt.output
	}
`

	// We send the count increments via module and to the summation component and verify that the updates propagate.
	config := `
	testcomponents.count "inc" {
		frequency = "10ms"
		max = 10
	}

	module.string "test" {
		content = ` + strconv.Quote(module) + `
		arguments {
			input = testcomponents.count.inc.count
		}
	}

	testcomponents.summation "sum" {
		input = module.string.test.exports.output
	}
`

	ctrl := runtime.New(testOptions(t))
	f, err := runtime.ParseSource(t.Name(), []byte(config))
	require.NoError(t, err)
	require.NotNil(t, f)

	err = ctrl.LoadSource(f, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ctrl.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	require.Eventually(t, func() bool {
		export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum")
		return export.LastAdded == 10
	}, 3*time.Second, 10*time.Millisecond)
}

func TestUpdates_TwoModules_SameCompNames(t *testing.T) {
	// We use this module in an Alloy config below.
	module := `
	testcomponents.count "inc" {
		frequency = "1ms"
		max = 100
	}

	testcomponents.passthrough "pt" {
		input = testcomponents.count.inc.count
		lag = "1ms"
	}

	export "output" {
		value = testcomponents.passthrough.pt.output
	}
`

	// We run two modules with above body, which will have the same component names, but different module IDs.
	config := `
	module.string "test_1" {
		content = ` + strconv.Quote(module) + `
	}

	testcomponents.summation "sum_1" {
		input = module.string.test_1.exports.output
	}
	
	module.string "test_2" {
		content = ` + strconv.Quote(module) + `
	}

	testcomponents.summation "sum_2" {
		input = module.string.test_2.exports.output
	}
`

	ctrl := runtime.New(testOptions(t))
	f, err := runtime.ParseSource(t.Name(), []byte(config))
	require.NoError(t, err)
	require.NotNil(t, f)

	err = ctrl.LoadSource(f, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ctrl.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	// Verify updates propagated correctly.
	require.Eventually(t, func() bool {
		export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum_1")
		return export.LastAdded == 100
	}, 3*time.Second, 10*time.Millisecond)

	require.Eventually(t, func() bool {
		export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum_2")
		return export.LastAdded == 100
	}, 3*time.Second, 10*time.Millisecond)
}

func TestUpdates_ReloadConfig(t *testing.T) {
	// We use this module in an Alloy config below.
	module := `
	argument "input" {
		optional = false
	}

	testcomponents.passthrough "pt" {
		input = argument.input.value
		lag = "1ms"
	}

	export "output" {
		value = testcomponents.passthrough.pt.output
	}
`

	// We send the count increments via module and to the summation component and verify that the updates propagate.
	config := `
	testcomponents.count "inc" {
		frequency = "10ms"
		max = 10
	}

	module.string "test" {
		content = ` + strconv.Quote(module) + `
		arguments {
			input = testcomponents.count.inc.count
		}
	}

	testcomponents.summation "sum" {
		input = module.string.test.exports.output
	}
`

	ctrl := runtime.New(testOptions(t))
	f, err := runtime.ParseSource(t.Name(), []byte(config))
	require.NoError(t, err)
	require.NotNil(t, f)

	err = ctrl.LoadSource(f, nil)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		ctrl.Run(ctx)
		close(done)
	}()
	defer func() {
		cancel()
		<-done
	}()

	require.Eventually(t, func() bool {
		export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum")
		return export.LastAdded == 10
	}, 3*time.Second, 10*time.Millisecond)

	// Reload with a new export.
	module = `
	argument "input" {
		optional = false
	}

	testcomponents.passthrough "pt" {
		input = argument.input.value
		lag = "1ms"
	}

	export "output" {
		value = -10
	}
`
	config = `
	testcomponents.count "inc" {
		frequency = "10ms"
		max = 10
	}

	module.string "test" {
		content = ` + strconv.Quote(module) + `
		arguments {
			input = testcomponents.count.inc.count
		}
	}

	testcomponents.summation "sum" {
		input = module.string.test.exports.output
	}
`
	f, err = runtime.ParseSource(t.Name(), []byte(config))
	require.NoError(t, err)
	require.NotNil(t, f)

	err = ctrl.LoadSource(f, nil)
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		export := getExport[testcomponents.SummationExports](t, ctrl, "", "testcomponents.summation.sum")
		return export.LastAdded == -10
	}, 3*time.Second, 10*time.Millisecond)
}

func testOptions(t *testing.T) runtime.Options {
	t.Helper()
	s, err := logging.New(os.Stderr, logging.DefaultOptions)
	require.NoError(t, err)

	clusterService, err := cluster_service.New(cluster_service.Options{
		Log:              s,
		EnableClustering: false,
		NodeName:         "test-node",
		AdvertiseAddress: "127.0.0.1:80",
	})
	require.NoError(t, err)

	otelService := otel_service.New(s)
	require.NotNil(t, otelService)

	remotecfgService, err := remotecfg_service.New(remotecfg_service.Options{
		Logger:      s,
		StoragePath: t.TempDir(),
		Metrics:     prometheus.DefaultRegisterer,
	})
	require.NoError(t, err)

	return runtime.Options{
		Logger:               s,
		DataPath:             t.TempDir(),
		MinStability:         featuregate.StabilityPublicPreview,
		EnableCommunityComps: true,
		Reg:                  nil,
		Services: []service.Service{
			http_service.New(http_service.Options{}),
			clusterService,
			otelService,
			labelstore.New(nil, prometheus.DefaultRegisterer),
			remotecfgService,
		},
	}
}

func getExport[T any](t *testing.T, ctrl *runtime.Runtime, moduleId string, nodeId string) T {
	t.Helper()
	info, err := ctrl.GetComponent(component.ID{
		ModuleID: moduleId,
		LocalID:  nodeId,
	}, component.InfoOptions{
		GetHealth:    true,
		GetArguments: true,
		GetExports:   true,
		GetDebugInfo: true,
	})
	require.NoError(t, err)
	return info.Exports.(T)
}

func verifyNoGoroutineLeaks(t *testing.T) {
	t.Helper()
	goleak.VerifyNone(
		t,
		goleak.IgnoreTopFunction("go.opencensus.io/stats/view.(*worker).start"),
		goleak.IgnoreTopFunction("go.opentelemetry.io/otel/sdk/trace.(*batchSpanProcessor).processQueue"),
		goleak.IgnoreTopFunction("internal/poll.runtime_pollWait"), // related to TCP keep alive
	)
}
