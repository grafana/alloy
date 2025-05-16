package oracledb

import (
	"testing"

	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/stretchr/testify/require"
)

func TestAlloyUnmarshal(t *testing.T) {
	alloyConfig := `
	connection_string  = "oracle://user:password@localhost:1521/orcl.localnet"
	max_idle_conns     = 0
	max_open_conns     = 10
	query_timeout      = 5
	custom_metrics     = ["custom_metrics.toml"]
	default_metrics    = "default_metrics.toml"`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		ConnectionString: alloytypes.Secret("oracle://user:password@localhost:1521/orcl.localnet"),
		MaxIdleConns:     0,
		MaxOpenConns:     10,
		QueryTimeout:     5,
		CustomMetrics:    []string{"custom_metrics.toml"},
		DefaultMetrics:   "default_metrics.toml",
	}

	require.Equal(t, expected, args)
}

func TestAlloyUnmarshal2(t *testing.T) {
	alloyConfig := `
	connection_string  = "localhost:1521/orcl.localnet"
	max_idle_conns     = 0
	max_open_conns     = 10
	query_timeout      = 5
	custom_metrics     = ["custom_metrics.toml"]
	default_metrics    = "default_metrics.toml"
	username           = "user"
	password           = "password"`

	var args Arguments
	err := syntax.Unmarshal([]byte(alloyConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		ConnectionString: alloytypes.Secret("localhost:1521/orcl.localnet"),
		MaxIdleConns:     0,
		MaxOpenConns:     10,
		QueryTimeout:     5,
		CustomMetrics:    []string{"custom_metrics.toml"},
		DefaultMetrics:   "default_metrics.toml",
		Username:         "user",
		Password:         alloytypes.Secret("password"),
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
			name: "schema in connection string with username and password",
			args: Arguments{
				ConnectionString: alloytypes.Secret("oracle://user:password@localhost:1521/orcl.localnet"),
				Username:         "user",
				Password:         alloytypes.Secret("password"),
			},
			wantErr: true,
			err:     errWrongSchema,
		},
		{
			name: "no password in connection string with scheme",
			args: Arguments{
				ConnectionString: alloytypes.Secret("oracle://localhost:1521/orcl.localnet"),
			},
			wantErr: true,
			err:     errNoPassword,
		},
		{
			name: "valid OracleDB",
			args: Arguments{
				ConnectionString: alloytypes.Secret("oracle://user:password@localhost:1521/orcl.localnet"),
				MaxIdleConns:     1,
				MaxOpenConns:     1,
				QueryTimeout:     5,
				CustomMetrics:    []string{"custom_metrics.toml"},
				DefaultMetrics:   "default_metrics.toml",
			},
		},
		{
			name: "valid OracleDB2",
			args: Arguments{
				ConnectionString: alloytypes.Secret("localhost:1521/orcl.localnet"),
				MaxIdleConns:     1,
				MaxOpenConns:     1,
				QueryTimeout:     5,
				CustomMetrics:    []string{"custom_metrics.toml"},
				DefaultMetrics:   "default_metrics.toml",
				Username:         "user",
				Password:         alloytypes.Secret("password"),
			},
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
