package build

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/catchpoint"
	"github.com/grafana/alloy/internal/static/integrations/catchpoint_exporter"
)

func (b *ConfigBuilder) appendCatchpointExporter(config *catchpoint_exporter.Config, instanceKey *string) discovery.Exports {
	args := toCatchpointExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "catchpoint")
}

func toCatchpointExporter(config *catchpoint_exporter.Config) *catchpoint.Arguments {
	return &catchpoint.Arguments{
		Port:           config.Port,
		VerboseLogging: config.VerboseLogging,
		WebhookPath:    config.WebhookPath,
	}
}
