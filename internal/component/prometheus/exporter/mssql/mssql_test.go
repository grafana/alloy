package mssql

import (
	"testing"
	"time"

	"github.com/burningalchemist/sql_exporter/config"
	"github.com/grafana/alloy/internal/static/integrations/mssql"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestAlloyUnmarshal(t *testing.T) {
	alloyConfig := `
	connection_string = "sqlserver://user:pass@localhost:1433"
	max_idle_connections = 3
	max_open_connections = 3
	timeout = "10s"`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		ConnectionString:   alloytypes.Secret("sqlserver://user:pass@localhost:1433"),
		MaxIdleConnections: 3,
		MaxOpenConnections: 3,
		Timeout:            10 * time.Second,
		ConnectionName:     "",
	}

	require.Equal(t, expected, args)
}

func TestAlloyUnmarshalWithName(t *testing.T) {
	alloyConfig := `
	connection_string = "sqlserver://user:pass@localhost:1433"
	connection_name = "localhost"
	max_idle_connections = 3
	max_open_connections = 3
	timeout = "10s"
	`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)
	var collectorConfig config.CollectorConfig
	err = yaml.UnmarshalStrict([]byte(args.QueryConfig.Value), &collectorConfig)
	require.NoError(t, err)

	require.Equal(t, alloytypes.Secret("sqlserver://user:pass@localhost:1433"), args.ConnectionString)
	require.Equal(t, "localhost", args.ConnectionName)
	require.Equal(t, 3, args.MaxIdleConnections)
	require.Equal(t, 3, args.MaxOpenConnections)
	require.Equal(t, 10*time.Second, args.Timeout)
}

func TestAlloyUnmarshalWithInlineQueryConfig(t *testing.T) {
	alloyConfig := `
	connection_string = "sqlserver://user:pass@localhost:1433"
	max_idle_connections = 3
	max_open_connections = 3
	timeout = "10s"
	query_config = "{ collector_name: mssql_standard, metrics: [ { metric_name: mssql_local_time_seconds, type: gauge, help: 'Local time in seconds since epoch (Unix time).', values: [ unix_time ], query: \"SELECT DATEDIFF(second, '19700101', GETUTCDATE()) AS unix_time\" } ] }"`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)
	var collectorConfig config.CollectorConfig
	err = yaml.UnmarshalStrict([]byte(args.QueryConfig.Value), &collectorConfig)
	require.NoError(t, err)

	require.Equal(t, alloytypes.Secret("sqlserver://user:pass@localhost:1433"), args.ConnectionString)
	require.Equal(t, 3, args.MaxIdleConnections)
	require.Equal(t, 3, args.MaxOpenConnections)
	require.Equal(t, 10*time.Second, args.Timeout)
	require.Equal(t, "mssql_standard", collectorConfig.Name)
	require.Equal(t, 1, len(collectorConfig.Metrics))
	require.Equal(t, "mssql_local_time_seconds", collectorConfig.Metrics[0].Name)
	require.Equal(t, "gauge", collectorConfig.Metrics[0].TypeString)
	require.Equal(t, "Local time in seconds since epoch (Unix time).", collectorConfig.Metrics[0].Help)
	require.Equal(t, 1, len(collectorConfig.Metrics[0].Values))
	require.Contains(t, collectorConfig.Metrics[0].Values, "unix_time")
	require.Equal(t, "SELECT DATEDIFF(second, '19700101', GETUTCDATE()) AS unix_time", collectorConfig.Metrics[0].QueryLiteral)
}

func TestAlloyUnmarshalWithInlineQueryConfigYaml(t *testing.T) {
	alloyConfig := `
	connection_string = "sqlserver://user:pass@localhost:1433"
	max_idle_connections = 3
	max_open_connections = 3
	timeout = "10s"
	query_config = "collector_name: mssql_standard\nmetrics:\n- metric_name: mssql_local_time_seconds\n  type: gauge\n  help: 'Local time in seconds since epoch (Unix time).'\n  values: [unix_time]\n  query: \"SELECT DATEDIFF(second, '19700101', GETUTCDATE()) AS unix_time\""`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)
	var collectorConfig config.CollectorConfig
	err = yaml.UnmarshalStrict([]byte(args.QueryConfig.Value), &collectorConfig)
	require.NoError(t, err)

	require.Equal(t, alloytypes.Secret("sqlserver://user:pass@localhost:1433"), args.ConnectionString)
	require.Equal(t, 3, args.MaxIdleConnections)
	require.Equal(t, 3, args.MaxOpenConnections)
	require.Equal(t, 10*time.Second, args.Timeout)
	require.Equal(t, "mssql_standard", collectorConfig.Name)
	require.Equal(t, 1, len(collectorConfig.Metrics))
	require.Equal(t, "mssql_local_time_seconds", collectorConfig.Metrics[0].Name)
	require.Equal(t, "gauge", collectorConfig.Metrics[0].TypeString)
	require.Equal(t, "Local time in seconds since epoch (Unix time).", collectorConfig.Metrics[0].Help)
	require.Equal(t, 1, len(collectorConfig.Metrics[0].Values))
	require.Contains(t, collectorConfig.Metrics[0].Values, "unix_time")
	require.Equal(t, "SELECT DATEDIFF(second, '19700101', GETUTCDATE()) AS unix_time", collectorConfig.Metrics[0].QueryLiteral)
}

