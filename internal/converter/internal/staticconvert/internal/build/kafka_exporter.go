package build

import (
	"github.com/grafana/agent/internal/component/discovery"
	"github.com/grafana/agent/internal/component/prometheus/exporter/kafka"
	"github.com/grafana/agent/internal/static/integrations/kafka_exporter"
	"github.com/grafana/river/rivertypes"
)

func (b *ConfigBuilder) appendKafkaExporter(config *kafka_exporter.Config, instanceKey *string) discovery.Exports {
	args := toKafkaExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "kafka")
}

func toKafkaExporter(config *kafka_exporter.Config) *kafka.Arguments {
	return &kafka.Arguments{
		KafkaURIs:               config.KafkaURIs,
		UseSASL:                 config.UseSASL,
		UseSASLHandshake:        config.UseSASLHandshake,
		SASLUsername:            config.SASLUsername,
		SASLPassword:            rivertypes.Secret(config.SASLPassword),
		SASLMechanism:           config.SASLMechanism,
		UseTLS:                  config.UseTLS,
		CAFile:                  config.CAFile,
		CertFile:                config.CertFile,
		KeyFile:                 config.KeyFile,
		InsecureSkipVerify:      config.InsecureSkipVerify,
		KafkaVersion:            config.KafkaVersion,
		UseZooKeeperLag:         config.UseZooKeeperLag,
		ZookeeperURIs:           config.ZookeeperURIs,
		ClusterName:             config.ClusterName,
		MetadataRefreshInterval: config.MetadataRefreshInterval,
		AllowConcurrent:         config.AllowConcurrent,
		MaxOffsets:              config.MaxOffsets,
		PruneIntervalSeconds:    config.PruneIntervalSeconds,
		TopicsFilter:            config.TopicsFilter,
		GroupFilter:             config.GroupFilter,
	}
}
