package otlp_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/otlp"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
)

// Test performs a basic integration test which runs the otelcol.exporter.otlp
// component and ensures that it can pass data to an OTLP gRPC server.
func Test(t *testing.T) {
	traceCh := make(chan ptrace.Traces)
	tracesServer := makeTracesServer(t, traceCh)

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.exporter.otlp")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		timeout = "250ms"

		client {
			endpoint = "%s"

			compression = "none"

			tls {
				insecure             = true
				insecure_skip_verify = true
			}
		}

		debug_metrics {
			disable_high_cardinality_metrics = true
		}
	`, tracesServer)
	var args otlp.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
	require.Equal(t, args.DebugMetricsConfig().DisableHighCardinalityMetrics, true)

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
	case tr := <-traceCh:
		require.Equal(t, 1, tr.SpanCount())
	}
}

// makeTracesServer returns a host:port which will accept traces over insecure
// gRPC.
func makeTracesServer(t *testing.T, ch chan ptrace.Traces) string {
	t.Helper()

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	srv := grpc.NewServer()
	ptraceotlp.RegisterGRPCServer(srv, &mockTracesReceiver{ch: ch})

	go func() {
		err := srv.Serve(lis)
		require.NoError(t, err)
	}()
	t.Cleanup(srv.Stop)

	return lis.Addr().String()
}

type mockTracesReceiver struct {
	ptraceotlp.UnimplementedGRPCServer
	ch chan ptrace.Traces
}

var _ ptraceotlp.GRPCServer = (*mockTracesReceiver)(nil)

func (ms *mockTracesReceiver) Export(_ context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	ms.ch <- req.Traces()
	return ptraceotlp.NewExportResponse(), nil
}

func createTestTraces() ptrace.Traces {
	// Matches format from the protobuf definition:
	// https://github.com/open-telemetry/opentelemetry-proto/blob/main/opentelemetry/proto/trace/v1/trace.proto
	bb := `{
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
				endpoint = "tempo:4317"
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
				endpoint = "tempo:4317"
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
			var args otlp.Arguments
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
				endpoint = "tempo-xxx.grafana.net/tempo:443"
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
				endpoint = "tempo-xxx.grafana.net/tempo:443"
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
				endpoint = "tempo-xxx.grafana.net/tempo:443"
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
			var args otlp.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}
