package build

import (
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/prometheus/exporter/self"
	agent_exporter "github.com/grafana/agent/internal/static/integrations/agent"
	agent_exporter_v2 "github.com/grafana/agent/internal/static/integrations/v2/agent"
)

func (b *ConfigBuilder) appendAgentExporter(config *agent_exporter.Config) discovery.Exports {
	args := toAgentExporter(config)
	return b.appendExporterBlock(args, config.Name(), nil, "self")
}

func toAgentExporter(config *agent_exporter.Config) *self.Arguments {
	return &self.Arguments{}
}

func (b *ConfigBuilder) appendAgentExporterV2(config *agent_exporter_v2.Config) discovery.Exports {
	args := toAgentExporterV2(config)
	return b.appendExporterBlock(args, config.Name(), config.Common.InstanceKey, "self")
}

func toAgentExporterV2(config *agent_exporter_v2.Config) *self.Arguments {
	return &self.Arguments{}
}
