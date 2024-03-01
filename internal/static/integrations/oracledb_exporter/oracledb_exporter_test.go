package oracledb_exporter

import (
	"errors"
	"testing"

	config_util "github.com/prometheus/common/config"
	go_ora "github.com/sijms/go-ora/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestOracleDBConfig(t *testing.T) {
	strConfig := `
enabled: true
connection_string: oracle://user:password@localhost:1521/orcl.localnet
scrape_timeout: "1m"
scrape_integration: true
max_idle_connections: 0
max_open_connections: 10
query_timeout: 5`

	var c Config
	require.NoError(t, yaml.Unmarshal([]byte(strConfig), &c))

	require.Equal(t, Config{
		ConnectionString: "oracle://user:password@localhost:1521/orcl.localnet",
		MaxIdleConns:     0,
		MaxOpenConns:     10,
		QueryTimeout:     5,
	}, c)
}

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name        string
		getConfig   func() Config
		expectedErr error
	}{
		{
			name: "valid",
			getConfig: func() Config {
				c := DefaultConfig
				c.ConnectionString = "oracle://user:password@localhost:1521/orcl.localnet"
				return c
			},
		},
		{
			name: "go_ora built connection string",
			getConfig: func() Config {
				c := DefaultConfig
				c.ConnectionString = config_util.Secret(go_ora.BuildUrl("localhost", 1521, "service", "user", "pass", nil))
				return c
			},
		},
		{
			name: "no hostname",
			getConfig: func() Config {
				c := DefaultConfig
				c.ConnectionString = config_util.Secret(go_ora.BuildUrl("", 1521, "service", "user", "pass", nil))
				return c
			},
			expectedErr: errNoHostname,
		},
		{
			name: "no connection string",
			getConfig: func() Config {
				return DefaultConfig
			},
			expectedErr: errNoConnectionString,
		},
		{
			name: "invalid scheme - cockroachdb",
			getConfig: func() Config {
				c := DefaultConfig
				c.ConnectionString = "postgres://maxroach@localhost:26257/movr?password=pwd"
				return c
			},
			expectedErr: errors.New("unexpected scheme of type 'postgres'. Was expecting 'oracle'"),
		},
		{
			name: "invalid connection string",
			getConfig: func() Config {
				c := DefaultConfig
				c.ConnectionString = "localhost:1521"
				return c
			},
			expectedErr: errors.New("unexpected scheme of type 'localhost'. Was expecting 'oracle'"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.getConfig()
			if tc.expectedErr == nil {
				require.NoError(t, validateConnString(string(cfg.ConnectionString)))
				return
			}
			require.ErrorContains(t, validateConnString(string(cfg.ConnectionString)), tc.expectedErr.Error())
		})
	}
}

func TestConfig_InstanceKey(t *testing.T) {
	c := DefaultConfig
	c.ConnectionString = "oracle://user:password@localhost:1521/orcl.localnet"

	ik := "agent-key"
	id, err := c.InstanceKey(ik)
	require.NoError(t, err)
	require.Equal(t, "localhost:1521", id)
}
