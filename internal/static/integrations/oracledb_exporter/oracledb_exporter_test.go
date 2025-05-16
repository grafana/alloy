package oracledb_exporter

import (
	"testing"

	config_util "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestOracleDBConfig(t *testing.T) {
	strConfig := `
enabled: true
connection_string: oracle://user:password@localhost:1521/orcl.localnet
scrape_timeout: "1m"
scrape_integration: true
max_idle_conns: 0
max_open_conns: 10
query_timeout: 5
default_metrics: "default.toml"
custom_metrics: ["custom.toml"]`

	var c Config
	require.NoError(t, yaml.Unmarshal([]byte(strConfig), &c))

	require.Equal(t, Config{
		ConnectionString: "oracle://user:password@localhost:1521/orcl.localnet",
		MaxIdleConns:     0,
		MaxOpenConns:     10,
		QueryTimeout:     5,
		CustomMetrics:    []string{"custom.toml"},
		DefaultMetrics:   "default.toml",
	}, c)
}

func TestConfig_InstanceKey(t *testing.T) {
	testCases := []struct {
		name             string
		connectionString string
		expectedID       string
	}{
		{
			name:             "connection string with credentials",
			connectionString: "oracle://user:password@localhost:1521/orcl.localnet",
			expectedID:       "localhost:1521",
		},
		{
			name:             "connection string without scheme",
			connectionString: "localhost:1521/orcl.localnet",
			expectedID:       "localhost:1521",
		},
		{
			name:             "connection string without credentials",
			connectionString: "oracle://localhost:1521/orcl.localnet",
			expectedID:       "localhost:1521",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := DefaultConfig
			c.ConnectionString = config_util.Secret(tc.connectionString)

			id, err := c.InstanceKey("agent-key")
			require.NoError(t, err)
			require.Equal(t, tc.expectedID, id)
		})
	}
}

func TestParseConnectionString(t *testing.T) {
	testCases := []struct {
		name          string
		config        Config
		expectedConn  string
		expectedUser  string
		expectedPass  string
		expectedError bool
	}{
		{
			name: "valid connection string with credentials",
			config: Config{
				ConnectionString: "oracle://user:password@localhost:1521/orcl.localnet",
			},
			expectedConn:  "localhost:1521/orcl.localnet",
			expectedUser:  "user",
			expectedPass:  "password",
			expectedError: false,
		},
		{
			name: "valid connection string with username and password",
			config: Config{
				ConnectionString: "localhost:1521/orcl.localnet",
				Username:         "user",
				Password:         "pass",
			},
			expectedConn:  "localhost:1521/orcl.localnet",
			expectedUser:  "user",
			expectedPass:  "pass",
			expectedError: false,
		},
		{
			name: "connection string with credentials and separate username/password",
			config: Config{
				ConnectionString: "oracle://user:password@localhost:1521/orcl.localnet",
				Username:         "user2",
				Password:         "pass2",
			},
			expectedConn:  "",
			expectedUser:  "",
			expectedPass:  "",
			expectedError: true,
		},
		{
			name: "connection string with timezone parameter",
			config: Config{
				ConnectionString: "oracle://user:password@localhost:1521/orcl.localnet?timezone=UTC",
			},
			expectedConn:  "localhost:1521/orcl.localnet?timezone=UTC",
			expectedUser:  "user",
			expectedPass:  "password",
			expectedError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			conn, user, pass, err := parseConnectionString(&tc.config)

			if tc.expectedError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.expectedConn, conn)
			require.Equal(t, tc.expectedUser, user)
			require.Equal(t, tc.expectedPass, pass)
		})
	}
}
