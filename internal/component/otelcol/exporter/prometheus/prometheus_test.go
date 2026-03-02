package prometheus_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/prometheus"
	"github.com/grafana/alloy/syntax"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected prometheus.Arguments
		errorMsg string
	}{
		{
			testName: "Defaults",
			cfg: `
					forward_to = []
				`,
			expected: prometheus.Arguments{
				IncludeTargetInfo:             true,
				IncludeScopeInfo:              false,
				IncludeScopeLabels:            true,
				GCFrequency:                   5 * time.Minute,
				AddMetricSuffixes:             true,
				ForwardTo:                     []storage.Appendable{},
				ResourceToTelemetryConversion: false,
				HonorMetadata:                 false,
			},
		},
		{
			testName: "ExplicitValues",
			cfg: `
					include_target_info = false
					include_scope_info = true
					include_scope_labels = false
					gc_frequency = "1s"
					add_metric_suffixes = false
					resource_to_telemetry_conversion = true
					honor_metadata = true
					forward_to = []
				`,
			expected: prometheus.Arguments{
				IncludeTargetInfo:             false,
				IncludeScopeInfo:              true,
				IncludeScopeLabels:            false,
				GCFrequency:                   1 * time.Second,
				AddMetricSuffixes:             false,
				ForwardTo:                     []storage.Appendable{},
				ResourceToTelemetryConversion: true,
				HonorMetadata:                 true,
			},
		},
		{
			testName: "Zero GCFrequency",
			cfg: `
					gc_frequency = "0s"
					forward_to = []
				`,
			errorMsg: "gc_frequency must be greater than 0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args prometheus.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			if tc.errorMsg != "" {
				require.EqualError(t, err, tc.errorMsg)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, args)
		})
	}
}
