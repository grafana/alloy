package cardinalityguardian_test

import (
	"testing"

	"github.com/go-viper/mapstructure/v2"
	"github.com/grafana/alloy/internal/component/otelcol/processor/cardinalityguardian"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected map[string]any
	}{
		{
			testName: "Defaults",
			cfg: `
				output {}
			`,
			expected: map[string]any{
				"max_cardinality_delta_per_epoch": 100,
				"epoch_duration_seconds":          300,
				"never_drop_labels":               []string{"http.status_code", "region"},
				"enforcement_mode":                "tag_only",
				"estimated_cost_per_metric_month": 0.05,
				"top_offenders_count":             10,
				"max_tracker_count":               0,
				"drop_log_max_per_epoch":          10,
			},
		},
		{
			testName: "Explicit Values",
			cfg: `
				max_cardinality_delta_per_epoch = 500
				epoch_duration_seconds = 600
				never_drop_labels = ["region"]
				enforcement_mode = "overflow_attribute"
				estimated_cost_per_metric_month = 0.10
				top_offenders_count = 20
				max_tracker_count = 100000
				metric_overrides = {
					"http.server.request.duration" = 5000,
				}
				drop_log_max_per_epoch = 5
				output {}
			`,
			expected: map[string]any{
				"max_cardinality_delta_per_epoch": 500,
				"epoch_duration_seconds":          600,
				"never_drop_labels":               []string{"region"},
				"enforcement_mode":                "overflow_attribute",
				"estimated_cost_per_metric_month": 0.10,
				"top_offenders_count":             20,
				"max_tracker_count":               100000,
				"metric_overrides": map[string]any{
					"http.server.request.duration": 5000,
				},
				"drop_log_max_per_epoch": 5,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args cardinalityguardian.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*cardinalityguardianprocessor.Config)

			var expected cardinalityguardianprocessor.Config
			err = mapstructure.Decode(tc.expected, &expected)
			require.NoError(t, err)

			require.Equal(t, expected, *actual)
		})
	}
}

func TestArguments_Validate(t *testing.T) {
	tests := []struct {
		testName      string
		cfg           string
		expectedError string
	}{
		{
			testName: "Invalid Max Cardinality Delta",
			cfg: `
				max_cardinality_delta_per_epoch = 0
				output {}
			`,
			expectedError: "max_cardinality_delta_per_epoch must be greater than 0",
		},
		{
			testName: "Epoch Duration Too Short",
			cfg: `
				epoch_duration_seconds = 5
				output {}
			`,
			expectedError: "epoch_duration_seconds must be at least 10",
		},
		{
			testName: "Invalid Enforcement Mode",
			cfg: `
				enforcement_mode = "drop_everything"
				output {}
			`,
			expectedError: `enforcement_mode must be one of: tag_only, overflow_attribute, strip_and_reaggregate; got "drop_everything"`,
		},
		{
			testName: "Invalid Top Offenders Count",
			cfg: `
				top_offenders_count = 501
				output {}
			`,
			expectedError: "top_offenders_count must be between 0 and 500",
		},
		{
			testName: "Invalid Metric Override",
			cfg: `
				metric_overrides = {
					"http.server.request.duration" = 0,
				}
				output {}
			`,
			expectedError: `metric_overrides["http.server.request.duration"] must be greater than 0`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args cardinalityguardian.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.ErrorContains(t, err, tc.expectedError)
		})
	}
}
