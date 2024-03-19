package build

import (
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/prometheus/exporter/snowflake"
	"github.com/grafana/agent/internal/static/integrations/snowflake_exporter"
	"github.com/grafana/river/rivertypes"
)

func (b *ConfigBuilder) appendSnowflakeExporter(config *snowflake_exporter.Config, instanceKey *string) discovery.Exports {
	args := toSnowflakeExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "snowflake")
}

func toSnowflakeExporter(config *snowflake_exporter.Config) *snowflake.Arguments {
	return &snowflake.Arguments{
		AccountName: config.AccountName,
		Username:    config.Username,
		Password:    rivertypes.Secret(config.Password),
		Role:        config.Role,
		Warehouse:   config.Warehouse,
	}
}
