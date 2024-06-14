// Package kafka provides an otelcol.exporter.kafka component.
package kafka

import (
	"time"

	"github.com/IBM/sarama"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.kafka",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := kafkaexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeAll)
		},
	})
}

// Arguments configures the otelcol.exporter.kafka component.
type Arguments struct {
	ProtocolVersion                      string        `alloy:"protocol_version,attr"`
	Brokers                              []string      `alloy:"brokers,attr,optional"`
	ResolveCanonicalBootstrapServersOnly bool          `alloy:"resolve_canonical_bootstrap_servers_only,attr,optional"`
	ClientID                             string        `alloy:"client_id,attr,optional"`
	Topic                                string        `alloy:"topic,attr,optional"`
	TopicFromAttribute                   string        `alloy:"topic_from_attribute,attr,optional"`
	Encoding                             string        `alloy:"encoding,attr,optional"`
	PartitionTracesByID                  bool          `alloy:"partition_traces_by_id,attr,optional"`
	PartitionMetricsByResourceAttributes bool          `alloy:"partition_metrics_by_resource_attributes,attr,optional"`
	Timeout                              time.Duration `alloy:"timeout,attr,optional"`

	Authentication otelcol.KafkaAuthenticationArguments `alloy:"authentication,block,optional"`
	Metadata       otelcol.KafkaMetadataArguments       `alloy:"metadata,block,optional"`
	Retry          otelcol.RetryArguments               `alloy:"retry_on_failure,block,optional"`
	Queue          otelcol.QueueArguments               `alloy:"sending_queue,block,optional"`
	Producer       Producer                             `alloy:"producer,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// Producer defines configuration for producer
type Producer struct {
	// Maximum message bytes the producer will accept to produce.
	MaxMessageBytes int `alloy:"max_message_bytes,attr,optional"`

	// RequiredAcks Number of acknowledgements required to assume that a message has been sent.
	// https://pkg.go.dev/github.com/IBM/sarama@v1.30.0#RequiredAcks
	// The options are:
	//   0 -> NoResponse.  doesn't send any response
	//   1 -> WaitForLocal. waits for only the local commit to succeed before responding ( default )
	//   -1 -> WaitForAll. waits for all in-sync replicas to commit before responding.
	RequiredAcks int `alloy:"required_acks,attr,optional"`

	// Compression Codec used to produce messages
	// https://pkg.go.dev/github.com/IBM/sarama@v1.30.0#CompressionCodec
	// The options are: 'none', 'gzip', 'snappy', 'lz4', and 'zstd'
	Compression string `alloy:"compression,attr,optional"`

	// The maximum number of messages the producer will send in a single
	// broker request. Defaults to 0 for unlimited. Similar to
	// `queue.buffering.max.messages` in the JVM producer.
	FlushMaxMessages int `alloy:"flush_max_messages,attr,optional"`
}

// Convert converts args into the upstream type.
func (args Producer) Convert() kafkaexporter.Producer {
	return kafkaexporter.Producer{
		MaxMessageBytes:  args.MaxMessageBytes,
		RequiredAcks:     sarama.RequiredAcks(args.RequiredAcks),
		Compression:      args.Compression,
		FlushMaxMessages: args.FlushMaxMessages,
	}
}

var (
	_ syntax.Validator   = (*Arguments)(nil)
	_ syntax.Defaulter   = (*Arguments)(nil)
	_ exporter.Arguments = (*Arguments)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Encoding: "otlp_proto",
		Brokers:  []string{"localhost:9092"},
		ClientID: "sarama",
		Timeout:  5 * time.Second,
		Metadata: otelcol.KafkaMetadataArguments{
			IncludeAllTopics: true,
			Retry: otelcol.KafkaMetadataRetryArguments{
				MaxRetries: 3,
				Backoff:    250 * time.Millisecond,
			},
		},
		Producer: Producer{
			MaxMessageBytes:  1000000,
			RequiredAcks:     1,
			Compression:      "none",
			FlushMaxMessages: 0,
		},
	}
	args.Retry.SetToDefault()
	args.Queue.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	otelCfg, err := args.Convert()
	if err != nil {
		return err
	}
	kafkaCfg := otelCfg.(*kafkaexporter.Config)
	return kafkaCfg.Validate()
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	input := make(map[string]interface{})
	input["auth"] = args.Authentication.Convert()

	var result kafkaexporter.Config
	err := mapstructure.Decode(input, &result)
	if err != nil {
		return nil, err
	}

	result.Brokers = args.Brokers
	result.ResolveCanonicalBootstrapServersOnly = args.ResolveCanonicalBootstrapServersOnly
	result.ProtocolVersion = args.ProtocolVersion
	result.ClientID = args.ClientID
	result.Topic = args.Topic
	result.TopicFromAttribute = args.TopicFromAttribute
	result.Encoding = args.Encoding
	result.PartitionTracesByID = args.PartitionTracesByID
	result.PartitionMetricsByResourceAttributes = args.PartitionMetricsByResourceAttributes
	result.TimeoutSettings = exporterhelper.TimeoutSettings{
		Timeout: args.Timeout,
	}
	result.Metadata = args.Metadata.Convert()
	result.BackOffConfig = *args.Retry.Convert()
	result.QueueSettings = *args.Queue.Convert()
	result.Producer = args.Producer.Convert()

	return &result, nil
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements exporter.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
