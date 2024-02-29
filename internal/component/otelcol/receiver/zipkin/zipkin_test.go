package zipkin_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/receiver/zipkin"
	"github.com/grafana/agent/internal/flow/componenttest"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/river"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"
	"github.com/phayes/freeport"
	"github.com/stretchr/testify/require"
)

func TestRun(t *testing.T) {
	httpAddr := getFreeAddr(t)

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.zipkin")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		endpoint = "%s"

		output { /* no-op */ }
	`, httpAddr)

	var args zipkin.Arguments
	require.NoError(t, river.Unmarshal([]byte(cfg), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))
}

func TestArguments_UnmarshalRiver(t *testing.T) {
	t.Run("grpc", func(t *testing.T) {
		httpAddr := getFreeAddr(t)
		in := fmt.Sprintf(`
		endpoint = "%s"
		cors {
			allowed_origins = ["https://*.test.com", "https://test.com"]
		}

		parse_string_tags = true

		debug_metrics {
			disable_high_cardinality_metrics = true
		}

		output { /* no-op */ }
		`, httpAddr)

		var args zipkin.Arguments
		require.NoError(t, river.Unmarshal([]byte(in), &args))
		require.Equal(t, args.DebugMetricsConfig().DisableHighCardinalityMetrics, true)
		ext, err := args.Convert()
		require.NoError(t, err)
		otelArgs, ok := (ext).(*zipkinreceiver.Config)

		require.True(t, ok)

		// Check the arguments
		require.Equal(t, otelArgs.HTTPServerSettings.Endpoint, httpAddr)
		require.Equal(t, len(otelArgs.HTTPServerSettings.CORS.AllowedOrigins), 2)
		require.Equal(t, otelArgs.HTTPServerSettings.CORS.AllowedOrigins[0], "https://*.test.com")
		require.Equal(t, otelArgs.HTTPServerSettings.CORS.AllowedOrigins[1], "https://test.com")
		require.Equal(t, otelArgs.ParseStringTags, true)
	})
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
		agentCfg string
		expected otelcol.DebugMetricsArguments
	}{
		{
			testName: "default",
			agentCfg: `
			output {}
			`,
			expected: otelcol.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
			},
		},
		{
			testName: "explicit_false",
			agentCfg: `
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
			agentCfg: `
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
			var args zipkin.Arguments
			require.NoError(t, river.Unmarshal([]byte(tc.agentCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}
