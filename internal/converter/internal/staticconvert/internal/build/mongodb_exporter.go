package build

import (
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/prometheus/exporter/mongodb"
	"github.com/grafana/agent/internal/static/integrations/mongodb_exporter"
	"github.com/grafana/river/rivertypes"
)

func (b *ConfigBuilder) appendMongodbExporter(config *mongodb_exporter.Config, instanceKey *string) discovery.Exports {
	args := toMongodbExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "mongodb")
}

func toMongodbExporter(config *mongodb_exporter.Config) *mongodb.Arguments {
	return &mongodb.Arguments{
		URI:                    rivertypes.Secret(config.URI),
		DirectConnect:          config.DirectConnect,
		DiscoveringMode:        config.DiscoveringMode,
		TLSBasicAuthConfigPath: config.TLSBasicAuthConfigPath,
	}
}
