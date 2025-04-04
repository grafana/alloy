package otelcolconvert

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/kafka"
	"github.com/grafana/alloy/internal/component/otelcol/extension"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
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
	overrideHook := func(val interface{}) interface{} {
		switch val.(type) {
		case extension.ExtensionHandler:
			ext := state.LookupExtension(*cfg.(*kafkaexporter.Config).QueueSettings.StorageID)
			return common.CustomTokenizer{Expr: fmt.Sprintf("%s.%s.handler", strings.Join(ext.Name, "."), ext.Label)}
		}
		return val
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
		Timeout:                              cfg.TimeoutSettings.Timeout,

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
