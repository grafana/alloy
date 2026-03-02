package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/kafka"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/config/configcompression"
)

func init() {
	converters = append(converters, kafkaExporterConverter{})
}

type kafkaExporterConverter struct{}

func (kafkaExporterConverter) Factory() component.Factory { return kafkaexporter.NewFactory() }

func (kafkaExporterConverter) InputComponentName() string {
	return "otelcol.exporter.kafka"
}

func (kafkaExporterConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()
	overrideHook := func(val any) any {
		switch val.(type) {
		case extension.ExtensionHandler:
			queue := cfg.(*kafkaexporter.Config).QueueBatchConfig.GetOrInsertDefault()
			ext := state.LookupExtension(*queue.StorageID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		}
		return common.GetAlloyTypesOverrideHook()(val)
	}

	args := toKafkaExporter(cfg.(*kafkaexporter.Config))
	block := common.NewBlockWithOverrideFn([]string{"otelcol", "exporter", "kafka"}, label, args, overrideHook)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toKafkaExporter(cfg *kafkaexporter.Config) *kafka.Arguments {
	var tlsCfgPtr *otelcol.TLSClientArguments
	if cfg.TLS != nil {
		tlsCfg := toTLSClientArguments(*cfg.TLS)
		tlsCfgPtr = &tlsCfg
	}

	return &kafka.Arguments{
		Brokers:                              cfg.Brokers,
		ProtocolVersion:                      cfg.ProtocolVersion,
		ResolveCanonicalBootstrapServersOnly: cfg.ResolveCanonicalBootstrapServersOnly,
		ClientID:                             cfg.ClientID,
		Topic:                                cfg.Topic,
		TopicFromAttribute:                   cfg.TopicFromAttribute,
		Encoding:                             cfg.Encoding,
		PartitionTracesByID:                  cfg.PartitionTracesByID,
		PartitionMetricsByResourceAttributes: cfg.PartitionMetricsByResourceAttributes,
		PartitionLogsByResourceAttributes:    cfg.PartitionLogsByResourceAttributes,
		PartitionLogsByTraceID:               cfg.PartitionLogsByTraceID,
		IncludeMetadataKeys:                  cfg.IncludeMetadataKeys,
		Timeout:                              cfg.TimeoutSettings.Timeout,

		Logs:    toKafkaSignalConfig(cfg.Logs),
		Metrics: toKafkaSignalConfig(cfg.Metrics),
		Traces:  toKafkaSignalConfig(cfg.Traces),

		Authentication: toKafkaAuthentication(encodeMapstruct(cfg.Authentication)),
		Metadata:       toKafkaMetadata(cfg.Metadata),
		Retry:          toRetryArguments(cfg.BackOffConfig),
		Queue:          toQueueArguments(cfg.QueueBatchConfig),
		Producer:       toKafkaProducer(cfg.Producer),

		TLS: tlsCfgPtr,

		DebugMetrics: common.DefaultValue[kafka.Arguments]().DebugMetrics,
	}
}

func toKafkaProducer(cfg configkafka.ProducerConfig) kafka.Producer {
	return kafka.Producer{
		MaxMessageBytes:        cfg.MaxMessageBytes,
		Compression:            cfg.Compression,
		CompressionParams:      toKafkaCompressionParams(cfg.CompressionParams),
		RequiredAcks:           int(cfg.RequiredAcks),
		FlushMaxMessages:       cfg.FlushMaxMessages,
		AllowAutoTopicCreation: cfg.AllowAutoTopicCreation,
	}
}

func toKafkaCompressionParams(cfg configcompression.CompressionParams) kafka.CompressionParams {
	return kafka.CompressionParams{
		Level: int(cfg.Level),
	}
}

func toKafkaSignalConfig(cfg kafkaexporter.SignalConfig) *kafka.KafkaExporterSignalConfig {
	return &kafka.KafkaExporterSignalConfig{
		Topic:                cfg.Topic,
		TopicFromMetadataKey: cfg.TopicFromMetadataKey,
		Encoding:             cfg.Encoding,
	}
}
