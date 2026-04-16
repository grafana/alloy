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
	c := DefaultConfig
	c.ConnectionString = config_util.Secret("localhost:1521/orcl.localnet")

	id, err := c.InstanceKey("agent-key")
	require.NoError(t, err)
	require.Equal(t, "localhost:1521", id)
}

func TestConfig_InstanceKey_multipleUsesDefaultKey(t *testing.T) {
	c := Config{
		Databases: []DatabaseInstance{
			{Name: "a", ConnectionString: "localhost:1521/a"},
			{Name: "b", ConnectionString: "localhost:1521/b"},
		},
	}
	id, err := c.InstanceKey("component-id")
	require.NoError(t, err)
	require.Equal(t, "component-id", id)
}

func TestNormalizeConnectionString_oracleURL(t *testing.T) {
	u, user, pass := NormalizeConnectionString("oracle://u:p@host:1521/svc", "", "")
	require.Equal(t, "host:1521/svc", u)
	require.Equal(t, "u", user)
	require.Equal(t, "p", pass)
}
