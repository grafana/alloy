package gcp_exporter_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/static/integrations/gcp_exporter"
)

func TestConfig_Validate(t *testing.T) {
	baseConfig := gcp_exporter.Config{
		ProjectIDs:            []string{"project1"},
		MetricPrefixes:        []string{"prefix1"},
		ExtraFilters:          nil,
		ClientTimeout:         0,
		RequestInterval:       0,
		RequestOffset:         0,
		IngestDelay:           false,
		DropDelegatedProjects: false,
	}

	t.Run("Base Config is Valid", func(t *testing.T) {
		err := baseConfig.Validate()
		require.NoError(t, err, "Base config was not valid but needs to be for these tests")
	})

	tests := []struct {
		name           string
		configModifier func(config gcp_exporter.Config) gcp_exporter.Config
		shouldError    bool
	}{
		{
			name: "nil ProjectIDs",
			configModifier: func(config gcp_exporter.Config) gcp_exporter.Config {
				config.ProjectIDs = nil
				return config
			},
			shouldError: true,
		},
		{
			name: "empty ProjectIDs",
			configModifier: func(config gcp_exporter.Config) gcp_exporter.Config {
				config.ProjectIDs = []string{}
				return config
			},
			shouldError: true,
		},
		{
			name: "nil MetricPrefixes",
			configModifier: func(config gcp_exporter.Config) gcp_exporter.Config {
				config.MetricPrefixes = nil
				return config
			},
			shouldError: true,
		},
		{
			name: "empty MetricPrefixes",
			configModifier: func(config gcp_exporter.Config) gcp_exporter.Config {
				config.MetricPrefixes = []string{}
				return config
			},
			shouldError: true,
		},
		{
			name: "extraFilter which does not match a MetricPrefix",
			configModifier: func(config gcp_exporter.Config) gcp_exporter.Config {
				config.MetricPrefixes = []string{"gcp.service/logging"}
				config.ExtraFilters = []string{`logging:resource.name==\"my_resource"`}
				return config
			},
			shouldError: true,
		},
		{
			name: "1 extraFilter which matches a MetricPrefix and 1 which does not",
			configModifier: func(config gcp_exporter.Config) gcp_exporter.Config {
				config.MetricPrefixes = []string{
					"gcp.service/logging",
					"gcp.service/compute",
				}
				config.ExtraFilters = []string{
					`gcp.service/logging:resource.name=="my_resource"`,
					`gcp.service/notcompute:compute_instance.name=="instance_a"`,
				}
				return config
			},
			shouldError: true,
		},
		{
			name: "extra filter with shorter prefix than metric prefix",
			configModifier: func(config gcp_exporter.Config) gcp_exporter.Config {
				config.MetricPrefixes = []string{
					"loadbalancing.googleapis.com/https/total_latencies",
					"loadbalancing.googleapis.com/https/request_bytes_count",
				}
				config.ExtraFilters = []string{
					`loadbalancing.googleapis.com:resource.labels.backend_target_name="something"`,
				}
				return config
			},
			shouldError: false,
		},
		{
			name: "2 extra filters which both match",
			configModifier: func(config gcp_exporter.Config) gcp_exporter.Config {
				config.MetricPrefixes = []string{
					"loadbalancing.googleapis.com/https/total_latencies",
					"loadbalancing.googleapis.com/https/request_bytes_count",
				}
				config.ExtraFilters = []string{
					`loadbalancing.googleapis.com/https/total_latencies:resource.labels.backend_target_name="something"`,
					`loadbalancing.googleapis.com/https/request_bytes_count:resource.labels.backend_target_name="something else"`,
				}
				return config
			},
			shouldError: false,
		},
		{
			name: "extra filter with colon in filter expression",
			configModifier: func(config gcp_exporter.Config) gcp_exporter.Config {
				config.MetricPrefixes = []string{
					"cloudsql.googleapis.com/database/cpu",
				}
				config.ExtraFilters = []string{
					`cloudsql.googleapis.com/database/cpu:resource.labels.database_id="project_id:database_name"`,
				}
				return config
			},
			shouldError: false,
		},
	}
	for _, tt := range tests {
		testName := tt.name
		if tt.shouldError {
			testName = testName + " should error"
		} else {
			testName = testName + " should succeed"
		}
		t.Run(testName, func(t *testing.T) {
			config := tt.configModifier(baseConfig)
			err := config.Validate()
			if tt.shouldError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
