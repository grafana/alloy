package postgres

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	exporter_postgres "github.com/grafana/alloy/internal/component/prometheus/exporter/postgres"
	"github.com/grafana/alloy/syntax"
)

const testExporterDSN = "postgres://user@127.0.0.1:5432/postgres?sslmode=disable"

func TestExporterConfigMatchesLegacyOptions(t *testing.T) {
	excludeDatabases := []string{"azure_maintenance"}

	tt := []struct {
		name             string
		block            *PrometheusExporterArguments
		disableDefault   bool
		userQueriesPath  string
		enabled          []string
		disabled         []string
		limit            uint
		queryLength      uint
		statementExclude []string
	}{
		{
			name:        "defaults",
			enabled:     []string{"stat_statements", "settings", "stat_activity", "stat_archiver", "stat_replication"},
			disabled:    []string{"database", "locks", "wal", "stat_user_tables"},
			limit:       100,
			queryLength: 120,
		},
		{
			name: "with enabled_collectors",
			block: &PrometheusExporterArguments{
				EnabledCollectors: []string{"database", "long_running_transactions"},
			},
			enabled:     []string{"database", "long_running_transactions", "settings", "stat_activity"},
			disabled:    []string{"stat_statements", "locks", "wal"},
			limit:       100,
			queryLength: 120,
		},
		{
			name: "with stat_statements flags and zero limit",
			block: &PrometheusExporterArguments{
				StatStatementFlags: &exporter_postgres.StatStatementFlags{
					IncludeQuery:     true,
					ExcludeDatabases: []string{"db1"},
				},
			},
			enabled:          []string{"stat_statements"},
			limit:            100,
			queryLength:      120,
			statementExclude: []string{"db1"},
		},
		{
			name: "with stat_statements flags and explicit limit",
			block: &PrometheusExporterArguments{
				StatStatementFlags: &exporter_postgres.StatStatementFlags{
					Limit:       25,
					QueryLength: 40,
				},
			},
			enabled:     []string{"stat_statements"},
			limit:       25,
			queryLength: 40,
		},
		{
			name: "with disable_default_metrics",
			block: &PrometheusExporterArguments{
				DisableDefaultMetrics:   true,
				CustomQueriesConfigPath: "/tmp/queries.yaml",
			},
			disableDefault:  true,
			userQueriesPath: "/tmp/queries.yaml",
			disabled:        []string{"stat_statements", "settings", "stat_activity", "database"},
			limit:           100,
			queryLength:     120,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := exporterConfig(resolveExporterArgs(tc.block), testExporterDSN, excludeDatabases)

			assert.Equal(t, []string{testExporterDSN}, cfg.DataSourceNames)
			assert.True(t, cfg.AutoDiscoverDatabases)
			assert.Equal(t, "pg", cfg.MetricPrefix)
			assert.Equal(t, 10*time.Second, cfg.CollectionTimeout)
			assert.False(t, cfg.WrapLargeCounters)
			assert.Equal(t, excludeDatabases, cfg.ExcludeDatabases)
			assert.Equal(t, tc.disableDefault, cfg.DisableDefaultMetrics)
			assert.Equal(t, tc.userQueriesPath, cfg.UserQueriesPath)
			for _, name := range tc.enabled {
				assert.True(t, cfg.Collectors[name], "collector %q should be enabled", name)
			}
			for _, name := range tc.disabled {
				assert.False(t, cfg.Collectors[name], "collector %q should be disabled", name)
			}
			assert.Equal(t, tc.limit, cfg.PGStatStatements.Limit)
			assert.Equal(t, tc.queryLength, cfg.PGStatStatements.QueryLength)
			assert.Equal(t, tc.statementExclude, cfg.PGStatStatements.ExcludeDatabases)

			_, err := cfg.Validate()
			require.NoError(t, err)
		})
	}

	t.Run("disable_settings_metrics turns off the settings collector", func(t *testing.T) {
		cfg := exporterConfig(resolveExporterArgs(&PrometheusExporterArguments{DisableSettingsMetrics: true}), testExporterDSN, nil)
		assert.False(t, cfg.Collectors["settings"])
		assert.True(t, cfg.Collectors["stat_activity"])
	})

	t.Run("unknown enabled_collectors fail validation", func(t *testing.T) {
		cfg := exporterConfig(resolveExporterArgs(&PrometheusExporterArguments{EnabledCollectors: []string{"nope"}}), testExporterDSN, nil)
		_, err := cfg.Validate()
		require.Error(t, err)
	})
}

func TestExporterConfigFromAlloySyntax(t *testing.T) {
	var args Arguments
	cfg := `
		data_source_name = "postgres://user@127.0.0.1:5432/postgres?sslmode=disable"
		forward_to = []
		prometheus_exporter {
			enabled_collectors = ["stat_statements", "database"]
		}
	`
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	pgCfg := exporterConfig(resolveExporterArgs(args.PrometheusExporter), string(args.DataSourceName), args.ExcludeDatabases)
	assert.True(t, pgCfg.Collectors["stat_statements"])
	assert.True(t, pgCfg.Collectors["database"])
	assert.False(t, pgCfg.Collectors["locks"])
	_, err := pgCfg.Validate()
	require.NoError(t, err)
}
