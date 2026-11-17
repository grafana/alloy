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

// newEmbeddedExporterCollector preserves database observability's effective
// collector coverage when legacy exporter groups move into standalone collectors.
func newEmbeddedExporterCollector(logger *slog.Logger, args exporter_postgres.Arguments, dsn string) (prometheus.Collector, error) {
	names := embeddedExporterCollectorNames(args)
	if len(names) == 0 {
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
	// Keep the server identity previously supplied by the legacy exporter.
	return prometheus.WrapCollectorWith(prometheus.Labels{"server": server}, col), nil
}

// embeddedExporterCollectorNames preserves the groups collected implicitly by
// the legacy exporter, independently of the standalone collector allowlist.
func embeddedExporterCollectorNames(args exporter_postgres.Arguments) []string {
	var names []string
	if !args.DisableDefaultMetrics {
		names = append(names, args.EnabledCollectors...)
		names = append(names, "stat_activity", "stat_archiver", "stat_replication", "replication_slots")
	}
	// Settings were controlled independently, even with default metrics disabled.
	if !args.DisableSettingsMetrics {
		names = append(names, "settings")
	}

	// The legacy exporter lets a custom namespace replace its corresponding
	// default query. Avoid collecting the migrated default alongside that query.
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
