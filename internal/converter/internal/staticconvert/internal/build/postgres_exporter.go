package build

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/postgres"
	"github.com/grafana/alloy/internal/static/integrations/postgres_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func (b *ConfigBuilder) appendPostgresExporter(config *postgres_exporter.Config, instanceKey *string) discovery.Exports {
	args := toPostgresExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "postgres")
}

func toPostgresExporter(config *postgres_exporter.Config) *postgres.Arguments {
	dataSourceNames := make([]alloytypes.Secret, 0)
	for _, dsn := range config.DataSourceNames {
		dataSourceNames = append(dataSourceNames, alloytypes.Secret(dsn))
	}

	return &postgres.Arguments{
		DataSourceNames:         dataSourceNames,
		DisableSettingsMetrics:  config.DisableSettingsMetrics,
		DisableDefaultMetrics:   config.DisableDefaultMetrics,
		CustomQueriesConfigPath: config.QueryPath,
		AutoDiscovery: postgres.AutoDiscovery{
			Enabled:           config.AutodiscoverDatabases,
			DatabaseAllowlist: config.IncludeDatabases,
			DatabaseDenylist:  config.ExcludeDatabases,
		},
		StatStatementFlags: postgres.StatStatementFlags{
			IncludeQuery: config.StatStatementIncludeQuery,
			QueryLength:  config.StatStatementQueryLength,
		},
	}
}
