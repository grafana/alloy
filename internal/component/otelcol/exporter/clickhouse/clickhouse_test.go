package clickhouse_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/clickhouse"
	"github.com/grafana/alloy/syntax"
	clickhouseexporter "github.com/open-telemetry/opentelemetry-collector-contrib/exporter/clickhouseexporter"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configopaque"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
	}{
		{
			testName: "Defaults",
			cfg:      `endpoint = "tcp://localhost:9000"`,
		},
		{
			testName: "FullConfig",
			cfg: `
				endpoint          = "tcp://localhost:9000"
				username          = "otel"
				password          = "secret"
				database          = "otel_db"
				logs_table_name   = "my_logs"
				traces_table_name = "my_traces"
				ttl               = "72h"
				create_schema     = false
				compress          = "zstd"
				async_insert      = false
				cluster_name      = "my_cluster"

				table_engine {
					name   = "ReplicatedMergeTree"
					params = "/clickhouse/tables/{shard}/{table}"
				}

				metrics_tables {
					gauge {
						name = "my_gauge"
					}
					sum {
						name = "my_sum"
					}
				}
			`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args clickhouse.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			err = args.Validate()
			require.NoError(t, err)

			actualConfig, err := args.Convert()
			require.NoError(t, err)

			cfg := actualConfig.(*clickhouseexporter.Config)
			require.NotEmpty(t, cfg.Endpoint)
		})
	}
}

func TestArguments_Defaults(t *testing.T) {
	var args clickhouse.Arguments
	err := syntax.Unmarshal([]byte(`endpoint = "tcp://localhost:9000"`), &args)
	require.NoError(t, err)

	require.Equal(t, "default", args.Database)
	require.Equal(t, "otel_logs", args.LogsTableName)
	require.Equal(t, "otel_traces", args.TracesTableName)
	require.Equal(t, true, args.CreateSchema)
	require.Equal(t, "lz4", args.Compress)
	require.Equal(t, true, args.AsyncInsert)
	require.Equal(t, false, args.JSON)
}

func TestArguments_ConvertJSON(t *testing.T) {
	var args clickhouse.Arguments
	err := syntax.Unmarshal([]byte(`
		endpoint = "tcp://localhost:9000"
		json     = true
	`), &args)
	require.NoError(t, err)

	cfg, err := args.Convert()
	require.NoError(t, err)

	clickCfg := cfg.(*clickhouseexporter.Config)
	require.Equal(t, true, clickCfg.JSON)
}

func TestArguments_ConvertPassword(t *testing.T) {
	var args clickhouse.Arguments
	err := syntax.Unmarshal([]byte(`
		endpoint = "tcp://localhost:9000"
		password = "mysecret"
	`), &args)
	require.NoError(t, err)

	cfg, err := args.Convert()
	require.NoError(t, err)

	clickCfg := cfg.(*clickhouseexporter.Config)
	require.Equal(t, configopaque.String("mysecret"), clickCfg.Password)
}

func TestArguments_ConvertTTL(t *testing.T) {
	var args clickhouse.Arguments
	err := syntax.Unmarshal([]byte(`
		endpoint = "tcp://localhost:9000"
		ttl       = "48h"
	`), &args)
	require.NoError(t, err)

	cfg, err := args.Convert()
	require.NoError(t, err)

	clickCfg := cfg.(*clickhouseexporter.Config)
	require.Equal(t, 48*time.Hour, clickCfg.TTL)
}

func TestArguments_ConvertMetricsTables(t *testing.T) {
	var args clickhouse.Arguments
	err := syntax.Unmarshal([]byte(`
		endpoint = "tcp://localhost:9000"

		metrics_tables {
			gauge {
				name = "custom_gauge"
			}
			histogram {
				name = "custom_histogram"
			}
		}
	`), &args)
	require.NoError(t, err)

	cfg, err := args.Convert()
	require.NoError(t, err)

	clickCfg := cfg.(*clickhouseexporter.Config)
	require.Equal(t, "custom_gauge", clickCfg.MetricsTables.Gauge.Name)
	require.Equal(t, "custom_histogram", clickCfg.MetricsTables.Histogram.Name)
	require.Equal(t, "otel_metrics_sum", clickCfg.MetricsTables.Sum.Name)
}

func TestArguments_ValidateRequiresEndpoint(t *testing.T) {
	var args clickhouse.Arguments
	args.SetToDefault()
	err := args.Validate()
	require.Error(t, err)
	require.Contains(t, err.Error(), "endpoint")
}
