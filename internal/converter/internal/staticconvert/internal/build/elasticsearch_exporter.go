package build

import (
	commonCfg "github.com/grafana/agent/internal/component/common/config"
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/prometheus/exporter/elasticsearch"
	"github.com/grafana/agent/internal/static/integrations/elasticsearch_exporter"
	"github.com/grafana/river/rivertypes"
)

func (b *ConfigBuilder) appendElasticsearchExporter(config *elasticsearch_exporter.Config, instanceKey *string) discovery.Exports {
	args := toElasticsearchExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "elasticsearch")
}

func toElasticsearchExporter(config *elasticsearch_exporter.Config) *elasticsearch.Arguments {
	arg := &elasticsearch.Arguments{
		Address:                   config.Address,
		Timeout:                   config.Timeout,
		AllNodes:                  config.AllNodes,
		Node:                      config.Node,
		ExportIndices:             config.ExportIndices,
		ExportIndicesSettings:     config.ExportIndicesSettings,
		ExportClusterSettings:     config.ExportClusterSettings,
		ExportShards:              config.ExportShards,
		IncludeAliases:            config.IncludeAliases,
		ExportSnapshots:           config.ExportSnapshots,
		ExportClusterInfoInterval: config.ExportClusterInfoInterval,
		CA:                        config.CA,
		ClientPrivateKey:          config.ClientPrivateKey,
		ClientCert:                config.ClientCert,
		InsecureSkipVerify:        config.InsecureSkipVerify,
		ExportDataStreams:         config.ExportDataStreams,
		ExportSLM:                 config.ExportSLM,
	}

	if config.BasicAuth != nil {
		arg.BasicAuth = &commonCfg.BasicAuth{
			Username:     config.BasicAuth.Username,
			Password:     rivertypes.Secret(config.BasicAuth.Password),
			PasswordFile: config.BasicAuth.PasswordFile,
		}
	}

	return arg
}
