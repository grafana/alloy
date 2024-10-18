package oracledb

import (
	"errors"
	"testing"

	"github.com/grafana/alloy/internal/static/integrations/oracledb_exporter"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	config_util "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
)

func TestAlloyUnmarshal(t *testing.T) {
	alloyConfig := `
	connection_string  = "oracle://user:password@localhost:1521/orcl.localnet"
	max_idle_conns     = 0
	max_open_conns     = 10
	query_timeout      = 5
	custom_metrics     = ` + "`metrics:\n- context: \"slow_queries\"\n  metricsdesc:\n    p95_time_usecs: \"Gauge metric with percentile 95 of elapsed time.\"\n    p99_time_usecs: \"Gauge metric with percentile 99 of elapsed time.\"\n  request: \"select  percentile_disc(0.95)  within group (order by elapsed_time) as p95_time_usecs,\n    percentile_disc(0.99)  within group (order by elapsed_time) as p99_time_usecs\n    from v$sql where last_active_time >= sysdate - 5/(24*60)\"`"

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		ConnectionString: alloytypes.Secret("oracle://user:password@localhost:1521/orcl.localnet"),
		MaxIdleConns:     0,
		MaxOpenConns:     10,
		QueryTimeout:     5,
		CustomMetrics: alloytypes.OptionalSecret{
			Value: `metrics:
- context: "slow_queries"
  metricsdesc:
    p95_time_usecs: "Gauge metric with percentile 95 of elapsed time."
    p99_time_usecs: "Gauge metric with percentile 99 of elapsed time."
  request: "select  percentile_disc(0.95)  within group (order by elapsed_time) as p95_time_usecs,
    percentile_disc(0.99)  within group (order by elapsed_time) as p99_time_usecs
    from v$sql where last_active_time >= sysdate - 5/(24*60)"`,
		},
	}

	require.Equal(t, expected, args)
}

func TestArgumentsValidate(t *testing.T) {
	tests := []struct {
		name    string
		args    Arguments
		wantErr bool
		err     error
	}{
		{
			name: "no connection string",
			args: Arguments{
				ConnectionString: alloytypes.Secret(""),
			},
			wantErr: true,
			err:     errNoConnectionString,
		},
		{
			name: "unable to parse connection string",
			args: Arguments{
				ConnectionString: alloytypes.Secret("oracle://user	password@localhost:1521/orcl.localnet"),
			},
			wantErr: true,
			err:     errors.New("unable to parse connection string:"),
		},
		{
			name: "unexpected scheme",
			args: Arguments{
				ConnectionString: alloytypes.Secret("notoracle://user:password@localhost:1521/orcl.localnet"),
			},
			wantErr: true,
			err:     errors.New("unexpected scheme of type"),
		},
		{
			name: "no host name",
			args: Arguments{
				ConnectionString: alloytypes.Secret("oracle://user:password@:1521/orcl.localnet"),
			},
			wantErr: true,
			err:     errNoHostname,
		},
		{
			name: "valid OracleDB",
			args: Arguments{
				ConnectionString: alloytypes.Secret("oracle://user:password@localhost:1521/orcl.localnet"),
				MaxIdleConns:     1,
				MaxOpenConns:     1,
				QueryTimeout:     5,
				CustomMetrics: alloytypes.OptionalSecret{
					Value: `metrics:
- context: "slow_queries"
  metricsdesc:
    p95_time_usecs: "Gauge metric with percentile 95 of elapsed time."
    p99_time_usecs: "Gauge metric with percentile 99 of elapsed time."
  request: "select percentile_disc(0.95) within group (order by elapsed_time) as p95_time_usecs,
           percentile_disc(0.99) within group (order by elapsed_time) as p99_time_usecs
           from v$sql where last_active_time >= sysdate - 5/(24*60)"`,
				},
			},
			wantErr: false,
			err:     errors.New("invalid custom_metrics"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.args.Validate()
			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.err.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}
func TestConvertCustom(t *testing.T) {
	strCustomMetrics := `metrics:
- context: "slow_queries"
  metricsdesc:
    p95_time_usecs: "Gauge metric with percentile 95 of elapsed time."
    p99_time_usecs: "Gauge metric with percentile 99 of elapsed time."
  request: "select  percentile_disc(0.95)  within group (order by elapsed_time) as p95_time_usecs,
    percentile_disc(0.99)  within group (order by elapsed_time) as p99_time_usecs
    from v$sql where last_active_time >= sysdate - 5/(24*60)"`

	alloyConfig := `
	connection_string  = "oracle://user:password@localhost:1521/orcl.localnet"
	custom_metrics     = ` + "`metrics:\n- context: \"slow_queries\"\n  metricsdesc:\n    p95_time_usecs: \"Gauge metric with percentile 95 of elapsed time.\"\n    p99_time_usecs: \"Gauge metric with percentile 99 of elapsed time.\"\n  request: \"select  percentile_disc(0.95)  within group (order by elapsed_time) as p95_time_usecs,\n    percentile_disc(0.99)  within group (order by elapsed_time) as p99_time_usecs\n    from v$sql where last_active_time >= sysdate - 5/(24*60)\"`"

	var args Arguments

	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	res := args.Convert()

	expected := oracledb_exporter.Config{
		ConnectionString: config_util.Secret("oracle://user:password@localhost:1521/orcl.localnet"),
		MaxIdleConns:     DefaultArguments.MaxIdleConns,
		MaxOpenConns:     DefaultArguments.MaxOpenConns,
		QueryTimeout:     DefaultArguments.QueryTimeout,
		CustomMetrics:    strCustomMetrics,
	}
	require.Equal(t, expected, *res)
}
func TestConvertNoCustom(t *testing.T) {
	alloyConfig := `
	connection_string  = "oracle://user:password@localhost:1521/orcl.localnet"
	`
	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	res := args.Convert()

	expected := oracledb_exporter.Config{
		ConnectionString: config_util.Secret("oracle://user:password@localhost:1521/orcl.localnet"),
		MaxIdleConns:     DefaultArguments.MaxIdleConns,
		MaxOpenConns:     DefaultArguments.MaxOpenConns,
		QueryTimeout:     DefaultArguments.QueryTimeout,
		CustomMetrics:    "",
	}
	require.Equal(t, expected, *res)
}
