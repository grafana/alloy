package batch_test

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/agent/internal/component/otelcol/processor/batch"
	"github.com/grafana/agent/internal/flow/componenttest"
	"github.com/grafana/agent/internal/flow/logging/level"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/river"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/batchprocessor"
)

// Test performs a basic integration test which runs the
// otelcol.processor.batch component and ensures that it can accept, process, and forward data.
func Test(t *testing.T) {
	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.processor.batch")
	require.NoError(t, err)

	cfg := `
		timeout = "10ms"
		
		output {
			// no-op: will be overridden by test code.
		}
	`
	var args batch.Arguments
	require.NoError(t, river.Unmarshal([]byte(cfg), &args))

	// Override our arguments so traces get forwarded to traceCh.
	traceCh := make(chan ptrace.Traces)
	args.Output = makeTracesOutput(traceCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
	require.NoError(t, ctrl.WaitExports(time.Second), "component never exported anything")

	// Send traces in the background to our processor.
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

	// Wait for our processor to finish and forward data to traceCh.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for traces")
	case tr := <-traceCh:
		require.Equal(t, 1, tr.SpanCount())
	}
}

// makeTracesOutput returns ConsumerArguments which will forward traces to the
// provided channel.
func makeTracesOutput(ch chan ptrace.Traces) *otelcol.ConsumerArguments {
	traceConsumer := fakeconsumer.Consumer{
		ConsumeTracesFunc: func(ctx context.Context, t ptrace.Traces) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- t:
				return nil
			}
		},
	}

	return &otelcol.ConsumerArguments{
		Traces: []otelcol.Consumer{&traceConsumer},
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

func TestArguments_UnmarshalRiver(t *testing.T) {
	tests := []struct {
		cfg               string
		expectedArguments batch.Arguments
	}{
		{
			cfg: `
			output {}
			`,
			expectedArguments: batch.Arguments{
				Timeout:                  batch.DefaultArguments.Timeout,
				SendBatchSize:            batch.DefaultArguments.SendBatchSize,
				SendBatchMaxSize:         0,
				MetadataKeys:             nil,
				MetadataCardinalityLimit: batch.DefaultArguments.MetadataCardinalityLimit,
			},
		},
		{
			cfg: `
			timeout = "11s"
			send_batch_size = 8000
			send_batch_max_size = 10000
			output {}
			`,
			expectedArguments: batch.Arguments{
				Timeout:                  11 * time.Second,
				SendBatchSize:            8000,
				SendBatchMaxSize:         10000,
				MetadataKeys:             nil,
				MetadataCardinalityLimit: batch.DefaultArguments.MetadataCardinalityLimit,
			},
		},
		{
			cfg: `
			timeout = "11s"
			send_batch_size = 8000
			send_batch_max_size = 10000
			metadata_keys = ["tenant_id"]
			metadata_cardinality_limit = 123
			output {}
			`,
			expectedArguments: batch.Arguments{
				Timeout:                  11 * time.Second,
				SendBatchSize:            8000,
				SendBatchMaxSize:         10000,
				MetadataKeys:             []string{"tenant_id"},
				MetadataCardinalityLimit: 123,
			},
		},
	}

	for _, tc := range tests {
		var args batch.Arguments
		err := river.Unmarshal([]byte(tc.cfg), &args)
		require.NoError(t, err)

		ext, err := args.Convert()
		require.NoError(t, err)

		otelArgs, ok := (ext).(*batchprocessor.Config)
		require.True(t, ok)

		require.Equal(t, otelArgs.Timeout, tc.expectedArguments.Timeout)
		require.Equal(t, otelArgs.SendBatchSize, tc.expectedArguments.SendBatchSize)
		require.Equal(t, otelArgs.SendBatchMaxSize, tc.expectedArguments.SendBatchMaxSize)
		require.Equal(t, otelArgs.MetadataKeys, tc.expectedArguments.MetadataKeys)
		require.Equal(t, otelArgs.MetadataCardinalityLimit, tc.expectedArguments.MetadataCardinalityLimit)
	}
}
