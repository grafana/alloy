package oracledb

import (
	"errors"
	"testing"

	"github.com/grafana/agent/internal/static/integrations/oracledb_exporter"
	"github.com/grafana/river"
	"github.com/grafana/river/rivertypes"
	config_util "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
)

func TestRiverUnmarshal(t *testing.T) {
	riverConfig := `
	connection_string  = "oracle://user:password@localhost:1521/orcl.localnet"
	max_idle_conns     = 0
	max_open_conns     = 10
	query_timeout      = 5
	`

	var args Arguments
	err := river.Unmarshal([]byte(riverConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		ConnectionString: rivertypes.Secret("oracle://user:password@localhost:1521/orcl.localnet"),
		MaxIdleConns:     0,
		MaxOpenConns:     10,
		QueryTimeout:     5,
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
				ConnectionString: rivertypes.Secret(""),
			},
			wantErr: true,
			err:     errNoConnectionString,
		},
		{
			name: "unable to parse connection string",
			args: Arguments{
				ConnectionString: rivertypes.Secret("oracle://user	password@localhost:1521/orcl.localnet"),
			},
			wantErr: true,
			err:     errors.New("unable to parse connection string:"),
		},
		{
			name: "unexpected scheme",
			args: Arguments{
				ConnectionString: rivertypes.Secret("notoracle://user:password@localhost:1521/orcl.localnet"),
			},
			wantErr: true,
			err:     errors.New("unexpected scheme of type"),
		},
		{
			name: "no host name",
			args: Arguments{
				ConnectionString: rivertypes.Secret("oracle://user:password@:1521/orcl.localnet"),
			},
			wantErr: true,
			err:     errNoHostname,
		},
		{
			name: "valid",
			args: Arguments{
				ConnectionString: rivertypes.Secret("oracle://user:password@localhost:1521/orcl.localnet"),
			},
			wantErr: false,
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

func TestConvert(t *testing.T) {
	riverConfig := `
	connection_string  = "oracle://user:password@localhost:1521/orcl.localnet"
	`
	var args Arguments
	err := river.Unmarshal([]byte(riverConfig), &args)
	require.NoError(t, err)

	res := args.Convert()

	expected := oracledb_exporter.Config{
		ConnectionString: config_util.Secret("oracle://user:password@localhost:1521/orcl.localnet"),
		MaxIdleConns:     DefaultArguments.MaxIdleConns,
		MaxOpenConns:     DefaultArguments.MaxOpenConns,
		QueryTimeout:     DefaultArguments.QueryTimeout,
	}
	require.Equal(t, expected, *res)
}
