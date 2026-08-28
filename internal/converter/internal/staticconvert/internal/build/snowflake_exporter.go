package build

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/snowflake"
	"github.com/grafana/alloy/internal/static/integrations/snowflake_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func (b *ConfigBuilder) appendSnowflakeExporter(config *snowflake_exporter.Config, instanceKey *string) discovery.Exports {
	args := toSnowflakeExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "snowflake")
}

func toSnowflakeExporter(config *snowflake_exporter.Config) *snowflake.Arguments {
	return &snowflake.Arguments{
		AccountName:           config.AccountName,
		Username:              config.Username,
		Password:              alloytypes.Secret(config.Password),
		PrivateKeyPath:        config.PrivateKeyPath,
		PrivateKeyPassword:    alloytypes.Secret(config.PrivateKeyPassword),
		Role:                  config.Role,
		Warehouse:             config.Warehouse,
		ExcludeDeletedTables:  config.ExcludeDeletedTables,
		EnableDriverTraceLogs: config.EnableDriverTraceLogs,
	}
}
