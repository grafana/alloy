package azureblob_test

import (
	"testing"
	"time"

	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/azureblob"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const validConfig = `
	url = "https://example.blob.core.windows.net"
	auth {
		connection_string = "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=dGVzdGtleQ==;EndpointSuffix=core.windows.net"
	}
`

func TestDebugMetricsConfig(t *testing.T) {
	tests := []struct {
		testName string
		agentCfg string
		expected otelcolCfg.DebugMetricsArguments
	}{
		{
			testName: "default",
			agentCfg: validConfig + `
			debug_metrics {
				disable_high_cardinality_metrics = true
			}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "no_optional_debug",
			agentCfg: validConfig,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_false",
			agentCfg: validConfig + `
			debug_metrics {
				disable_high_cardinality_metrics = false
			}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_true",
			agentCfg: validConfig + `
			debug_metrics {
				disable_high_cardinality_metrics = true
			}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_debug_level",
			agentCfg: validConfig + `
			debug_metrics {
				level = "none"
			}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelNone,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args azureblob.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.agentCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}

func TestConfig(t *testing.T) {
	tests := []struct {
		testName string
		agentCfg string
		expected azureblobexporter.Config
	}{
		{
			testName: "default",
			agentCfg: validConfig,
			expected: azureblobexporter.Config{
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 30 * time.Second,
				},
				QueueSettings: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				URL:           "https://example.blob.core.windows.net",
				Auth: azureblobexporter.Authentication{
					Type:             azureblobexporter.AuthType("connection_string"),
					ConnectionString: "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=dGVzdGtleQ==;EndpointSuffix=core.windows.net",
				},
				Container: azureblobexporter.TelemetryConfig{
					Logs:    "logs",
					Metrics: "metrics",
					Traces:  "traces",
				},
				BlobNameFormat: azureblobexporter.BlobNameFormat{
					MetricsFormat:            "2006/01/02/metrics_15_04_05.json",
					LogsFormat:               "2006/01/02/logs_15_04_05.json",
					TracesFormat:             "2006/01/02/traces_15_04_05.json",
					SerialNumEnabled:         true,
					SerialNumRange:           10000,
					SerialNumBeforeExtension: false,
					Params:                   map[string]string{},
					TemplateEnabled:          false,
					TimeParserEnabled:        true,
				},
				FormatType: "json",
				AppendBlob: azureblobexporter.AppendBlob{
					Enabled:   false,
					Separator: "\n",
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     5 * time.Second,
					RandomizationFactor: 0.5,
					Multiplier:          1.5,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      5 * time.Minute,
				},
			},
		},
		{
			testName: "explicit_values",
			agentCfg: `
			timeout = "12s"
			url = "https://explicit.blob.core.windows.net"
			format = "proto"
			auth {
				type = "service_principal"
				tenant_id = "tid"
				client_id = "cid"
				client_secret = "sec"
			}
			container {
				logs = "l"
				metrics = "m"
				traces = "t"
			}
			blob_name_format {
				metrics_format = "m.json"
				logs_format = "l.json"
				traces_format = "t.json"
				serial_num_enabled = false
				serial_num_range = 42
				serial_num_before_extension = true
				timezone = "UTC"
				template_enabled = true
				time_parser_enabled = false
				time_parser_ranges = ["start", "end"]
				params = { "env" = "prod" }
			}
			append_blob {
				enabled = true
				separator = "\r\n"
			}
			retry_on_failure {
				enabled = true
				initial_interval = "2s"
				randomization_factor = 0.1
				multiplier = 2.0
				max_interval = "10s"
				max_elapsed_time = "1m"
			}
			`,
			expected: azureblobexporter.Config{
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 12 * time.Second,
				},
				QueueSettings: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
				URL:           "https://explicit.blob.core.windows.net",
				Auth: azureblobexporter.Authentication{
					Type:         azureblobexporter.AuthType("service_principal"),
					TenantID:     "tid",
					ClientID:     "cid",
					ClientSecret: "sec",
				},
				Container: azureblobexporter.TelemetryConfig{
					Logs:    "l",
					Metrics: "m",
					Traces:  "t",
				},
				BlobNameFormat: azureblobexporter.BlobNameFormat{
					MetricsFormat:            "m.json",
					LogsFormat:               "l.json",
					TracesFormat:             "t.json",
					SerialNumEnabled:         false,
					SerialNumRange:           42,
					SerialNumBeforeExtension: true,
					Timezone:                 "UTC",
					TemplateEnabled:          true,
					TimeParserEnabled:        false,
					TimeParserRanges:         []string{"start", "end"},
					Params:                   map[string]string{"env": "prod"},
				},
				FormatType: "proto",
				AppendBlob: azureblobexporter.AppendBlob{
					Enabled:   true,
					Separator: "\r\n",
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     2 * time.Second,
					RandomizationFactor: 0.1,
					Multiplier:          2,
					MaxInterval:         10 * time.Second,
					MaxElapsedTime:      1 * time.Minute,
				},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args azureblob.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.agentCfg), &args))
			actual, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, &tc.expected, actual)
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*azureblob.Arguments)
		wantErr string
	}{
		{
			name: "serial number range is zero",
			mutate: func(args *azureblob.Arguments) {
				args.BlobNameFormat.SerialNumRange = 0
			},
			wantErr: "blob_name_format.serial_num_range must be > 0",
		},
		{
			name: "serial number range is ignored when disabled",
			mutate: func(args *azureblob.Arguments) {
				args.BlobNameFormat.SerialNumEnabled = false
				args.BlobNameFormat.SerialNumRange = 0
			},
		},
		{
			name: "unknown format",
			mutate: func(args *azureblob.Arguments) {
				args.Format = "invalid"
			},
			wantErr: "unknown format type: invalid",
		},
		{
			name: "missing connection string",
			mutate: func(args *azureblob.Arguments) {
				args.Auth.ConnectionString = ""
			},
			wantErr: "connection_string cannot be empty",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args azureblob.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(validConfig), &args))
			tc.mutate(&args)

			if tc.wantErr == "" {
				require.NoError(t, args.Validate())
				return
			}
			require.ErrorContains(t, args.Validate(), tc.wantErr)
		})
	}
}
