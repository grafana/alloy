package otlphttp_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/otlphttp"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Test performs a basic integration test which runs the
// otelcol.exporter.otlphttp component and ensures that it can pass data to an
// OTLP HTTP server.
func Test(t *testing.T) {
	ch := make(chan ptrace.Traces)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		decoder := &ptrace.ProtoUnmarshaler{}
		trace, _ := decoder.UnmarshalTraces(b)
		require.Equal(t, 1, trace.SpanCount())
		name := trace.ResourceSpans().At(0).ScopeSpans().At(0).Spans().At(0).Name()
		require.Equal(t, "TestSpan", name)
		ch <- trace
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.exporter.otlphttp")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		client {
			endpoint = "%s"

			compression = "none"

			tls {
				insecure             = true
				insecure_skip_verify = true
			}
		}
	`, srv.URL)
	var args otlphttp.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")

	// Send traces in the background to our exporter.
	go func() {
		exports := ctrl.Exports().(otelcol.ConsumerExports)

		bo := backoff.New(ctx, backoff.Config{
			MinBackoff: 10 * time.Millisecond,
			MaxBackoff: 100 * time.Millisecond,
		})
		for bo.Ongoing() {
			err := exports.Input.ConsumeTraces(ctx, createTestTraces())
			if err != nil {
				level.Error(l).Log("msg", "failed to send traces", "err", err)
				bo.Wait()
				continue
			}

			return
		}
	}()

	// Wait for our exporter to finish and pass data to our HTTP server.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for traces")
	case tr := <-ch:
		require.Equal(t, 1, tr.SpanCount())
	}
}

func createTestTraces() ptrace.Traces {
	// Matches format from the protobuf definition:
	// https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto
	var bb = `{
		"resource_spans": [{
			"scope_spans": [{
				"spans": [{
					"name": "TestSpan"
				}]
			}]
		}]
	}`

	decoder := &ptrace.JSONUnmarshaler{}
	data, err := decoder.UnmarshalTraces([]byte(bb))
	if err != nil {
		panic(err)
	}
	return data
}

func TestQueueBatchConfig(t *testing.T) {
	tests := []struct {
		testName string
		alloyCfg string
		expected otelcol.QueueArguments
	}{
		{
			testName: "default",
			alloyCfg: `
			client {
				endpoint = "http://tempo:4317"
			}
			sending_queue {
				batch {}
			}
			`,
			expected: otelcol.QueueArguments{
				Enabled:      true,
				NumConsumers: 10,
				QueueSize:    1000,
				Sizer:        "requests",
				Batch: &otelcol.BatchConfig{
					FlushTimeout: 200 * time.Millisecond,
					MinSize:      2000,
					MaxSize:      3000,
					Sizer:        "items",
				},
			},
		},
		{
			testName: "explicit_batch",
			alloyCfg: `
			client {
				endpoint = "http://tempo:4317"
			}
			sending_queue {
				batch {
					flush_timeout = "100ms"
					min_size      = 4096
					max_size      = 16384
					sizer         = "bytes"
				}
			}
			`,
			expected: otelcol.QueueArguments{
				Enabled:      true,
				NumConsumers: 10,
				QueueSize:    1000,
				Sizer:        "requests",
				Batch: &otelcol.BatchConfig{
					FlushTimeout: 100 * time.Millisecond,
					MinSize:      4096,
					MaxSize:      16384,
					Sizer:        "bytes",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args otlphttp.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected.Enabled, args.Queue.Enabled)
			require.Equal(t, tc.expected.NumConsumers, args.Queue.NumConsumers)
			require.Equal(t, tc.expected.QueueSize, args.Queue.QueueSize)
			require.Equal(t, tc.expected.Sizer, args.Queue.Sizer)
			require.Equal(t, tc.expected.Batch, args.Queue.Batch)
		})
	}
}

func TestDebugMetricsConfig(t *testing.T) {
	tests := []struct {
		testName string
		alloyCfg string
		expected otelcolCfg.DebugMetricsArguments
	}{
		{
			testName: "default",
			alloyCfg: `
			client {
				endpoint = "http://tempo:4317"
			}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_false",
			alloyCfg: `
			client {
				endpoint = "http://tempo:4317"
			}
			debug_metrics {
				disable_high_cardinality_metrics = false
			}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_true",
			alloyCfg: `
			client {
				endpoint = "http://tempo:4317"
			}
			debug_metrics {
				disable_high_cardinality_metrics = true
			}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args otlphttp.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}
