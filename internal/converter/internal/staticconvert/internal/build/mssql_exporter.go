package build

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/mssql"
	mssql_exporter "github.com/grafana/alloy/internal/static/integrations/mssql"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func (b *ConfigBuilder) appendMssqlExporter(config *mssql_exporter.Config, instanceKey *string) discovery.Exports {
	args := toMssqlExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "mssql")
}

func toMssqlExporter(config *mssql_exporter.Config) *mssql.Arguments {
	return &mssql.Arguments{
		ConnectionString:   alloytypes.Secret(config.ConnectionString),
		ConnectionName:     config.ConnectionName,
		MaxIdleConnections: config.MaxIdleConnections,
		MaxOpenConnections: config.MaxOpenConnections,
		Timeout:            config.Timeout,
	}
}
