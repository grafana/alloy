package build

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/mongodb"
	"github.com/grafana/alloy/internal/static/integrations/mongodb_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func (b *ConfigBuilder) appendMongodbExporter(config *mongodb_exporter.Config, instanceKey *string) discovery.Exports {
	args := toMongodbExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "mongodb")
}

func toMongodbExporter(config *mongodb_exporter.Config) *mongodb.Arguments {
	return &mongodb.Arguments{
		URI:                      alloytypes.Secret(config.URI),
		CompatibleMode:           config.CompatibleMode,
		CollectAll:               config.CollectAll,
		DirectConnect:            config.DirectConnect,
		DiscoveringMode:          config.DiscoveringMode,
		EnableDBStats:            config.EnableDBStats,
		EnableDBStatsFreeStorage: config.EnableDBStatsFreeStorage,
		EnableDiagnosticData:     config.EnableDiagnosticData,
		EnableReplicasetStatus:   config.EnableReplicasetStatus,
		EnableReplicasetConfig:   config.EnableReplicasetConfig,
		EnableCurrentopMetrics:   config.EnableCurrentopMetrics,
		EnableTopMetrics:         config.EnableTopMetrics,
		EnableIndexStats:         config.EnableIndexStats,
		EnableCollStats:          config.EnableCollStats,
		EnableProfile:            config.EnableProfile,
		EnableShards:             config.EnableShards,
		EnableFCV:                config.EnableFCV,
		EnablePBMMetrics:         config.EnablePBMMetrics,
	}
}
