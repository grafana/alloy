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
	switch args.(Arguments).Encoding {
	case "raw":
		return exporter.TypeLogs
	case "jaeger_proto", "jaeger_json", "zipkin_proto", "zipkin_json":
		return exporter.TypeTraces
	default:
		return exporter.TypeAll
	}
}

// Arguments configures the otelcol.exporter.kafka component.
type Arguments struct {
	ProtocolVersion                      string        `alloy:"protocol_version,attr"`
	Brokers                              []string      `alloy:"brokers,attr,optional"`
	ResolveCanonicalBootstrapServersOnly bool          `alloy:"resolve_canonical_bootstrap_servers_only,attr,optional"`
	ClientID                             string        `alloy:"client_id,attr,optional"`
	Topic                                string        `alloy:"topic,attr,optional"` // Deprecated
	TopicFromAttribute                   string        `alloy:"topic_from_attribute,attr,optional"`
	Encoding                             string        `alloy:"encoding,attr,optional"`
	PartitionTracesByID                  bool          `alloy:"partition_traces_by_id,attr,optional"`
	PartitionMetricsByResourceAttributes bool          `alloy:"partition_metrics_by_resource_attributes,attr,optional"`
	Timeout                              time.Duration `alloy:"timeout,attr,optional"`

	Logs    KafkaExporterSignalConfig `alloy:"logs,block,optional"`
	Metrics KafkaExporterSignalConfig `alloy:"metrics,block,optional"`
	Traces  KafkaExporterSignalConfig `alloy:"traces,block,optional"`

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
	Topic    string `alloy:"topic,attr"`
	Encoding string `alloy:"encoding,attr,optional"`
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
func (args Producer) Convert() configkafka.ProducerConfig {
	return configkafka.ProducerConfig{
		MaxMessageBytes:  args.MaxMessageBytes,
		RequiredAcks:     configkafka.RequiredAcks(args.RequiredAcks),
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
		// Do not set the encoding argument - it is deprecated.
		// Encoding: "otlp_proto",
		Brokers:  []string{"localhost:9092"},
		ClientID: "sarama",
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
			MaxMessageBytes:  1000000,
			RequiredAcks:     1,
			Compression:      "none",
			FlushMaxMessages: 0,
		},
		Logs: KafkaExporterSignalConfig{
			Topic:    "otlp_logs",
			Encoding: "otlp_proto",
		},
		Metrics: KafkaExporterSignalConfig{
			Topic:    "otlp_metrics",
			Encoding: "otlp_proto",
		},
		Traces: KafkaExporterSignalConfig{
			Topic:    "otlp_spans",
			Encoding: "otlp_proto",
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
	result.TopicFromAttribute = args.TopicFromAttribute
	// Do not set the encoding argument - it is deprecated.
	// result.Encoding = args.Encoding
	result.PartitionTracesByID = args.PartitionTracesByID
	result.PartitionMetricsByResourceAttributes = args.PartitionMetricsByResourceAttributes
	result.TimeoutSettings = exporterhelper.TimeoutConfig{
		Timeout: args.Timeout,
	}
	result.Metadata = args.Metadata.Convert()
	result.BackOffConfig = *args.Retry.Convert()

	if args.Logs.Topic != "" {
		result.Logs.Topic = args.Logs.Topic
	} else if args.Topic != "" {
		result.Logs.Topic = args.Topic
	} else {
		result.Logs.Topic = "otlp_logs"
	}

	if args.Metrics.Topic != "" {
		result.Metrics.Topic = args.Metrics.Topic
	} else if args.Topic != "" {
		result.Metrics.Topic = args.Topic
	} else {
		result.Metrics.Topic = "otlp_metrics"
	}

	if args.Traces.Topic != "" {
		result.Traces.Topic = args.Traces.Topic
	} else if args.Topic != "" {
		result.Traces.Topic = args.Topic
	} else {
		result.Traces.Topic = "otlp_spans"
	}

	if args.Logs.Encoding != "" {
		result.Logs.Encoding = args.Logs.Encoding
	} else if args.Encoding != "" {
		result.Logs.Encoding = args.Encoding
	} else {
		result.Logs.Encoding = "otlp_proto"
	}

	if args.Metrics.Encoding != "" {
		result.Metrics.Encoding = args.Metrics.Encoding
	} else if args.Encoding != "" {
		result.Metrics.Encoding = args.Encoding
	} else {
		result.Metrics.Encoding = "otlp_proto"
	}

	if args.Traces.Encoding != "" {
		result.Traces.Encoding = args.Traces.Encoding
	} else if args.Encoding != "" {
		result.Traces.Encoding = args.Encoding
	} else {
		result.Traces.Encoding = "otlp_proto"
	}

	if args.TLS != nil {
		tlsCfg := args.TLS.Convert()
		result.TLS = tlsCfg
	}

	q, err := args.Queue.Convert()
	if err != nil {
		return nil, err
	}
	result.QueueSettings = *q
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
