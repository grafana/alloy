// Compatibility with the v0.19.1-based postgres_exporter fork used before
// upgrading to v0.20.1. Upstream moved legacy metric groups into standalone
// collectors in these PRs:
//   - settings (also removed disable-settings-metrics): https://github.com/prometheus-community/postgres_exporter/pull/1303
//   - stat_archiver: https://github.com/prometheus-community/postgres_exporter/pull/1304
//   - stat_activity: https://github.com/prometheus-community/postgres_exporter/pull/1305
//   - stat_replication: https://github.com/prometheus-community/postgres_exporter/pull/1306
//   - replication_slots (also renamed the collector and slot metrics): https://github.com/prometheus-community/postgres_exporter/pull/1307
//
// These changes affect collector selection, settings flags, custom queries,
// and server labels. The adapters below preserve those behaviors for
// database_observability.postgres independently of the Prometheus
// exporter component. They retain upstream metric names and the combined
// replication_slots collector; they do not reproduce the old output exactly.
// Removing an adapter requires an explicit migration decision for the behavior
// documented below, rather than just another dependency bump.

package postgres

import (
	"fmt"
	"log/slog"
	"os"
	"slices"

	pg_collector "github.com/prometheus-community/postgres_exporter/collector"
	pg_exporter "github.com/prometheus-community/postgres_exporter/exporter"
	"github.com/prometheus/client_golang/prometheus"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/component/database_observability/postgres/collector"
	exporter_postgres "github.com/grafana/alloy/internal/component/prometheus/exporter/postgres"
)

// newEmbeddedExporterCollector configures the standalone collectors needed to
// retain the legacy exporter's implicit metric groups and settings flag behavior.
func newEmbeddedExporterCollector(logger *slog.Logger, args exporter_postgres.Arguments, dsn string) (prometheus.Collector, error) {
	names := embeddedExporterCollectorNames(args)
	if len(names) == 0 {
		// An empty list means "enable defaults" to NewPostgresCollector. Skip
		// construction so disabling all groups does not re-enable them.
		return nil, nil
	}
	opts := []pg_collector.Option{pg_collector.WithCollectionTimeout("10s")}
	if flags := args.StatStatementFlags; flags != nil {
		opts = append(opts, pg_collector.WithStatStatementsConfig(pg_collector.StatStatementsConfig{
			IncludeQuery: flags.IncludeQuery, QueryLength: flags.QueryLength, Limit: flags.Limit,
			ExcludeDatabases: flags.ExcludeDatabases, ExcludeUsers: flags.ExcludeUsers,
		}))
	}
	col, err := pg_collector.NewPostgresCollector(logger, args.AutoDiscovery.DatabaseDenylist, dsn, names, opts...)
	if err != nil {
		return nil, err
	}
	server, err := embeddedExporterServerLabel(dsn)
	if err != nil {
		return nil, err
	}
	// The legacy groups included server="host:port"; their standalone
	// replacements omit it. Keep that identity for existing metric consumers.
	// This currently also adds server to previously standalone metrics, such as
	// stat_statements and scrape diagnostics; that side effect remains to resolve.
	return prometheus.WrapCollectorWith(prometheus.Labels{"server": server}, col), nil
}

// embeddedExporterCollectorNames preserves the distinction between the old
// standalone allowlist and the legacy groups that ran independently of it.
func embeddedExporterCollectorNames(args exporter_postgres.Arguments) []string {
	var names []string
	if !args.DisableDefaultMetrics {
		names = append(names, args.EnabledCollectors...)
		// Before v0.20.1 these groups ran even when enabled_collectors selected
		// only stat_statements. The slot group supplied the active/lag metrics;
		// its replacement also includes the formerly optional slot metrics.
		// The rename and merge are in https://github.com/prometheus-community/postgres_exporter/pull/1307.
		names = append(names, "stat_activity", "stat_archiver", "stat_replication", "replication_slots")
	}
	// The old DisableSettingsMetrics option was independent of
	// DisableDefaultMetrics. Keep settings enabled when only defaults are disabled.
	// The option was removed in https://github.com/prometheus-community/postgres_exporter/pull/1303.
	if !args.DisableSettingsMetrics {
		names = append(names, "settings")
	}

	// The legacy exporter let a custom pg_* namespace replace its default
	// query. Since v0.20.1 standalone collectors do not read that file, suppress
	// a migrated group when its namespace is present to avoid overlapping metrics.
	var overrides pg_exporter.UserQueries
	if args.CustomQueriesConfigPath != "" {
		if data, err := os.ReadFile(args.CustomQueriesConfigPath); err == nil {
			// Invalid/unreadable files are reported by the exporter's load-error metric,
			// as before. They must not prevent the default collectors from starting.
			if err := yaml.Unmarshal(data, &overrides); err != nil {
				overrides = nil
			}
		}
	}
	result := make([]string, 0, len(names))
	for _, name := range names {
		if name == "settings" && args.DisableSettingsMetrics {
			continue
		}
		switch name {
		case "stat_activity", "stat_archiver", "stat_replication", "replication_slots":
			if _, replaced := overrides["pg_"+name]; replaced {
				continue
			}
		}
		if !slices.Contains(result, name) {
			result = append(result, name)
		}
	}
	return result
}

// embeddedExporterServerLabel follows the legacy exporter's host:port identity,
// including its localhost and 5432 defaults, without including credentials.
func embeddedExporterServerLabel(dsn string) (string, error) {
	fields, err := collector.ParseURL(dsn)
	if err != nil {
		return "", fmt.Errorf("cannot parse embedded exporter DSN: %w", err)
	}
	host, port := fields["host"], fields["port"]
	if host == "" {
		host = "localhost"
	}
	if port == "" {
		port = "5432"
	}
	return host + ":" + port, nil
}
