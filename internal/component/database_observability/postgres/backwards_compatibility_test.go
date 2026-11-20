package postgres

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedExporterCollectorNames(t *testing.T) {
	tests := []struct {
		name string
		user *PrometheusExporterArguments
		want []string
	}{
		{name: "preserves current defaults and implicit legacy groups", want: []string{"database", "database_wraparound", "postmaster", "replication", "stat_bgwriter", "stat_checkpointer", "stat_statements", "wal", "stat_activity", "stat_archiver", "stat_replication", "replication_slots", "settings"}},
		{name: "explicit standalone list does not remove legacy groups", user: &PrometheusExporterArguments{EnabledCollectors: []string{"wal"}}, want: []string{"wal", "stat_activity", "stat_archiver", "stat_replication", "replication_slots", "settings"}},
		{name: "settings remains independent of disabled defaults", user: &PrometheusExporterArguments{DisableDefaultMetrics: true}, want: []string{"settings"}},
		{name: "both disabled", user: &PrometheusExporterArguments{DisableDefaultMetrics: true, DisableSettingsMetrics: true}, want: []string{}},
		{name: "settings toggle overrides explicit selection", user: &PrometheusExporterArguments{DisableSettingsMetrics: true, EnabledCollectors: []string{"settings", "stat_statements", "stat_activity"}}, want: []string{"stat_statements", "stat_activity", "stat_archiver", "stat_replication", "replication_slots"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := resolveExporterArgs(tt.user)
			require.Equal(t, tt.want, embeddedExporterCollectorNames(args))
		})
	}
}

func TestEmbeddedExporterCustomQueryOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "queries.yml")
	require.NoError(t, os.WriteFile(path, []byte("pg_stat_activity:\n  query: SELECT 1\n  metrics: []\n"), 0600))
	args := resolveExporterArgs(nil)
	args.CustomQueriesConfigPath = path
	require.NotContains(t, embeddedExporterCollectorNames(args), "stat_activity")
	require.Contains(t, embeddedExporterCollectorNames(args), "stat_archiver")
	args.CustomQueriesConfigPath = filepath.Join(t.TempDir(), "missing.yml")
	require.Contains(t, embeddedExporterCollectorNames(args), "stat_activity", "the exporter reports missing query files; defaults must remain enabled")
	require.NoError(t, os.WriteFile(path, []byte("pg_stat_activity:\n  query: [invalid"), 0600))
	args.CustomQueriesConfigPath = path
	require.Contains(t, embeddedExporterCollectorNames(args), "stat_activity")
}

func TestEmbeddedExporterServerLabel(t *testing.T) {
	for dsn, want := range map[string]string{
		"postgresql://user:secret@db.example:5433/app?sslmode=disable": "db.example:5433",
		"postgresql://user:secret@db.example/app":                      "db.example:5432",
		"postgresql://": "localhost:5432",
	} {
		got, err := embeddedExporterServerLabel(dsn)
		require.NoError(t, err)
		require.Equal(t, want, got)
		require.NotContains(t, got, "secret")
	}
}
