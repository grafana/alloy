//go:build !freebsd && !openbsd

package datadog_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/datadog"
	datadog_config "github.com/grafana/alloy/internal/component/otelcol/exporter/datadog/config"
	datadogOtelconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configoptional"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestConfigConversion(t *testing.T) {
	var (
		defaultRetrySettings = configretry.NewDefaultBackOffConfig()
		defaultTimeout       = 15 * time.Second
		defaultQueueConfig   = configoptional.Some(exporterhelper.NewDefaultQueueConfig())

		// Until logs get added, our default config is not equal to the default factory config
		// from the official exporter; as such as need to init it all here
		defaultExporterSettings = datadogOtelconfig.MetricsExporterConfig{
			ResourceAttributesAsTags:           false,
			InstrumentationScopeMetadataAsTags: true,
		}
		defaultHistSettings = datadogOtelconfig.HistogramConfig{
			Mode:             "distributions",
			SendAggregations: false,
		}
		defaultSumSettings = datadogOtelconfig.SumConfig{
			CumulativeMonotonicMode:        datadogOtelconfig.CumulativeMonotonicSumModeToDelta,
			InitialCumulativeMonotonicMode: datadogOtelconfig.InitialValueModeAuto,
		}
		defaultSummarySettings = datadogOtelconfig.SummaryConfig{
			Mode: datadogOtelconfig.SummaryModeGauges,
		}

		defaultClient = confighttp.ClientConfig{
			Timeout:         defaultTimeout,
			MaxIdleConns:    100,
			IdleConnTimeout: 90 * time.Second,
		}
		connsPerHost = 10
	)

	tests := []struct {
		testName string
		alloyCfg string
		expected datadogOtelconfig.Config
	}{
		{
			testName: "full customise",
			alloyCfg: `
				hostname = "customhostname"
				hostname_detection_timeout = "5s"

				client {
					timeout = "10s"
					max_conns_per_host = 10
				}

				api {
					api_key = "abc"
					fail_on_invalid_key = true
				}
				traces {
					ignore_resources = ["(GET|POST) /healthcheck"]
					span_name_remappings = {
						"instrumentation:express.server" = "express",
					}
				}
				logs {
					use_compression = true
					compression_level = 9
				}
				metrics {
					delta_ttl = 1200
					exporter {
						resource_attributes_as_tags = true
						instrumentation_scope_metadata_as_tags = false
					}
					histograms {
						mode = "counters"
					}
					sums {
						initial_cumulative_monotonic_value = "keep"
					}
					summaries {
						mode = "noquantiles"
					}
				}
			`,
			expected: datadogOtelconfig.Config{
				ClientConfig:             confighttp.ClientConfig{Timeout: 10 * time.Second, Endpoint: "", MaxConnsPerHost: connsPerHost, MaxIdleConns: 100, IdleConnTimeout: 90 * time.Second},
				QueueSettings:            defaultQueueConfig,
				BackOffConfig:            defaultRetrySettings,
				TagsConfig:               datadogOtelconfig.TagsConfig{Hostname: "customhostname"},
				OnlyMetadata:             false,
				HostnameDetectionTimeout: 5 * time.Second,
				API: datadogOtelconfig.APIConfig{
					Key:              configopaque.String("abc"),
					Site:             "datadoghq.com",
					FailOnInvalidKey: true,
				},
				Logs: datadogOtelconfig.LogsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://http-intake.logs.datadoghq.com",
					},

					UseCompression:   true,
					CompressionLevel: 9,
					BatchWait:        5,
				},
				Metrics: datadogOtelconfig.MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://api.datadoghq.com",
					},
					DeltaTTL: 1200,
					ExporterConfig: datadogOtelconfig.MetricsExporterConfig{
						ResourceAttributesAsTags:           true,
						InstrumentationScopeMetadataAsTags: false,
					},
					HistConfig: datadogOtelconfig.HistogramConfig{
						SendAggregations: false,
						Mode:             datadogOtelconfig.HistogramModeCounters,
					},
					SumConfig: datadogOtelconfig.SumConfig{
						CumulativeMonotonicMode:        datadogOtelconfig.CumulativeMonotonicSumModeToDelta,
						InitialCumulativeMonotonicMode: datadogOtelconfig.InitialValueModeKeep,
					},
					SummaryConfig: datadogOtelconfig.SummaryConfig{
						Mode: datadogOtelconfig.SummaryModeNoQuantiles,
					},
				},
				Traces: datadogOtelconfig.TracesExporterConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://trace.agent.datadoghq.com",
					},
					TracesConfig: datadogOtelconfig.TracesConfig{
						SpanNameRemappings: map[string]string{
							"instrumentation:express.server": "express",
						},
						IgnoreResources: []string{"(GET|POST) /healthcheck"},
					},
				},
				HostMetadata: datadogOtelconfig.HostMetadataConfig{
					Enabled:        true,
					HostnameSource: datadogOtelconfig.HostnameSourceConfigOrSystem,
					ReporterPeriod: 30 * time.Minute,
				},
			},
		},
		{
			testName: "default",
			alloyCfg: ` 
				api {
					api_key = "abc"
				}
			`,
			expected: datadogOtelconfig.Config{
				ClientConfig:             defaultClient,
				QueueSettings:            defaultQueueConfig,
				BackOffConfig:            defaultRetrySettings,
				TagsConfig:               datadogOtelconfig.TagsConfig{},
				OnlyMetadata:             false,
				HostnameDetectionTimeout: 25 * time.Second,
				API:                      datadogOtelconfig.APIConfig{Key: configopaque.String("abc"), Site: "datadoghq.com"},
				Logs: datadogOtelconfig.LogsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://http-intake.logs.datadoghq.com",
					},
					UseCompression:   true,
					CompressionLevel: 6,
					BatchWait:        5,
				},
				Metrics: datadogOtelconfig.MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://api.datadoghq.com",
					},
					DeltaTTL:       3600,
					ExporterConfig: defaultExporterSettings,
					HistConfig:     defaultHistSettings,
					SumConfig:      defaultSumSettings,
					SummaryConfig:  defaultSummarySettings,
				},
				Traces: datadogOtelconfig.TracesExporterConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://trace.agent.datadoghq.com",
					},
					TracesConfig: datadogOtelconfig.TracesConfig{
						IgnoreResources: []string{},
					},
				},
				HostMetadata: datadogOtelconfig.HostMetadataConfig{
					Enabled:        true,
					HostnameSource: datadogOtelconfig.HostnameSourceConfigOrSystem,
					ReporterPeriod: 30 * time.Minute,
				},
			},
		},
		{
			testName: "alt datadog site",
			alloyCfg: ` 
				api {
					api_key = "abc"
					site = "ap1.datadoghq.com"
				}
				// endpoint overwritten for traces only
				traces {
        			endpoint = "https://trace.agent.datadoghq.com"
    			}
			`,
			expected: datadogOtelconfig.Config{
				ClientConfig:             defaultClient,
				QueueSettings:            defaultQueueConfig,
				BackOffConfig:            defaultRetrySettings,
				TagsConfig:               datadogOtelconfig.TagsConfig{},
				OnlyMetadata:             false,
				HostnameDetectionTimeout: 25 * time.Second,
				API:                      datadogOtelconfig.APIConfig{Key: configopaque.String("abc"), Site: "ap1.datadoghq.com"},
				Logs: datadogOtelconfig.LogsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://http-intake.logs.ap1.datadoghq.com",
					},
					UseCompression:   true,
					CompressionLevel: 6,
					BatchWait:        5,
				},
				Metrics: datadogOtelconfig.MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://api.ap1.datadoghq.com",
					},
					DeltaTTL:       3600,
					ExporterConfig: defaultExporterSettings,
					HistConfig:     defaultHistSettings,
					SumConfig:      defaultSumSettings,
					SummaryConfig:  defaultSummarySettings,
				},
				Traces: datadogOtelconfig.TracesExporterConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://trace.agent.datadoghq.com",
					},
					TracesConfig: datadogOtelconfig.TracesConfig{
						IgnoreResources: []string{},
					},
				},
				HostMetadata: datadogOtelconfig.HostMetadataConfig{
					Enabled:        true,
					HostnameSource: datadogOtelconfig.HostnameSourceConfigOrSystem,
					ReporterPeriod: 30 * time.Minute,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args datadog.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			actual, err := args.Convert()
			require.NoError(t, err)
			require.Equal(t, &tc.expected, actual.(*datadogOtelconfig.Config))
		})
	}
}

func TestValidate(t *testing.T) {
	for _, tt := range []struct {
		name      string
		cfg       *datadog.Arguments
		expectErr bool
	}{
		{
			name: "invalid config",
			cfg: &datadog.Arguments{
				APISettings: datadog_config.DatadogAPIArguments{
					Key: "abc",
				},
				OnlyMetadata: true,
				HostMetadata: datadog_config.DatadogHostMetadataArguments{
					Enabled: false,
				},
			},
			expectErr: true,
		},
		{
			name: "valid config",
			cfg: &datadog.Arguments{
				APISettings: datadog_config.DatadogAPIArguments{
					Key: "abc",
				},
			},
			expectErr: false,
		},
	} {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.cfg.Validate()
			if tt.expectErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