func TestUnmarshalInvalid(t *testing.T) {
	invalidAlloyConfig := `
	connection_string = "sqlserver://user:pass@localhost:1433"
	max_idle_connections = 1
	max_open_connections = 1
	timeout = "-1s"
	`

	var invalidArgs Arguments
	err := syntax.Unmarshal([]byte(invalidAlloyConfig), &invalidArgs)
	require.Error(t, err)
	require.EqualError(t, err, "timeout must be positive")
}

func TestUnmarshalInvalidQueryConfigYaml(t *testing.T) {
	invalidAlloyConfig := `
	connection_string = "sqlserver://user:pass@localhost:1433"
	max_idle_connections = 1
	max_open_connections = 1
	timeout = "1s"
	query_config = "{ collector_name: mssql_standard, metrics: [ { metric_name: mssql_local_time_seconds, type: gauge, help: 'Local time in seconds since epoch (Unix time).', values: [ unix_time ], query: \"SELECT DATEDIFF(second, '19700101', GETUTCDATE()) AS unix_time\" }"
	`

	var invalidArgs Arguments
	err := syntax.Unmarshal([]byte(invalidAlloyConfig), &invalidArgs)
	require.Error(t, err)
	require.EqualError(t, err, "invalid query_config: yaml: line 1: did not find expected ',' or ']'")
}

func TestUnmarshalInvalidProperty(t *testing.T) {
	invalidAlloyConfig := `
	connection_string = "sqlserver://user:pass@localhost:1433"
	max_idle_connections = 1
	max_open_connections = 1
	timeout = "1s"
	query_config = "collector_name: mssql_standard\nbad_param: true\nmetrics:\n- metric_name: mssql_local_time_seconds\n  type: gauge\n  help: 'Local time in seconds since epoch (Unix time).'\n  values: [unix_time]\n  query: \"SELECT DATEDIFF(second, '19700101', GETUTCDATE()) AS unix_time\""
	`

	var invalidArgs Arguments
	err := syntax.Unmarshal([]byte(invalidAlloyConfig), &invalidArgs)
	require.Error(t, err)
	require.EqualError(t, err, "invalid query_config: unknown fields in collector: bad_param")
}

func TestArgumentsValidate(t *testing.T) {
	tests := []struct {
		name    string
		args    Arguments
		wantErr bool
	}{
		{
			name: "invalid max open connections",
			args: Arguments{
				ConnectionString:   alloytypes.Secret("test"),
				MaxIdleConnections: 1,
				MaxOpenConnections: 0,
				Timeout:            10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid max idle connections",
			args: Arguments{
				ConnectionString:   alloytypes.Secret("test"),
				MaxIdleConnections: 0,
				MaxOpenConnections: 1,
				Timeout:            10 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "invalid timeout",
			args: Arguments{
				ConnectionString:   alloytypes.Secret("test"),
				MaxIdleConnections: 1,
				MaxOpenConnections: 1,
				Timeout:            0,
			},
			wantErr: true,
		},
		{
			name: "valid",
			args: Arguments{
				ConnectionString:   alloytypes.Secret("test"),
				MaxIdleConnections: 1,
				MaxOpenConnections: 1,
				Timeout:            10 * time.Second,
				QueryConfig: alloytypes.OptionalSecret{
					Value: `{ collector_name: mssql_standard, metrics: [ { metric_name: mssql_local_time_seconds, type: gauge, help: 'Local time in seconds since epoch (Unix time).', values: [ unix_time ], query: "SELECT DATEDIFF(second, '19700101', GETUTCDATE()) AS unix_time" } ] }`,
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestConvert(t *testing.T) {
	strQueryConfig := `collector_name: mssql_standard
metrics:
- metric_name: mssql_local_time_seconds
  type: gauge
  help: 'Local time in seconds since epoch (Unix time).'
  values: [unix_time]
  query: "SELECT DATEDIFF(second, '19700101', GETUTCDATE()) AS unix_time"`

	args := Arguments{
		ConnectionString:   alloytypes.Secret("sqlserver://user:pass@localhost:1433"),
		MaxIdleConnections: 1,
		MaxOpenConnections: 1,
		Timeout:            10 * time.Second,
		QueryConfig: alloytypes.OptionalSecret{
			Value: strQueryConfig,
		},
	}
	res := args.Convert()

	expected := mssql.Config{
		ConnectionString:   config_util.Secret("sqlserver://user:pass@localhost:1433"),
		MaxIdleConnections: 1,
		MaxOpenConnections: 1,
		Timeout:            10 * time.Second,
		QueryConfig:        []byte(strQueryConfig),
	}
	require.Equal(t, expected, *res)
}
