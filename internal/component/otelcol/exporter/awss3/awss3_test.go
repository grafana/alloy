package awss3_test

import (
	"testing"

	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/vcenter"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestDebugMetricsConfig(t *testing.T) {
	tests := []struct {
		testName string
		alloyCfg string
		expected otelcolCfg.DebugMetricsArguments
	}{
		{
			testName: "default",
			alloyCfg: `
			endpoint = "http://localhost:1234"
			username = "user"
			password = "pass"

			output {}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
			},
		},
		{
			testName: "explicit_false",
			alloyCfg: `
			endpoint = "http://localhost:1234"
			username = "user"
			password = "pass"

			debug_metrics {
				disable_high_cardinality_metrics = false
			}

			output {}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
			},
		},
		{
			testName: "explicit_true",
			alloyCfg: `
			endpoint = "http://localhost:1234"
			username = "user"
			password = "pass"

			debug_metrics {
				disable_high_cardinality_metrics = true
			}

			output {}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args vcenter.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}
