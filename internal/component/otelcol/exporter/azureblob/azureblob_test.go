package azureblob_test

import (
	"testing"
	"time"

	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/azureblob"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"
	"github.com/stretchr/testify/require"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
)

func TestDebugMetricsConfig(t *testing.T) {
	tests := []struct {
		testName string
		agentCfg string
		expected otelcolCfg.DebugMetricsArguments
	}{
		{
			testName: "default",
			agentCfg: `
			blob_uploader {
				url = "https://example.blob.core.windows.net"
				auth {
					connection_string = "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=dGVzdGtleQ==;EndpointSuffix=core.windows.net"
				}
			}
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
			agentCfg: `
			blob_uploader {
				url = "https://example.blob.core.windows.net"
				auth {
					connection_string = "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=dGVzdGtleQ==;EndpointSuffix=core.windows.net"
				}
			}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_false",
			agentCfg: `
			blob_uploader {
				url = "https://example.blob.core.windows.net"
				auth {
					connection_string = "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=dGVzdGtleQ==;EndpointSuffix=core.windows.net"
				}
			}
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
			agentCfg: `
			blob_uploader {
				url = "https://example.blob.core.windows.net"
				auth {
					connection_string = "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=dGVzdGtleQ==;EndpointSuffix=core.windows.net"
				}
			}
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
			agentCfg: `
			blob_uploader {
				url = "https://example.blob.core.windows.net"
				auth {
					connection_string = "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=dGVzdGtleQ==;EndpointSuffix=core.windows.net"
				}
			}
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

// Checks that the component can start with the sumo_ic marshaler.
func TestSumoICMarshaler(t *testing.T) {
	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.exporter.azureblob")
	require.NoError(t, err)

	cfg := `
		blob_uploader {
			url = "https://example.blob.core.windows.net"
			auth {
				connection_string = "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=dGVzdGtleQ==;EndpointSuffix=core.windows.net"
			}
		}

		marshaler {
			type = "sumo_ic"
		}
	`
	var args azureblob.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")
}

// Checks that the component can be updated with the sumo_ic marshaler.
func TestSumoICMarshalerUpdate(t *testing.T) {
	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.exporter.azureblob")
	require.NoError(t, err)

	cfg := `
		blob_uploader {
			url = "https://example.blob.core.windows.net"
			auth {
				connection_string = "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=dGVzdGtleQ==;EndpointSuffix=core.windows.net"
			}
		}

		marshaler {
			type = "otlp_json"
		}
	`
	var args azureblob.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second), "component never started")

	cfg2 := `
		blob_uploader {
			url = "https://example.blob.core.windows.net"
			auth {
				connection_string = "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=dGVzdGtleQ==;EndpointSuffix=core.windows.net"
			}
		}

		marshaler {
			type = "sumo_ic"
		}
	`

	var args2 azureblob.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg2), &args2))
	require.NoError(t, ctrl.Update(args2))
}

func TestConfig(t *testing.T) {
	tests := []struct {
		testName string
		agentCfg string
		expected azureblobexporter.Config
	}{
		{
			testName: "default",
			agentCfg: `
			blob_uploader {
				url = "https://example.blob.core.windows.net"
				auth {
					type = "connection_string"
					connection_string = "DefaultEndpointsProtocol=https;AccountName=devstoreaccount1;AccountKey=dGVzdGtleQ==;EndpointSuffix=core.windows.net"
				}
			}
			`,
			expected: azureblobexporter.Config{
				URL: "https://example.blob.core.windows.net",
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
					SerialNumRange:           10000,
					SerialNumBeforeExtension: false,
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
			blob_uploader {
				url = "https://explicit.blob.core.windows.net"
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
					serial_num_range = 42
					serial_num_before_extension = true
					params = { "env" = "prod" }
				}
			}

			marshaler { type = "otlp_proto" }
			append_blob {
				enabled = true
				separator = "\r\n"
			}
			encodings {
				logs = "text_encoding"
				metrics = "text_encoding/custom"
				traces = "custom/name"
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
				URL: "https://explicit.blob.core.windows.net",
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
					SerialNumRange:           42,
					SerialNumBeforeExtension: true,
					Params:                   map[string]string{"env": "prod"},
				},
				FormatType: "proto",
				AppendBlob: azureblobexporter.AppendBlob{
					Enabled:   true,
					Separator: "\r\n",
				},
				Encodings: azureblobexporter.Encodings{
					Logs:    ptr(otelcomponent.NewID(otelcomponent.MustNewType("text_encoding"))),
					Metrics: ptr(otelcomponent.NewIDWithName(otelcomponent.MustNewType("text_encoding"), "custom")),
					Traces:  ptr(otelcomponent.NewIDWithName(otelcomponent.MustNewType("custom"), "name")),
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

// ptr returns a pointer to the provided value.
func ptr[T any](v T) *T { return &v }
