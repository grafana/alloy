// Package kafka provides an otelcol.exporter.kafka component.
package kafka

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/confmap/xconfmap"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.kafka",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := kafkaexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), GetSignalType)
		},
	})
}

func GetSignalType(opts component.Options, args component.Arguments) exporter.TypeSignal {
	var signal exporter.TypeSignal
	signal = 0

	arguments := args.(Arguments)
	switch arguments.Encoding {
	case "raw":
		signal = exporter.TypeLogs
	case "jaeger_proto", "jaeger_json", "zipkin_proto", "zipkin_json":
		signal = exporter.TypeTraces
	case "otlp_proto", "otlp_json":
		signal = exporter.TypeAll
	}

	if arguments.Logs != nil {
		signal |= exporter.TypeLogs
	}
	if arguments.Metrics != nil {
		signal |= exporter.TypeMetrics
	}
	if arguments.Traces != nil {
		signal |= exporter.TypeTraces
	}

	return signal
}

// Arguments configures the otelcol.exporter.kafka component.
type Arguments struct {
	ProtocolVersion                      string        `alloy:"protocol_version,attr"`
	Brokers                              []string      `alloy:"brokers,attr,optional"`
	ResolveCanonicalBootstrapServersOnly bool          `alloy:"resolve_canonical_bootstrap_servers_only,attr,optional"`
	ClientID                             string        `alloy:"client_id,attr,optional"`
	Topic                                string        `alloy:"topic,attr,optional"` // Deprecated
	TopicFromAttribute                   string        `alloy:"topic_from_attribute,attr,optional"`
	Encoding                             string        `alloy:"encoding,attr,optional"` // Deprecated
	PartitionTracesByID                  bool          `alloy:"partition_traces_by_id,attr,optional"`
	PartitionMetricsByResourceAttributes bool          `alloy:"partition_metrics_by_resource_attributes,attr,optional"`
	PartitionLogsByResourceAttributes    bool          `alloy:"partition_logs_by_resource_attributes,attr,optional"`
	PartitionLogsByTraceID               bool          `alloy:"partition_logs_by_trace_id,attr,optional"`
	Timeout                              time.Duration `alloy:"timeout,attr,optional"`
	IncludeMetadataKeys                  []string      `alloy:"include_metadata_keys,attr,optional"`

	Logs    *KafkaExporterSignalConfig `alloy:"logs,block,optional"`
	Metrics *KafkaExporterSignalConfig `alloy:"metrics,block,optional"`
	Traces  *KafkaExporterSignalConfig `alloy:"traces,block,optional"`

	Authentication otelcol.KafkaAuthenticationArguments `alloy:"authentication,block,optional"`
	Metadata       otelcol.KafkaMetadataArguments       `alloy:"metadata,block,optional"`
	Retry          otelcol.RetryArguments               `alloy:"retry_on_failure,block,optional"`
	Queue          otelcol.QueueArguments               `alloy:"sending_queue,block,optional"`
	Producer       Producer                             `alloy:"producer,block,optional"`
	TLS            *otelcol.TLSClientArguments          `alloy:"tls,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

type KafkaExporterSignalConfig struct {
	Topic                string `alloy:"topic,attr,optional"`
	TopicFromMetadataKey string `alloy:"topic_from_metadata_key,attr,optional"`
	Encoding             string `alloy:"encoding,attr,optional"`
}

// A utility struct for handling deprecated arguments.
type deprecatedArg struct {
	// The value to which the deprecated argument is set.
	value string

	// The default value to use if neither the deprecated argument
	// nor the "new" argument have a non-empty value.
	defaultValue string
}

func (c *KafkaExporterSignalConfig) convert(topic, encoding deprecatedArg) kafkaexporter.SignalConfig {
	result := kafkaexporter.SignalConfig{}

	if c != nil { // Use values from the new block if set.
		if len(c.Topic) > 0 {
			result.Topic = c.Topic
		}
		if len(c.Encoding) > 0 {
			result.Encoding = c.Encoding
		}
		result.TopicFromMetadataKey = c.TopicFromMetadataKey
	} else { // Try to use deprecated attributes only if the new block is not set.
		if len(topic.value) > 0 {
			result.Topic = topic.value
		}

		if len(encoding.value) > 0 {
			result.Encoding = encoding.value
		}
	}

	if len(result.Topic) == 0 {
		result.Topic = topic.defaultValue
	}
	if len(result.Encoding) == 0 {
		result.Encoding = encoding.defaultValue
	}

	return result
}

// Producer defines configuration for producer
type Producer struct {
	// Maximum message bytes the producer will accept to produce.
	MaxMessageBytes int `alloy:"max_message_bytes,attr,optional"`

	// RequiredAcks Number of acknowledgements required to assume that a message has been sent.
	// https://docs.confluent.io/platform/current/installation/configuration/producer-configs.html#acks
	// The options are:
	//   0 -> NoResponse.  doesn't send any response
	//   1 -> WaitForLocal. waits for only the local commit to succeed before responding ( default )
	//   -1 -> WaitForAll. waits for all in-sync replicas to commit before responding.
	RequiredAcks int `alloy:"required_acks,attr,optional"`

	// Compression Codec used to produce messages
	// https://pkg.go.dev/github.com/IBM/sarama@v1.30.0#CompressionCodec
	// The options are: 'none', 'gzip', 'snappy', 'lz4', and 'zstd'
	Compression string `alloy:"compression,attr,optional"`

	// CompressionParams defines parameters for compression codec.
	CompressionParams CompressionParams `alloy:"compression_params,block,optional"`

	// The maximum number of messages the producer will send in a single
	// broker request. Defaults to 0 for unlimited. Similar to
	// `queue.buffering.max.messages` in the JVM producer.
	FlushMaxMessages int `alloy:"flush_max_messages,attr,optional"`

	// Whether or not to allow automatic topic creation.
	AllowAutoTopicCreation bool `alloy:"allow_auto_topic_creation,attr,optional"`
}

// Convert converts args into the upstream type.
func (args Producer) Convert() configkafka.ProducerConfig {
	return configkafka.ProducerConfig{
		MaxMessageBytes:        args.MaxMessageBytes,
		RequiredAcks:           configkafka.RequiredAcks(args.RequiredAcks),
		Compression:            args.Compression,
		CompressionParams:      args.CompressionParams.Convert(),
		FlushMaxMessages:       args.FlushMaxMessages,
		AllowAutoTopicCreation: args.AllowAutoTopicCreation,
	}
}

type CompressionParams struct {
	Level int `alloy:"level,attr,optional"`
}

func (c *CompressionParams) Convert() configcompression.CompressionParams {
	return configcompression.CompressionParams{
		Level: configcompression.Level(c.Level),
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
		Brokers:  []string{"localhost:9092"},
		ClientID: "otel-collector",
		Timeout:  5 * time.Second,
		Metadata: otelcol.KafkaMetadataArguments{
			Full:            true,
			RefreshInterval: 10 * time.Minute,
			Retry: otelcol.KafkaMetadataRetryArguments{
				MaxRetries: 3,
				Backoff:    250 * time.Millisecond,
			},
		},
		Producer: Producer{
			MaxMessageBytes: 1000000,
			RequiredAcks:    1,
			Compression:     "none",
			CompressionParams: CompressionParams{
				Level: 0, // Default compression level
			},
			FlushMaxMessages:       0,
			AllowAutoTopicCreation: true,
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
	return xconfmap.Validate(kafkaCfg)
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	input := make(map[string]any)
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
	result.TopicFromAttribute = args.TopicFromAttribute
	// Do not set the encoding argument - it is deprecated.
	// result.Encoding = args.Encoding
	result.PartitionTracesByID = args.PartitionTracesByID
	result.PartitionMetricsByResourceAttributes = args.PartitionMetricsByResourceAttributes
	result.PartitionLogsByResourceAttributes = args.PartitionLogsByResourceAttributes
	result.PartitionLogsByTraceID = args.PartitionLogsByTraceID
	result.IncludeMetadataKeys = args.IncludeMetadataKeys
	result.TimeoutSettings = exporterhelper.TimeoutConfig{
		Timeout: args.Timeout,
	}
	result.Metadata = args.Metadata.Convert()
	result.BackOffConfig = *args.Retry.Convert()

	result.Logs = args.Logs.convert(
		deprecatedArg{
			value:        args.Topic,
			defaultValue: "otlp_logs",
		},
		deprecatedArg{
			value:        args.Encoding,
			defaultValue: "otlp_proto",
		},
	)

	result.Metrics = args.Metrics.convert(
		deprecatedArg{
			value:        args.Topic,
			defaultValue: "otlp_metrics",
		},
		deprecatedArg{
			value:        args.Encoding,
			defaultValue: "otlp_proto",
		},
	)

	result.Traces = args.Traces.convert(
		deprecatedArg{
			value:        args.Topic,
			defaultValue: "otlp_spans",
		},
		deprecatedArg{
			value:        args.Encoding,
			defaultValue: "otlp_proto",
		},
	)

	if args.TLS != nil {
		tlsCfg := args.TLS.Convert()
		result.TLS = tlsCfg
	}

	q, err := args.Queue.Convert()
	if err != nil {
		return nil, err
	}
	result.QueueBatchConfig = q
	result.Producer = args.Producer.Convert()

	if args.TLS != nil {
		tlsCfg := args.TLS.Convert()
		result.TLS = tlsCfg
	}

	return &result, nil
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return args.Queue.Extensions()
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements exporter.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
