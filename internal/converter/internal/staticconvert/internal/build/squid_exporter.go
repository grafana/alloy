package build

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/squid"
	"github.com/grafana/alloy/internal/static/integrations/squid_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func (b *ConfigBuilder) appendSquidExporter(config *squid_exporter.Config, instanceKey *string) discovery.Exports {
	args := toSquidExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "squid")
}

func toSquidExporter(config *squid_exporter.Config) *squid.Arguments {
	return &squid.Arguments{
		SquidAddr:     config.Address,
		SquidUser:     config.Username,
		SquidPassword: alloytypes.Secret(config.Password),
	}
}
