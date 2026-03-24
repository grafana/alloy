package faro_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/faro"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/faroexporter"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestConfigConversion(t *testing.T) {
	var (
		defaultRetrySettings = configretry.NewDefaultBackOffConfig()
		defaultTimeout       = 30 * time.Second
		defaultQueueConfig   = configoptional.Some(exporterhelper.NewDefaultQueueConfig())
	)

	tests := []struct {
		testName string
		alloyCfg string
		expected faroexporter.Config
	}{
		{
			testName: "full customise",
			alloyCfg: `
				client {
					endpoint = "https://faro.example.com/collect"
					timeout = "10s"
					compression = "none"
					write_buffer_size = "512KiB"
					headers = {
						"X-Scope-OrgID" = "123",
					}
					force_attempt_http2 = false
				}
				sending_queue {
					enabled = true
					num_consumers = 10
					queue_size = 1000
				}
				retry_on_failure {
					enabled = true
					initial_interval = "1s"
					max_interval = "30s"
					max_elapsed_time = "5m"
					randomization_factor = 0.1
					multiplier = 2
				}
			`,
			expected: faroexporter.Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint:        "https://faro.example.com/collect",
					Timeout:         10 * time.Second,
					Compression:     "none",
					WriteBufferSize: 512 * 1024,
					MaxIdleConns:    100,
					IdleConnTimeout: 90 * time.Second,
					Headers: configopaque.MapList{
						configopaque.Pair{Name: "X-Scope-OrgID", Value: "123"},
					},
					ForceAttemptHTTP2: false,
				},
				QueueConfig: defaultQueueConfig,
				RetryConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     time.Second,
					MaxInterval:         30 * time.Second,
					MaxElapsedTime:      5 * time.Minute,
					RandomizationFactor: 0.1,
					Multiplier:          2,
				},
			},
		},
		{
			testName: "default",
			alloyCfg: `
				client {
					endpoint = "https://faro.example.com/collect"
				}
			`,
			expected: faroexporter.Config{
				ClientConfig: confighttp.ClientConfig{
					Endpoint:          "https://faro.example.com/collect",
					Timeout:           defaultTimeout,
					Compression:       "gzip",
					WriteBufferSize:   512 * 1024,
					MaxIdleConns:      100,
					IdleConnTimeout:   90 * time.Second,
					Headers:           configopaque.MapList{},
					ForceAttemptHTTP2: true,
				},
				QueueConfig: defaultQueueConfig,
				RetryConfig: defaultRetrySettings,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args faro.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			actual, err := args.Convert()
			require.NoError(t, err)
			require.Equal(t, &tc.expected, actual.(*faroexporter.Config))
		})
	}
}
