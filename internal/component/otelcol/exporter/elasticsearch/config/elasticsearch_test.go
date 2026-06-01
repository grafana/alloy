package config_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/elasticsearch/config"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	"github.com/stretchr/testify/require"
)

func TestConfigConversion_Minimal(t *testing.T) {
	alloyCfg := `
		endpoints = ["http://localhost:9200"]
	`

	var args config.ElasticsearchArguments
	require.NoError(t, syntax.Unmarshal([]byte(alloyCfg), &args))

	otelCfg, err := args.Convert()
	require.NoError(t, err)

	cfg := otelCfg.(*elasticsearchexporter.Config)
	require.Equal(t, []string{"http://localhost:9200"}, cfg.Endpoints)
	require.Empty(t, cfg.CloudID)
	require.Equal(t, 90*time.Second, cfg.ClientConfig.Timeout)
	require.True(t, cfg.Retry.Enabled)
	require.Equal(t, []int{http.StatusTooManyRequests}, cfg.Retry.RetryOnStatus)
	require.Equal(t, "otel", cfg.Mapping.Mode)
	require.Equal(t, []string{"bodymap", "ecs", "none", "otel", "raw"}, cfg.Mapping.AllowedModes)
	require.Equal(t, "%Y.%m.%d", cfg.LogstashFormat.DateFormat)
}

func TestConfigConversion_Full(t *testing.T) {
	alloyCfg := `
		endpoints = ["http://es-1:9200", "http://es-2:9200"]
		num_workers = 4
		logs_index = "alloy-logs"
		metrics_index = "alloy-metrics"
		traces_index = "alloy-traces"
		pipeline = "alloy-pipeline"
		metadata_keys = ["X-Tenant", "X-Source"]

		client {
			timeout            = "30s"
			compression        = "gzip"
			max_idle_conns     = 200
			idle_conn_timeout  = "10s"
			force_attempt_http2 = true
			headers = { "X-Custom" = "value" }
		}

		authentication {
			user     = "elastic"
			password = "changeme"
		}

		discover {
			on_start = true
			interval = "1m"
		}

		retry {
			enabled          = true
			max_retries      = 5
			initial_interval = "1s"
			max_interval     = "30s"
			retry_on_status  = [429, 500, 503]
		}

		mapping {
			mode          = "otel"
			allowed_modes = ["otel", "ecs"]
		}

		logstash_format {
			enabled          = true
			prefix_separator = "_"
			date_format      = "%Y-%m-%d"
		}

		telemetry {
			log_request_body  = true
			log_response_body = false
		}

		logs_dynamic_id {
			enabled = true
		}

		logs_dynamic_pipeline {
			enabled = true
		}

		sending_queue {
			enabled       = true
			num_consumers = 5
			queue_size    = 100
			batch {
				flush_timeout = "5s"
				min_size      = 1024
				max_size      = 4096
				sizer         = "bytes"
			}
		}
	`

	var args config.ElasticsearchArguments
	require.NoError(t, syntax.Unmarshal([]byte(alloyCfg), &args))

	otelCfg, err := args.Convert()
	require.NoError(t, err)

	cfg := otelCfg.(*elasticsearchexporter.Config)
	require.Equal(t, []string{"http://es-1:9200", "http://es-2:9200"}, cfg.Endpoints)
	require.Equal(t, 4, cfg.NumWorkers)
	require.Equal(t, "alloy-logs", cfg.LogsIndex)
	require.Equal(t, "alloy-metrics", cfg.MetricsIndex)
	require.Equal(t, "alloy-traces", cfg.TracesIndex)
	require.Equal(t, "alloy-pipeline", cfg.Pipeline)
	require.Equal(t, []string{"X-Tenant", "X-Source"}, cfg.MetadataKeys)

	require.Equal(t, 30*time.Second, cfg.ClientConfig.Timeout)
	require.Equal(t, 200, cfg.ClientConfig.MaxIdleConns)

	require.Equal(t, "elastic", cfg.Authentication.User)
	require.EqualValues(t, "changeme", cfg.Authentication.Password)

	require.True(t, cfg.Discovery.OnStart)
	require.Equal(t, time.Minute, cfg.Discovery.Interval)

	require.True(t, cfg.Retry.Enabled)
	require.Equal(t, 5, cfg.Retry.MaxRetries)
	require.Equal(t, time.Second, cfg.Retry.InitialInterval)
	require.Equal(t, 30*time.Second, cfg.Retry.MaxInterval)
	require.Equal(t, []int{429, 500, 503}, cfg.Retry.RetryOnStatus)

	require.Equal(t, []string{"otel", "ecs"}, cfg.Mapping.AllowedModes)

	require.True(t, cfg.LogstashFormat.Enabled)
	require.Equal(t, "_", cfg.LogstashFormat.PrefixSeparator)
	require.Equal(t, "%Y-%m-%d", cfg.LogstashFormat.DateFormat)

	require.True(t, cfg.LogRequestBody)
	require.False(t, cfg.LogResponseBody)

	require.True(t, cfg.LogsDynamicID.Enabled)
	require.True(t, cfg.LogsDynamicPipeline.Enabled)

	require.True(t, cfg.QueueBatchConfig.HasValue())
	queue := cfg.QueueBatchConfig.Get()
	require.Equal(t, 5, queue.NumConsumers)
	require.Equal(t, int64(100), queue.QueueSize)
	require.True(t, queue.Batch.HasValue())
	batch := queue.Batch.Get()
	require.Equal(t, 5*time.Second, batch.FlushTimeout)
	require.Equal(t, int64(1024), batch.MinSize)
	require.Equal(t, int64(4096), batch.MaxSize)
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name        string
		alloyCfg    string
		expectError string
	}{
		{
			name:        "no endpoint",
			alloyCfg:    `client { timeout = "10s" }`,
			expectError: "at least one of",
		},
		{
			name: "both endpoints and cloudid",
			alloyCfg: `
				endpoints = ["http://localhost:9200"]
				cloudid = "abc:def"
			`,
			expectError: "only one of",
		},
		{
			name: "endpoint via client only",
			alloyCfg: `
				client { endpoint = "http://localhost:9200" }
			`,
			expectError: "",
		},
		{
			name: "endpoints only",
			alloyCfg: `
				endpoints = ["http://localhost:9200"]
			`,
			expectError: "",
		},
		{
			name: "cloudid only",
			alloyCfg: `
				cloudid = "cluster:dXMtZWFzdC0xLmF3cy5leGFtcGxlLmNvbSRhYmM="
			`,
			expectError: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var args config.ElasticsearchArguments
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
	var args config.ElasticsearchArguments
	require.NoError(t, syntax.Unmarshal([]byte(`endpoints = ["http://localhost:9200"]`), &args))
	dm := args.DebugMetricsConfig()
	require.True(t, dm.DisableHighCardinalityMetrics)
}
