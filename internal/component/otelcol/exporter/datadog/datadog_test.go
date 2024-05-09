package datadog_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/datadog"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	"go.opentelemetry.io/collector/config/confighttp"

	"github.com/grafana/alloy/syntax"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestConfigConversion(t *testing.T) {
	var (
		defaultRetrySettings   = configretry.NewDefaultBackOffConfig()
		defaultTimeoutSettings = exporterhelper.NewDefaultTimeoutSettings()

		defaultQueueSettings = exporterhelper.QueueSettings{
			Enabled:      true,
			NumConsumers: 10,
			QueueSize:    1000,
		}

		defaultClient = confighttp.ClientConfig{
			Endpoint:        "",
			Compression:     "gzip",
			WriteBufferSize: 512 * 1024,
			Headers:         map[string]configopaque.String{},
			Timeout:         defaultTimeoutSettings.Timeout,
		}
	)

	tests := []struct {
		testName string
		alloyCfg string
		expected datadogexporter.Config
	}{
		{
			testName: "default",
			alloyCfg: `
				hostname = "customhostname" 
				only_metadata = false 
				api {
					api_key = "abc"
				}
			`,
			expected: datadogexporter.Config{
				ClientConfig:  defaultClient,
				QueueSettings: defaultQueueSettings,
				BackOffConfig: defaultRetrySettings,
				TagsConfig:    datadogexporter.TagsConfig{Hostname: "customhostname"},
				OnlyMetadata:  false,
				API:           datadogexporter.APIConfig{Key: configopaque.String("abc")},
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
