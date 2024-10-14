package splunkhec_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/splunkhec"
	splunkhec_config "github.com/grafana/alloy/internal/component/otelcol/exporter/splunkhec/config"
	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestConfigConversion(t *testing.T) {
	tests := []struct {
		testName string
		alloyCfg string
		expected splunkhec_config.SplunkHecArguments
	}{
		{
			testName: "full customise",
			alloyCfg: `
				splunk {
				   token = "token"
				   source = "source"
				   sourcetype = "sourcetype"
				   index = "index"
				}
				client {
				   endpoint = "http://localhost:8088"
				   timeout = "10s"
				   insecure_skip_verify = true
		        }
			`,
			expected: splunkhec_config.SplunkHecArguments{
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),
				RetrySettings: configretry.NewDefaultBackOffConfig(),
				Splunk: splunkhec_config.SplunkConf{
					Token:                   "token",
					Source:                  "source",
					SourceType:              "sourcetype",
					Index:                   "index",
					LogDataEnabled:          true,
					ProfilingDataEnabled:    true,
					DisableCompression:      false,
					MaxContentLengthLogs:    2097152,
					MaxContentLengthMetrics: 2097152,
					MaxContentLengthTraces:  2097152,
					MaxEventSize:            5242880,
					SplunkAppName:           "Alloy",
					SplunkAppVersion:        "",
					HealthPath:              "/services/collector/health",
					HecHealthCheckEnabled:   false,
					ExportRaw:               false,
					UseMultiMetricFormat:    false,
					Heartbeat:               splunkhec_config.SplunkHecHeartbeat{},
					Telemetry:               splunkhec_config.SplunkHecTelemetry{},
				},
				SplunkHecClientArguments: splunkhec_config.SplunkHecClientArguments{
					Endpoint:           "http://localhost:8088",
					Timeout:            10 * time.Second,
					InsecureSkipVerify: true,
				},
			},
		},
		{
			testName: "minimal customise",
			alloyCfg: `
				splunk {
				   token = "token"
		         }
				client {
				  endpoint = "http://localhost:8088"
				}
				`,
			expected: splunkhec_config.SplunkHecArguments{
				QueueSettings: exporterhelper.NewDefaultQueueSettings(),
				RetrySettings: configretry.NewDefaultBackOffConfig(),
				Splunk: splunkhec_config.SplunkConf{
					Token:                   "token",
					Source:                  "",
					SourceType:              "",
					Index:                   "",
					LogDataEnabled:          true,
					ProfilingDataEnabled:    true,
					DisableCompression:      false,
					MaxContentLengthLogs:    2097152,
					MaxContentLengthMetrics: 2097152,
					MaxContentLengthTraces:  2097152,
					MaxEventSize:            5242880,
					SplunkAppName:           "Alloy",
					SplunkAppVersion:        "",
					HealthPath:              "/services/collector/health",
					HecHealthCheckEnabled:   false,
					ExportRaw:               false,
					UseMultiMetricFormat:    false,
					Heartbeat:               splunkhec_config.SplunkHecHeartbeat{},
					Telemetry:               splunkhec_config.SplunkHecTelemetry{},
				},
				SplunkHecClientArguments: splunkhec_config.SplunkHecClientArguments{
					Endpoint: "http://localhost:8088",
					Timeout:  15 * time.Second,
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.testName, func(t *testing.T) {
			t.Parallel()
			var args splunkhec.Arguments
			err := syntax.Unmarshal([]byte(tt.alloyCfg), &args)
			if err != nil {
				t.Fatal(err)
			}

			cfg, err := args.Convert()
			if err != nil {
				t.Fatal(err)
			}

			require.Equal(t, tt.expected, cfg)
		})
	}
}
