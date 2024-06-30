package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/kafka"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"go.opentelemetry.io/collector/component"
)

func init() {
	converters = append(converters, kafkaExporterConverter{})
}

type kafkaExporterConverter struct{}

func (kafkaExporterConverter) Factory() component.Factory { return kafkaexporter.NewFactory() }

func (kafkaExporterConverter) InputComponentName() string {
	return "otelcol.exporter.kafka"
}

func (kafkaExporterConverter) ConvertAndAppend(state *State, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toKafkaExporter(cfg.(*kafkaexporter.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "exporter", "kafka"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toKafkaExporter(cfg *kafkaexporter.Config) *kafka.Arguments {
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
		Timeout:                              cfg.Timeout,

		Authentication: toKafkaAuthentication(encodeMapstruct(cfg.Authentication)),
		Metadata:       toKafkaMetadata(cfg.Metadata),
		Retry:          toRetryArguments(cfg.BackOffConfig),
		Queue:          toQueueArguments(cfg.QueueSettings),
		Producer:       toKafkaProducer(cfg.Producer),

		DebugMetrics: common.DefaultValue[kafka.Arguments]().DebugMetrics,
	}
}

func toKafkaProducer(cfg kafkaexporter.Producer) kafka.Producer {
	return kafka.Producer{
		MaxMessageBytes:  cfg.MaxMessageBytes,
		Compression:      cfg.Compression,
		RequiredAcks:     int(cfg.RequiredAcks),
		FlushMaxMessages: cfg.FlushMaxMessages,
	}
}
