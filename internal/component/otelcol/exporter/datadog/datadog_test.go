//go:build !freebsd

package datadog_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/datadog"
	datadog_config "github.com/grafana/alloy/internal/component/otelcol/exporter/datadog/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"

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
		defaultQueueConfig   = exporterhelper.NewDefaultQueueConfig()

		// Until logs get added, our default config is not equal to the default factory config
		// from the official exporter; as such as need to init it all here
		defaultExporterSettings = datadogexporter.MetricsExporterConfig{
			ResourceAttributesAsTags:           false,
			InstrumentationScopeMetadataAsTags: false,
		}
		defaultHistSettings = datadogexporter.HistogramConfig{
			Mode:             "distributions",
			SendAggregations: false,
		}
		defaultSumSettings = datadogexporter.SumConfig{
			CumulativeMonotonicMode:        datadogexporter.CumulativeMonotonicSumModeToDelta,
			InitialCumulativeMonotonicMode: datadogexporter.InitialValueModeAuto,
		}
		defaultSummarySettings = datadogexporter.SummaryConfig{
			Mode: datadogexporter.SummaryModeGauges,
		}

		defaultClient = confighttp.ClientConfig{
			Timeout: defaultTimeout,
		}
		connsPerHost    = 10
		connsPerHostPtr = &connsPerHost
	)

	tests := []struct {
		testName string
		alloyCfg string
		expected datadogexporter.Config
	}{
		{
			testName: "full customise",
			alloyCfg: `
				hostname = "customhostname" 

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
				metrics {
					delta_ttl = 1200
					exporter {
						resource_attributes_as_tags = true
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
			expected: datadogexporter.Config{
				ClientConfig:  confighttp.ClientConfig{Timeout: 10 * time.Second, Endpoint: "", MaxConnsPerHost: connsPerHostPtr},
				QueueSettings: defaultQueueConfig,
				BackOffConfig: defaultRetrySettings,
				TagsConfig:    datadogexporter.TagsConfig{Hostname: "customhostname"},
				OnlyMetadata:  false,
				API: datadogexporter.APIConfig{
					Key:              configopaque.String("abc"),
					Site:             "datadoghq.com",
					FailOnInvalidKey: true,
				},
				Metrics: datadogexporter.MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://api.datadoghq.com",
					},
					DeltaTTL: 1200,
					ExporterConfig: datadogexporter.MetricsExporterConfig{
						ResourceAttributesAsTags:           true,
						InstrumentationScopeMetadataAsTags: false,
					},
					HistConfig: datadogexporter.HistogramConfig{
						SendAggregations: false,
						Mode:             datadogexporter.HistogramModeCounters,
					},
					SumConfig: datadogexporter.SumConfig{
						CumulativeMonotonicMode:        datadogexporter.CumulativeMonotonicSumModeToDelta,
						InitialCumulativeMonotonicMode: datadogexporter.InitialValueModeKeep,
					},
					SummaryConfig: datadogexporter.SummaryConfig{
						Mode: datadogexporter.SummaryModeNoQuantiles,
					},
				},
				Traces: datadogexporter.TracesConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://trace.agent.datadoghq.com",
					},
					SpanNameRemappings: map[string]string{
						"instrumentation:express.server": "express",
					},
					IgnoreResources: []string{"(GET|POST) /healthcheck"},
				},
				HostMetadata: datadogexporter.HostMetadataConfig{
					Enabled:        true,
					HostnameSource: datadogexporter.HostnameSourceConfigOrSystem,
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
			expected: datadogexporter.Config{
				ClientConfig:  defaultClient,
				QueueSettings: defaultQueueConfig,
				BackOffConfig: defaultRetrySettings,
				TagsConfig:    datadogexporter.TagsConfig{},
				OnlyMetadata:  false,
				API:           datadogexporter.APIConfig{Key: configopaque.String("abc"), Site: "datadoghq.com"},
				Metrics: datadogexporter.MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://api.datadoghq.com",
					},
					DeltaTTL:       3600,
					ExporterConfig: defaultExporterSettings,
					HistConfig:     defaultHistSettings,
					SumConfig:      defaultSumSettings,
					SummaryConfig:  defaultSummarySettings,
				},
				Traces: datadogexporter.TracesConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://trace.agent.datadoghq.com",
					},
					IgnoreResources: []string{},
				},
				HostMetadata: datadogexporter.HostMetadataConfig{
					Enabled:        true,
					HostnameSource: datadogexporter.HostnameSourceConfigOrSystem,
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
			expected: datadogexporter.Config{
				ClientConfig:  defaultClient,
				QueueSettings: defaultQueueConfig,
				BackOffConfig: defaultRetrySettings,
				TagsConfig:    datadogexporter.TagsConfig{},
				OnlyMetadata:  false,
				API:           datadogexporter.APIConfig{Key: configopaque.String("abc"), Site: "ap1.datadoghq.com"},
				Metrics: datadogexporter.MetricsConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://api.ap1.datadoghq.com",
					},
					DeltaTTL:       3600,
					ExporterConfig: defaultExporterSettings,
					HistConfig:     defaultHistSettings,
					SumConfig:      defaultSumSettings,
					SummaryConfig:  defaultSummarySettings,
				},
				Traces: datadogexporter.TracesConfig{
					TCPAddrConfig: confignet.TCPAddrConfig{
						Endpoint: "https://trace.agent.datadoghq.com",
					},
					IgnoreResources: []string{},
				},
				HostMetadata: datadogexporter.HostMetadataConfig{
					Enabled:        true,
					HostnameSource: datadogexporter.HostnameSourceConfigOrSystem,
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
			require.Equal(t, &tc.expected, actual.(*datadogexporter.Config))
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
