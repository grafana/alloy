package build

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/oracledb"
	"github.com/grafana/alloy/internal/static/integrations/oracledb_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func (b *ConfigBuilder) appendOracledbExporter(config *oracledb_exporter.Config, instanceKey *string) discovery.Exports {
	args := toOracledbExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "oracledb")
}

func toOracledbExporter(config *oracledb_exporter.Config) *oracledb.Arguments {
	return &oracledb.Arguments{
		ConnectionString: alloytypes.Secret(config.ConnectionString),
		MaxIdleConns:     config.MaxIdleConns,
		MaxOpenConns:     config.MaxOpenConns,
		QueryTimeout:     config.QueryTimeout,
	}
}
