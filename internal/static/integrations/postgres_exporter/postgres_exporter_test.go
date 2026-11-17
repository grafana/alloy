package postgres_exporter

import (
	"testing"

	"github.com/prometheus-community/postgres_exporter/collector"
	config_util "github.com/prometheus/common/config"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/runtime/logging"
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

func TestResolveEnabledCollectors(t *testing.T) {
	tests := []struct {
		name            string
		enabled         []string
		disableSettings bool
		want            []string
	}{
		{name: "explicit allowlist replaces defaults", enabled: []string{"wal"}, want: []string{"wal"}},
		{name: "settings removed from explicit allowlist", enabled: []string{"settings", "wal"}, disableSettings: true, want: []string{"wal"}},
		{name: "settings is the only selected collector", enabled: []string{"settings"}, disableSettings: true, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := append([]string(nil), tt.enabled...)
			got := resolveEnabledCollectors(tt.enabled, tt.disableSettings)
			require.Equal(t, tt.want, got)
			require.Equal(t, before, tt.enabled, "selection must not mutate another component's config")
		})
	}
	t.Run("defaults match upstream v0.20.1", func(t *testing.T) {
		c, err := collector.NewPostgresCollector(logging.NewSlogNop(), nil, "postgresql://localhost/app", nil)
		require.NoError(t, err)
		got := resolveEnabledCollectors(nil, false)
		require.Len(t, got, len(c.Collectors))
		for _, name := range got {
			require.Contains(t, c.Collectors, name)
		}
		withoutSettings := resolveEnabledCollectors(nil, true)
		require.Len(t, withoutSettings, len(got)-1)
		require.NotContains(t, withoutSettings, "settings")
	})
}

func TestNewWithNoSelectedCollectors(t *testing.T) {
	cfg := &Config{
		DataSourceNames:   []config_util.Secret{"postgresql://localhost/app"},
		EnabledCollectors: []string{"settings"}, DisableSettingsMetrics: true,
	}
	integration, err := New(logging.NewSlogNop(), cfg)
	require.NoError(t, err)
	// An empty resolved list must not be sent to upstream: it means "all defaults" there.
	_, err = integration.MetricsHandler()
	require.NoError(t, err)
}
