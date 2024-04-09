package opencensus_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/alloy/componenttest"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/opencensus"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/opencensusreceiver"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
)

// Test ensures that otelcol.receiver.opencensus can start successfully.
func Test(t *testing.T) {
	httpAddr := getFreeAddr(t)

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.opencensus")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		endpoint = "%s"
		transport = "tcp"

		output { /* no-op */ }
	`, httpAddr)

	var args opencensus.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))
}

func TestDefaultArguments_UnmarshalAlloy(t *testing.T) {
	in := `output { /* no-op */ }`

	var args opencensus.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(in), &args))
	ext, err := args.Convert()
	require.NoError(t, err)
	otelArgs, ok := (ext).(*opencensusreceiver.Config)

	require.True(t, ok)

	var defaultArgs opencensus.Arguments
	defaultArgs.SetToDefault()
	// Check the gRPC arguments
	require.Equal(t, defaultArgs.GRPC.Endpoint, otelArgs.NetAddr.Endpoint)
	require.Equal(t, defaultArgs.GRPC.Transport, otelArgs.NetAddr.Transport)
	require.Equal(t, int(defaultArgs.GRPC.ReadBufferSize), otelArgs.ReadBufferSize)
}

func TestArguments_UnmarshalAlloy(t *testing.T) {
	httpAddr := getFreeAddr(t)
	in := fmt.Sprintf(`
		cors_allowed_origins = ["https://*.test.com", "https://test.com"]

		endpoint = "%s"
		transport = "tcp"

		output { /* no-op */ }
	`, httpAddr)

	var args opencensus.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(in), &args))
	args.Convert()
	ext, err := args.Convert()
	require.NoError(t, err)
	otelArgs, ok := (ext).(*opencensusreceiver.Config)

	require.True(t, ok)

	// Check the gRPC arguments
	require.Equal(t, otelArgs.NetAddr.Endpoint, httpAddr)
	require.Equal(t, otelArgs.NetAddr.Transport, "tcp")

	// Check the CORS arguments
	require.Equal(t, len(otelArgs.CorsOrigins), 2)
	require.Equal(t, otelArgs.CorsOrigins[0], "https://*.test.com")
	require.Equal(t, otelArgs.CorsOrigins[1], "https://test.com")
}

func getFreeAddr(t *testing.T) string {
	t.Helper()

	portNumber, err := freeport.GetFreePort()
	require.NoError(t, err)

	return fmt.Sprintf("localhost:%d", portNumber)
}

func TestDebugMetricsConfig(t *testing.T) {
	tests := []struct {
		testName string
		alloyCfg string
		expected otelcol.DebugMetricsArguments
	}{
		{
			testName: "default",
			alloyCfg: `
			output {}
			`,
			expected: otelcol.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
			},
		},
		{
			testName: "explicit_false",
			alloyCfg: `
			debug_metrics {
				disable_high_cardinality_metrics = false
			}
			output {}
			`,
			expected: otelcol.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
			},
		},
		{
			testName: "explicit_true",
			alloyCfg: `
			debug_metrics {
				disable_high_cardinality_metrics = true
			}
			output {}
			`,
			expected: otelcol.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args opencensus.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}
