package build

import (
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/prometheus/exporter/dnsmasq"
	"github.com/grafana/agent/internal/static/integrations/dnsmasq_exporter"
)

func (b *ConfigBuilder) appendDnsmasqExporter(config *dnsmasq_exporter.Config, instanceKey *string) discovery.Exports {
	args := toDnsmasqExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "dnsmasq")
}

func toDnsmasqExporter(config *dnsmasq_exporter.Config) *dnsmasq.Arguments {
	return &dnsmasq.Arguments{
		Address:      config.DnsmasqAddress,
		LeasesFile:   config.LeasesPath,
		ExposeLeases: config.ExposeLeases,
	}
}
