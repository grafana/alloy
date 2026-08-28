package build

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/self"
	agent_exporter "github.com/grafana/alloy/internal/static/integrations/agent"
	agent_exporter_v2 "github.com/grafana/alloy/internal/static/integrations/v2/agent"
)

func (b *ConfigBuilder) appendAgentExporter(config *agent_exporter.Config) discovery.Exports {
	args := toAgentExporter()
	return b.appendExporterBlock(args, config.Name(), nil, "self")
}

func toAgentExporter() *self.Arguments {
	return &self.Arguments{}
}

func (b *ConfigBuilder) appendAgentExporterV2(config *agent_exporter_v2.Config) discovery.Exports {
	args := toAgentExporterV2()
	return b.appendExporterBlock(args, config.Name(), config.Common.InstanceKey, "self")
}

func toAgentExporterV2() *self.Arguments {
	return &self.Arguments{}
}
