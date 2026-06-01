package config_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/opensearch/config"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/opensearchexporter"
	"github.com/stretchr/testify/require"
)

func TestConfigConversion_Minimal(t *testing.T) {
	alloyCfg := `
		client {
			endpoint = "http://localhost:9200"
		}
	`

	var args config.OpenSearchArguments
	require.NoError(t, syntax.Unmarshal([]byte(alloyCfg), &args))

	otelCfg, err := args.Convert()
	require.NoError(t, err)

	cfg := otelCfg.(*opensearchexporter.Config)
	require.Equal(t, "http://localhost:9200", cfg.ClientConfig.Endpoint)
	require.Equal(t, "default", cfg.Dataset)
	require.Equal(t, "namespace", cfg.Namespace)
	require.Equal(t, "create", cfg.BulkAction)
	require.Equal(t, "ss4o", cfg.MappingsSettings.Mode)
}

func TestConfigConversion_Full(t *testing.T) {
	alloyCfg := `
		dataset                  = "k8s"
		namespace                = "prod"
		logs_index               = "logs-cluster"
		logs_index_fallback      = "logs-fallback"
		logs_index_time_format   = "yyyy.MM.dd"
		traces_index             = "traces-cluster"
		traces_index_fallback    = "traces-fallback"
		traces_index_time_format = "yyyy.MM.dd"
		bulk_action              = "index"
		timeout                  = "30s"

		client {
			endpoint = "https://opensearch:9200"
			tls {
				insecure_skip_verify = true
			}
		}

		mapping {
			mode            = "ecs"
			timestamp_field = "@timestamp"
			unix_timestamp  = true
			dedup           = true
			dedot           = false
			fields = {
				"k8s.namespace.name" = "kubernetes.namespace",
			}
		}

		retry_on_failure {
			enabled          = true
			initial_interval = "1s"
			max_interval     = "30s"
		}

		sending_queue {
			enabled       = true
			num_consumers = 5
			queue_size    = 200
		}
	`

	var args config.OpenSearchArguments
	require.NoError(t, syntax.Unmarshal([]byte(alloyCfg), &args))

	otelCfg, err := args.Convert()
	require.NoError(t, err)

	cfg := otelCfg.(*opensearchexporter.Config)
	require.Equal(t, "https://opensearch:9200", cfg.ClientConfig.Endpoint)
	require.True(t, cfg.ClientConfig.TLS.InsecureSkipVerify)
	require.Equal(t, "k8s", cfg.Dataset)
	require.Equal(t, "prod", cfg.Namespace)
	require.Equal(t, "logs-cluster", cfg.LogsIndex)
	require.Equal(t, "logs-fallback", cfg.LogsIndexFallback)
	require.Equal(t, "yyyy.MM.dd", cfg.LogsIndexTimeFormat)
	require.Equal(t, "traces-cluster", cfg.TracesIndex)
	require.Equal(t, "traces-fallback", cfg.TracesIndexFallback)
	require.Equal(t, "yyyy.MM.dd", cfg.TracesIndexTimeFormat)
	require.Equal(t, "index", cfg.BulkAction)
	require.Equal(t, 30*time.Second, cfg.TimeoutSettings.Timeout)
	require.Equal(t, "ecs", cfg.MappingsSettings.Mode)
	require.True(t, cfg.MappingsSettings.UnixTimestamp)
	require.True(t, cfg.MappingsSettings.Dedup)
	require.False(t, cfg.MappingsSettings.Dedot)
	require.Equal(t, "@timestamp", cfg.MappingsSettings.TimestampField)
	require.Equal(t, map[string]string{"k8s.namespace.name": "kubernetes.namespace"}, cfg.MappingsSettings.Fields)
	require.True(t, cfg.BackOffConfig.Enabled)
	require.Equal(t, time.Second, cfg.BackOffConfig.InitialInterval)
	require.True(t, cfg.QueueConfig.HasValue())
	require.Equal(t, 5, cfg.QueueConfig.Get().NumConsumers)
	require.Equal(t, int64(200), cfg.QueueConfig.Get().QueueSize)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		alloyCfg    string
		expectError string
	}{
		{
			name:     "valid minimal",
			alloyCfg: `client { endpoint = "http://localhost:9200" }`,
		},
		{
			name: "missing dataset",
			alloyCfg: `
				client { endpoint = "http://localhost:9200" }
				dataset = ""
			`,
			expectError: "dataset must be specified",
		},
		{
			name: "missing namespace",
			alloyCfg: `
				client { endpoint = "http://localhost:9200" }
				namespace = ""
			`,
			expectError: "namespace must be specified",
		},
		{
			name: "invalid bulk_action",
			alloyCfg: `
				client { endpoint = "http://localhost:9200" }
				bulk_action = "update"
			`,
			expectError: "bulk_action must be",
		},
		{
			name: "invalid mapping mode",
			alloyCfg: `
				client { endpoint = "http://localhost:9200" }
				mapping { mode = "invalid" }
			`,
			expectError: "mapping.mode must be one of",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args config.OpenSearchArguments
			err := syntax.Unmarshal([]byte(tc.alloyCfg), &args)
			if tc.expectError == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.expectError)
			}
		})
	}
}

func TestDebugMetricsConfig(t *testing.T) {
	var args config.OpenSearchArguments
	require.NoError(t, syntax.Unmarshal([]byte(`client { endpoint = "http://localhost:9200" }`), &args))
	require.True(t, args.DebugMetricsConfig().DisableHighCardinalityMetrics)
}
