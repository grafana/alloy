//go:build !freebsd && !openbsd

package datadog_config_test

import (
	"testing"

	datadog_config "github.com/grafana/alloy/internal/component/otelcol/exporter/datadog/config"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
)

func TestUnmarshalDatadogHostMetadataArguments(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       string
		expectErr bool
	}{
		{
			name: "valid host metadata arguments config",
			cfg: `
				enabled = true
				hostname_source = "first_resource"
				tags = ["abc", "def", "ghi"]
			`,
		},
		{
			name: "invalid api config",
			cfg: `
				tags = "abc"
				`,
			expectErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sl datadog_config.DatadogHostMetadataArguments
			err := syntax.Unmarshal([]byte(tt.cfg), &sl)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnmarshalDatadogAPIConfig(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       string
		expectErr bool
	}{
		{
			name: "valid api config",
			cfg: `
				api_key = "abc"
				fail_on_invalid_key = true
				site = "https://example.com"
			`,
		},
		{
			name: "invalid api config",
			cfg: `
				fail_on_invalid_key = true
				site = "https://example.com"
				`,
			expectErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sl datadog_config.DatadogAPIArguments
			err := syntax.Unmarshal([]byte(tt.cfg), &sl)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
func TestUnmarshalDatadogTraceConfig(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       string
		expectErr bool
	}{
		{
			name: "valid trace config",
			cfg: `
				endpoint = "abc.xyz"
				ignore_resources = ["(GET|POST) /healthcheck"]
				span_name_remappings = {
					"io.opentelemetry.javaagent.spring.client" = "spring.client",
					"instrumentation:express.server" = "express",
					"go.opentelemetry.io_contrib_instrumentation_net_http_otelhttp.client" = "http.client",
				}
				span_name_as_resource_name = true
				peer_tags = ["tag"]
			`,
		},
		{
			name: "invalid trace config",
			cfg: `
				endpoint = "abc.xyz"
				ignore_resources = "(GET|POST) /healthcheck"
				`,
			expectErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sl datadog_config.DatadogTracesArguments
			err := syntax.Unmarshal([]byte(tt.cfg), &sl)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnmarshalDatadogMetricConfig(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       string
		expectErr bool
	}{
		{
			name: "valid metrics config",
			cfg: `
				delta_ttl = 3600
				endpoint = "https://api.datadoghq.com"

				exporter {
					resource_attributes_as_tags = false
					instrumentation_scope_metadata_as_tags = false
				}

				histograms {
					mode = "distributions"
					send_aggregation_metrics = false
				}

				sums {
					cumulative_monotonic_mode = "to_delta"
					initial_cumulative_monotonic_value = "auto"      
				}

				summaries {
					mode = "gauges"
				}
				`,
		},
		{
			name: "invalid deprecated field use",
			cfg: `
				histograms {
					mode = distributions
					send_count_sum_metrics = false
					send_aggregation_metrics = false
				}
				`,
			expectErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sl datadog_config.DatadogMetricsArguments
			err := syntax.Unmarshal([]byte(tt.cfg), &sl)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestUnmarshalDatadogLogsConfig(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       string
		expectErr bool
	}{
		{
			name: "valid logs arguments config",
			cfg: `
				compression_level = 3
				use_compression = true
				batch_wait = 2
				endpoint = "test"
			`,
		},
		{
			name: "invalid logs config",
			cfg: `
				compression_level = "9"
				use_compression = "true"
				`,
			expectErr: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var sl datadog_config.DatadogLogsArguments
			err := syntax.Unmarshal([]byte(tt.cfg), &sl)
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
