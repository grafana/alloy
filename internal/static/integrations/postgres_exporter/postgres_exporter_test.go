package postgres_exporter

import (
	"testing"

	pgconfig "github.com/prometheus-community/postgres_exporter/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func Test_ParsePostgresURL(t *testing.T) {
	t.Run("with valid url", func(t *testing.T) {
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
	})

	t.Run("with minimal url", func(t *testing.T) {
		minimal := "postgres://"
		expected := map[string]string{}
		actual, err := parsePostgresURL(minimal)
		require.NoError(t, err)
		require.Equal(t, actual, expected)
	})
}

func TestUpstreamConfig(t *testing.T) {
	t.Run("defaults keep upstream's default collectors and pass validation", func(t *testing.T) {
		cfg := (&Config{}).UpstreamConfig([]string{"postgres://localhost:5432/db"})
		require.Equal(t, pgconfig.DefaultCollectorConfig(), cfg.Collectors)
		require.Equal(t, uint(100), cfg.PGStatStatements.Limit)
		require.Equal(t, uint(120), cfg.PGStatStatements.QueryLength)
		_, err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("enabled_collectors is an allowlist plus the legacy default queries", func(t *testing.T) {
		cfg := (&Config{EnabledCollectors: []string{"stat_statements"}}).UpstreamConfig([]string{"postgres://localhost:5432/db"})
		var enabled []string
		for name, on := range cfg.Collectors {
			if on {
				enabled = append(enabled, name)
			}
		}
		require.ElementsMatch(t, []string{"stat_statements", "settings", "stat_activity", "stat_archiver", "stat_replication"}, enabled)
	})

	t.Run("disable_default_metrics turns every collector off", func(t *testing.T) {
		cfg := (&Config{DisableDefaultMetrics: true, QueryPath: "q.yaml"}).UpstreamConfig([]string{"postgres://localhost:5432/db"})
		for name, on := range cfg.Collectors {
			require.False(t, on, name)
		}
		require.True(t, cfg.DisableDefaultMetrics)
		require.Equal(t, "q.yaml", cfg.UserQueriesPath)
	})

	t.Run("disable_settings_metrics turns off settings only", func(t *testing.T) {
		cfg := (&Config{DisableSettingsMetrics: true}).UpstreamConfig([]string{"postgres://localhost:5432/db"})
		require.False(t, cfg.Collectors["settings"])
		require.True(t, cfg.Collectors["stat_activity"])
	})

	t.Run("zero stat_statements limit and query_length get defaults", func(t *testing.T) {
		cfg := (&Config{StatStatementFlags: &StatStatementFlags{IncludeQuery: true, ExcludeUsers: []string{"u"}}}).UpstreamConfig([]string{"postgres://localhost:5432/db"})
		require.Equal(t, pgconfig.PGStatStatementsConfig{IncludeQuery: true, QueryLength: 120, Limit: 100, ExcludeUsers: []string{"u"}}, cfg.PGStatStatements)
		_, err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("explicit stat_statements limit and query_length are kept", func(t *testing.T) {
		cfg := (&Config{StatStatementFlags: &StatStatementFlags{Limit: 7, QueryLength: 9}}).UpstreamConfig([]string{"postgres://localhost:5432/db"})
		require.Equal(t, uint(7), cfg.PGStatStatements.Limit)
		require.Equal(t, uint(9), cfg.PGStatStatements.QueryLength)
	})

	t.Run("unknown collector fails validation", func(t *testing.T) {
		cfg := (&Config{EnabledCollectors: []string{"nope"}}).UpstreamConfig([]string{"postgres://localhost:5432/db"})
		_, err := cfg.Validate()
		require.Error(t, err)
	})
}

func Test_getDataSourceNames(t *testing.T) {
	tt := []struct {
		name   string
		config string
		env    string
		expect []string
	}{
		{
			name:   "env",
			config: "{}",
			env:    "foo",
			expect: []string{"foo"},
		},
		{
			name:   "multi-env",
			config: "{}",
			env:    "foo,bar",
			expect: []string{"foo", "bar"},
		},
		{
			name: "config",
			config: `{
        "data_source_names": [
          "foo"
        ]
      }`,
			env:    "",
			expect: []string{"foo"},
		},
		{
			name: "config and env",
			config: `{
        "data_source_names": [
          "foo"
        ]
      }`,
			env:    "bar",
			expect: []string{"foo"},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("POSTGRES_EXPORTER_DATA_SOURCE_NAME", tc.env)

			var cfg Config
			err := yaml.Unmarshal([]byte(tc.config), &cfg)
			require.NoError(t, err)

			res, err := cfg.getDataSourceNames()
			require.NoError(t, err)
			require.Equal(t, tc.expect, res)
		})
	}
}
