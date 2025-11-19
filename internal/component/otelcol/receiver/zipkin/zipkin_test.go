package zipkin_test

import (
	"fmt"
	"testing"
	"time"

	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/zipkin"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/zipkinreceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
)

func TestRun(t *testing.T) {
	httpAddr := componenttest.GetFreeAddr(t)

	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.zipkin")
	require.NoError(t, err)

	cfg := fmt.Sprintf(`
		endpoint = "%s"

		output { /* no-op */ }
	`, httpAddr)

	var args zipkin.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))
}

func TestArguments_UnmarshalDefaults(t *testing.T) {
	in := "output {}"

	var args zipkin.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(in), &args))

	ext, err := args.Convert()
	require.NoError(t, err)

	otelArgs, ok := (ext).(*zipkinreceiver.Config)
	require.True(t, ok)

	expected := zipkinreceiver.Config{
		ServerConfig: confighttp.ServerConfig{
			Endpoint:              "0.0.0.0:9411",
			CompressionAlgorithms: []string{"", "gzip", "zstd", "zlib", "snappy", "deflate", "lz4"},
			KeepAlivesEnabled:     true,
		},
	}

	// Check the arguments
	require.Equal(t, &expected, otelArgs)
}

func TestArguments_UnmarshalAlloy(t *testing.T) {
	t.Run("grpc", func(t *testing.T) {
		httpAddr := componenttest.GetFreeAddr(t)
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
		require.NoError(t, syntax.Unmarshal([]byte(in), &args))
		require.Equal(t, args.DebugMetricsConfig().DisableHighCardinalityMetrics, true)
		ext, err := args.Convert()
		require.NoError(t, err)
		otelArgs, ok := (ext).(*zipkinreceiver.Config)

		require.True(t, ok)

		// Check the arguments
		require.Equal(t, otelArgs.ServerConfig.Endpoint, httpAddr)
		require.Equal(t, len(otelArgs.ServerConfig.CORS.Get().AllowedOrigins), 2)
		require.Equal(t, otelArgs.ServerConfig.CORS.Get().AllowedOrigins[0], "https://*.test.com")
		require.Equal(t, otelArgs.ServerConfig.CORS.Get().AllowedOrigins[1], "https://test.com")
		require.Equal(t, otelArgs.ParseStringTags, true)
	})
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
			output {}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
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
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
				Level:                         otelcolCfg.LevelDetailed,
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
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args zipkin.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}
