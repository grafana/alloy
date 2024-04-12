package build

import (
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus/exporter/kafka"
	"github.com/grafana/alloy/internal/static/integrations/kafka_exporter"
	"github.com/grafana/alloy/syntax/alloytypes"
)

func (b *ConfigBuilder) appendKafkaExporter(config *kafka_exporter.Config, instanceKey *string) discovery.Exports {
	args := toKafkaExporter(config)
	return b.appendExporterBlock(args, config.Name(), instanceKey, "kafka")
}

func toKafkaExporter(config *kafka_exporter.Config) *kafka.Arguments {
	return &kafka.Arguments{
		Instance:                config.Instance,
		KafkaURIs:               config.KafkaURIs,
		UseSASL:                 config.UseSASL,
		UseSASLHandshake:        config.UseSASLHandshake,
		SASLUsername:            config.SASLUsername,
		SASLPassword:            alloytypes.Secret(config.SASLPassword),
		SASLMechanism:           config.SASLMechanism,
		SASLDisablePAFXFast:     config.SASLDisablePAFXFast,
		UseTLS:                  config.UseTLS,
		TlsServerName:           config.TlsServerName,
		CAFile:                  config.CAFile,
		CertFile:                config.CertFile,
		KeyFile:                 config.KeyFile,
		InsecureSkipVerify:      config.InsecureSkipVerify,
		KafkaVersion:            config.KafkaVersion,
		UseZooKeeperLag:         config.UseZooKeeperLag,
		ZookeeperURIs:           config.ZookeeperURIs,
		ClusterName:             config.ClusterName,
		MetadataRefreshInterval: config.MetadataRefreshInterval,
		ServiceName:             config.ServiceName,
		KerberosConfigPath:      config.KerberosConfigPath,
		Realm:                   config.Realm,
		KeyTabPath:              config.KeyTabPath,
		KerberosAuthType:        config.KerberosAuthType,
		OffsetShowAll:           config.OffsetShowAll,
		TopicWorkers:            config.TopicWorkers,
		AllowConcurrent:         config.AllowConcurrent,
		AllowAutoTopicCreation:  config.AllowAutoTopicCreation,
		MaxOffsets:              config.MaxOffsets,
		PruneIntervalSeconds:    config.PruneIntervalSeconds,
		TopicsFilter:            config.TopicsFilter,
		TopicsExclude:           config.TopicsExclude,
		GroupFilter:             config.GroupFilter,
		GroupExclude:            config.GroupExclude,
	}
}
