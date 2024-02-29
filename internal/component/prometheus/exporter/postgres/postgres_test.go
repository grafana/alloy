package postgres

import (
	"testing"

	"github.com/grafana/agent/internal/static/integrations/postgres_exporter"
	"github.com/grafana/river"
	"github.com/grafana/river/rivertypes"
	config_util "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
)

func TestRiverConfigUnmarshal(t *testing.T) {
	var exampleRiverConfig = `
	data_source_names = ["postgresql://username:password@localhost:5432/database?sslmode=disable"]
	disable_settings_metrics = true
	disable_default_metrics = true
	custom_queries_config_path = "/tmp/queries.yaml"
	
	autodiscovery {
		enabled = false
		database_allowlist = ["include1"]
		database_denylist  = ["exclude1", "exclude2"]
	}`

	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)

	expected := Arguments{
		DataSourceNames:        []rivertypes.Secret{rivertypes.Secret("postgresql://username:password@localhost:5432/database?sslmode=disable")},
		DisableSettingsMetrics: true,
		AutoDiscovery: AutoDiscovery{
			Enabled:           false,
			DatabaseDenylist:  []string{"exclude1", "exclude2"},
			DatabaseAllowlist: []string{"include1"},
		},
		DisableDefaultMetrics:   true,
		CustomQueriesConfigPath: "/tmp/queries.yaml",
	}

	require.Equal(t, expected, args)
}

func TestRiverConfigConvert(t *testing.T) {
	var exampleRiverConfig = `
	data_source_names = ["postgresql://username:password@localhost:5432/database?sslmode=disable"]
	disable_settings_metrics = true
	disable_default_metrics = true
	custom_queries_config_path = "/tmp/queries.yaml"
	
	autodiscovery {
		enabled = false
		database_allowlist = ["include1"]
		database_denylist  = ["exclude1", "exclude2"]
	}`

	var args Arguments
	err := river.Unmarshal([]byte(exampleRiverConfig), &args)
	require.NoError(t, err)

	c := args.Convert()

	expected := postgres_exporter.Config{
		DataSourceNames:        []config_util.Secret{config_util.Secret("postgresql://username:password@localhost:5432/database?sslmode=disable")},
		DisableSettingsMetrics: true,
		AutodiscoverDatabases:  false,
		ExcludeDatabases:       []string{"exclude1", "exclude2"},
		IncludeDatabases:       []string{"include1"},
		DisableDefaultMetrics:  true,
		QueryPath:              "/tmp/queries.yaml",
	}
	require.Equal(t, expected, *c)
}

func TestParsePostgresURL(t *testing.T) {
	dsn := "postgresql://linus:42secret@localhost:5432/postgres?sslmode=disable"
	expected := map[string]string{
		"dbname":   "postgres",
		"host":     "localhost",
		"password": "42secret",
		"port":     "5432",
		"sslmode":  "disable",
		"user":     "linus",
	}

	actual, err := parsePostgresURL(dsn)
	require.NoError(t, err)
	require.Equal(t, actual, expected)
}
