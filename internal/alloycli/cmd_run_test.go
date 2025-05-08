package alloycli

import (
	"testing"

	"github.com/go-kit/log"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/featuregate"
)

func TestConfigurePrometheusMetricNameValidationScheme(t *testing.T) {
	tests := []struct {
		name           string
		run            alloyRun
		expectedError  string
		expectedScheme model.ValidationScheme
	}{
		{
			name: "legacy validation scheme",
			run: alloyRun{
				prometheusMetricNameValidationScheme: prometheusLegacyMetricValidationScheme,
				minStability:                         featuregate.StabilityGenerallyAvailable,
			},
			expectedScheme: model.LegacyValidation,
		},
		{
			name: "utf8 validation scheme with experimental stability",
			run: alloyRun{
				prometheusMetricNameValidationScheme: prometheusUTF8MetricValidationScheme,
				minStability:                         featuregate.StabilityExperimental,
			},
			expectedScheme: model.UTF8Validation,
		},
		{
			name: "utf8 validation scheme with GA stability should fail",
			run: alloyRun{
				prometheusMetricNameValidationScheme: prometheusUTF8MetricValidationScheme,
				minStability:                         featuregate.StabilityGenerallyAvailable,
			},
			expectedError: `Prometheus utf-8 metric name validation scheme is at stability level "experimental", which is below the minimum allowed stability level "generally-available"`,
		},
		{
			name: "invalid validation scheme",
			run: alloyRun{
				prometheusMetricNameValidationScheme: "invalid",
				minStability:                         featuregate.StabilityGenerallyAvailable,
			},
			expectedError: `invalid prometheus metric name validation scheme: "invalid"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Reset the global validation scheme before each test
			defer func() {
				model.NameValidationScheme = model.LegacyValidation
			}()

			err := tc.run.configurePrometheusMetricNameValidationScheme(log.NewNopLogger())

			if tc.expectedError != "" {
				require.ErrorContains(t, err, tc.expectedError)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedScheme, model.NameValidationScheme)
		})
	}
}
