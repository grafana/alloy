package build

import (
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/prometheus/exporter/apache"
	"github.com/grafana/agent/internal/static/integrations/apache_http"
	apache_exporter_v2 "github.com/grafana/agent/internal/static/integrations/v2/apache_http"
)

func (b *ConfigBuilder) appendApacheExporter(config *apache_http.Config) discovery.Exports {
	args := toApacheExporter(config)
	return b.appendExporterBlock(args, config.Name(), nil, "apache")
}

func toApacheExporter(config *apache_http.Config) *apache.Arguments {
	return &apache.Arguments{
		ApacheAddr:         config.ApacheAddr,
		ApacheHostOverride: config.ApacheHostOverride,
		ApacheInsecure:     config.ApacheInsecure,
	}
}

func (b *ConfigBuilder) appendApacheExporterV2(config *apache_exporter_v2.Config) discovery.Exports {
	args := toApacheExporterV2(config)
	return b.appendExporterBlock(args, config.Name(), config.Common.InstanceKey, "apache")
}

func toApacheExporterV2(config *apache_exporter_v2.Config) *apache.Arguments {
	return &apache.Arguments{
		ApacheAddr:         config.ApacheAddr,
		ApacheHostOverride: config.ApacheHostOverride,
		ApacheInsecure:     config.ApacheInsecure,
	}
}
