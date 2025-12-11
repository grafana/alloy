package operator

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestAlloyUnmarshal(t *testing.T) {
	var exampleAlloyConfig = `
    forward_to = []
    namespaces = ["my-app"]
    selector {
        match_expression {
            key = "team"
            operator = "In"
            values = ["ops"]
        }
        match_labels = {
            team = "ops",
        }
    }
`

	var args Arguments
	err := syntax.Unmarshal([]byte(exampleAlloyConfig), &args)
	require.NoError(t, err)
}

func TestScrapeOptionsValidationScheme(t *testing.T) {
	tests := []struct {
		name           string
		args           Arguments
		expectError    bool
		expectedScheme model.ValidationScheme
	}{
		{
			name: "default legacy validation",
			args: Arguments{
				KubernetesRole: "endpoints",
				Scrape: ScrapeOptions{},
			},
			expectError:    false,
			expectedScheme: model.LegacyValidation,
		},
		{
			name: "explicit legacy validation",
			args: Arguments{
				KubernetesRole: "endpoints",
				Scrape: ScrapeOptions{
					MetricNameValidationScheme: "legacy",
				},
			},
			expectError:    false,
			expectedScheme: model.LegacyValidation,
		},
		{
			name: "utf8 validation",
			args: Arguments{
				KubernetesRole: "endpoints",
				Scrape: ScrapeOptions{
					MetricNameValidationScheme: "utf8",
				},
			},
			expectError:    false,
			expectedScheme: model.UTF8Validation,
		},
		{
			name: "invalid validation scheme",
			args: Arguments{
				KubernetesRole: "endpoints",
				Scrape: ScrapeOptions{
					MetricNameValidationScheme: "invalid",
				},
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.expectError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				cfg := tt.args.Scrape.GlobalConfig()
				require.Equal(t, tt.expectedScheme, cfg.MetricNameValidationScheme)
			}
		})
	}
}
