package clickhouse_test

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/clickhouse"
	"github.com/grafana/alloy/syntax"
)

func TestUnmarshalDefaults(t *testing.T) {
	var args clickhouse.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(`endpoint = "tcp://localhost:9000"`), &args))

	require.Equal(t, "tcp://localhost:9000", args.Endpoint)
	require.Equal(t, "default", args.Database)
	require.Equal(t, "otel_logs", args.LogsTableName)
	require.Equal(t, "otel_traces", args.TracesTableName)
	require.Equal(t, "MergeTree", args.TableEngine.Name)
	require.True(t, args.CreateSchema)
	require.Equal(t, "lz4", args.Compress)
	require.True(t, args.AsyncInsert)
	require.True(t, args.Queue.Enabled)
	require.True(t, args.Retry.Enabled)
}

func TestValidate(t *testing.T) {
	var args clickhouse.Arguments
	args.SetToDefault()
	require.EqualError(t, args.Validate(), "endpoint must be specified")

	args.Endpoint = "tcp://localhost:9000"
	require.NoError(t, args.Validate())
}

// TestConvert_MetricsTableNamesDefaulted guards against a regression where
// Convert() never backfilled per-metric-type table names: without an
// explicit metrics_tables block, they used to reach the exporter as empty
// strings and fail at runtime (found via a real ClickHouse deployment,
// see bugs.md Bug 3).
func TestConvert_MetricsTableNamesDefaulted(t *testing.T) {
	var args clickhouse.Arguments
	args.SetToDefault()
	args.Endpoint = "tcp://localhost:9000"

	converted, err := args.Convert()
	require.NoError(t, err)

	cfg, ok := converted.(*clickhouseexporter.Config)
	require.True(t, ok)

	require.Equal(t, "otel_metrics_gauge", cfg.MetricsTables.Gauge.Name)
	require.Equal(t, "otel_metrics_sum", cfg.MetricsTables.Sum.Name)
	require.Equal(t, "otel_metrics_summary", cfg.MetricsTables.Summary.Name)
	require.Equal(t, "otel_metrics_histogram", cfg.MetricsTables.Histogram.Name)
	require.Equal(t, "otel_metrics_exponential_histogram", cfg.MetricsTables.ExponentialHistogram.Name)
}

func TestConvert(t *testing.T) {
	cfgStr := `
		endpoint = "tcp://localhost:9000"
		username = "default"
		password = "secret"
		database = "otel"
		cluster_name = "my_cluster"
		ttl = "48h"

		table_engine {
			name   = "ReplicatedMergeTree"
			params = "'/clickhouse/tables/{shard}/otel_logs', '{replica}'"
		}

		metrics_tables {
			gauge {
				name = "custom_gauge"
			}
		}
	`
	var args clickhouse.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfgStr), &args))

	converted, err := args.Convert()
	require.NoError(t, err)

	cfg, ok := converted.(*clickhouseexporter.Config)
	require.True(t, ok)

	require.Equal(t, "tcp://localhost:9000", cfg.Endpoint)
	require.Equal(t, "default", cfg.Username)
	require.Equal(t, "secret", string(cfg.Password))
	require.Equal(t, "otel", cfg.Database)
	require.Equal(t, "my_cluster", cfg.ClusterName)
	require.Equal(t, "ReplicatedMergeTree", cfg.TableEngine.Name)
	require.Equal(t, "custom_gauge", cfg.MetricsTables.Gauge.Name)
	require.NoError(t, cfg.Validate())
}
