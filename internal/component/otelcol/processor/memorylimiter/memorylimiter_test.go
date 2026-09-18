package memorylimiter_test

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/processor/memorylimiter"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

// Test performs a basic integration test which runs the
// otelcol.processor.memory_limiter component and ensures that it can accept,
// process, and forward data.
func Test(t *testing.T) {
	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.processor.memory_limiter")
	require.NoError(t, err)

	cfg := `
		check_interval = "10ms"
		limit          = "20MiB"
		
		output {
			// no-op: will be overridden by test code.
		}
	`
	var args memorylimiter.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

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
				l.Error("failed to send traces", "err", err)
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

func TestDefaultArguments(t *testing.T) {
	var args memorylimiter.Arguments
	args.SetToDefault()

	cfg, err := args.Convert()
	require.NoError(t, err)

	// Canary for the upstream defaults our docs promise. If this fails, a contrib
	// bump changed one: update the docs, then these values.
	require.Equal(t, &memorylimiterprocessor.Config{
		MinGCIntervalWhenSoftLimited: 10 * time.Second,
		MaxGCIntervalWhenSoftLimited: 30 * time.Second,
		MaxGCIntervalWhenHardLimited: 30 * time.Second,
	}, cfg.(*memorylimiterprocessor.Config))
}

func TestGCIntervalArguments(t *testing.T) {
	tests := []struct {
		name             string
		cfg              string
		expectedErr      string
		soft, hard       time.Duration
		maxSoft, maxHard time.Duration
	}{
		{
			name: "explicit values",
			cfg: `
				check_interval = "1s"
				limit = "100MiB"
				min_gc_interval_when_soft_limited = "30s"
				min_gc_interval_when_hard_limited = "5s"
				max_gc_interval_when_soft_limited = "2m"
				max_gc_interval_when_hard_limited = "1m"
				output {}
			`,
			soft:    30 * time.Second,
			hard:    5 * time.Second,
			maxSoft: 2 * time.Minute,
			maxHard: 1 * time.Minute,
		},
		{
			name: "max below min is rejected",
			cfg: `
				check_interval = "1s"
				limit = "100MiB"
				min_gc_interval_when_soft_limited = "30s"
				max_gc_interval_when_soft_limited = "10s"
				output {}
			`,
			expectedErr: "'max_gc_interval_when_soft_limited' must be greater than or equal to 'min_gc_interval_when_soft_limited'",
		},
		{
			name: "soft below hard is rejected",
			cfg: `
				check_interval = "1s"
				limit = "100MiB"
				min_gc_interval_when_soft_limited = "1s"
				min_gc_interval_when_hard_limited = "5s"
				output {}
			`,
			expectedErr: "'min_gc_interval_when_soft_limited' should be larger than 'min_gc_interval_when_hard_limited'",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args memorylimiter.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				return
			}
			require.NoError(t, err)

			cfg, err := args.Convert()
			require.NoError(t, err)

			otelCfg := cfg.(*memorylimiterprocessor.Config)
			require.Equal(t, tc.soft, otelCfg.MinGCIntervalWhenSoftLimited)
			require.Equal(t, tc.hard, otelCfg.MinGCIntervalWhenHardLimited)
			require.Equal(t, tc.maxSoft, otelCfg.MaxGCIntervalWhenSoftLimited)
			require.Equal(t, tc.maxHard, otelCfg.MaxGCIntervalWhenHardLimited)
		})
	}
}

// check_interval has no Alloy-side rule; upstream's Validate is what rejects it.
func TestCheckIntervalRejected(t *testing.T) {
	cfg := `
		check_interval = "0s"
		limit          = "100MiB"
		output {}
	`

	var args memorylimiter.Arguments
	require.ErrorContains(t, syntax.Unmarshal([]byte(cfg), &args),
		"'check_interval' must be greater than zero")
}

// Upstream owns these rules, but its messages name limit_mib / spike_limit_mib.
// Validate rounds down first and the errors are rewritten on the way out, so what
// a user sees names the attributes they actually wrote.
func TestMiBRounding(t *testing.T) {
	tests := []struct {
		name         string
		cfg          string
		expectedErr  string
		limit, spike uint32
	}{
		{
			name:        "a limit below 1MiB rounds to nothing",
			cfg:         `check_interval = "1s"` + "\n" + `limit = "512KiB"` + "\n" + `output {}`,
			expectedErr: "'limit' or 'limit_percentage' must be greater than zero",
		},
		{
			name:        "a limit and spike that round to the same MiB",
			cfg:         `check_interval = "1s"` + "\n" + `limit = "1900KiB"` + "\n" + `spike_limit = "1200KiB"` + "\n" + `output {}`,
			expectedErr: "'spike_limit' must be smaller than 'limit'",
		},
		{
			name:        "a spike limit equal to the limit",
			cfg:         `check_interval = "1s"` + "\n" + `limit = "10MiB"` + "\n" + `spike_limit = "10MiB"` + "\n" + `output {}`,
			expectedErr: "'spike_limit' must be smaller than 'limit'",
		},
		{
			name:  "a fractional limit rounds down",
			cfg:   `check_interval = "1s"` + "\n" + `limit = "102912KiB"` + "\n" + `output {}`,
			limit: 100,
			spike: 20,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args memorylimiter.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			if tc.expectedErr != "" {
				require.ErrorContains(t, err, tc.expectedErr)
				require.NotContains(t, err.Error(), "_mib")
				return
			}
			require.NoError(t, err)

			cfg, err := args.Convert()
			require.NoError(t, err)

			otelCfg := cfg.(*memorylimiterprocessor.Config)
			require.Equal(t, tc.limit, otelCfg.MemoryLimitMiB)
			require.Equal(t, tc.spike, otelCfg.MemorySpikeLimitMiB)
		})
	}
}
