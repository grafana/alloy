package kafka_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/kafka"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configcompression"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	defaultExpected := func() kafkaexporter.Config {
		return kafkaexporter.Config{
			TimeoutSettings: exporterhelper.TimeoutConfig{
				Timeout: 5 * time.Second,
			},
			QueueBatchConfig: configoptional.Some(exporterhelper.NewDefaultQueueConfig()),
			Logs: kafkaexporter.SignalConfig{
				Topic:                "otlp_logs",
				TopicFromMetadataKey: "",
				Encoding:             "otlp_proto",
			},
			Metrics: kafkaexporter.SignalConfig{
				Topic:                "otlp_metrics",
				TopicFromMetadataKey: "",
				Encoding:             "otlp_proto",
			},
			Traces: kafkaexporter.SignalConfig{
				Topic:                "otlp_spans",
				TopicFromMetadataKey: "",
				Encoding:             "otlp_proto",
			},
			Profiles: kafkaexporter.SignalConfig{
				Topic:                "",
				TopicFromMetadataKey: "",
				Encoding:             "",
			},
			Topic:                                "",
			IncludeMetadataKeys:                  []string(nil),
			TopicFromAttribute:                   "",
			Encoding:                             "",
			PartitionTracesByID:                  false,
			PartitionMetricsByResourceAttributes: false,
			PartitionLogsByResourceAttributes:    false,
			PartitionLogsByTraceID:               false,
			BackOffConfig: configretry.BackOffConfig{
				Enabled:             true,
				InitialInterval:     5 * time.Second,
				RandomizationFactor: 0.5,
				Multiplier:          1.5,
				MaxInterval:         30 * time.Second,
				MaxElapsedTime:      5 * time.Minute,
			},
			ClientConfig: configkafka.ClientConfig{
				Brokers:         []string{"localhost:9092"},
				ProtocolVersion: "2.0.0",
				ClientID:        "otel-collector",
				Metadata: configkafka.MetadataConfig{
					Full:            true,
					RefreshInterval: 10 * time.Minute,
					Retry: configkafka.MetadataRetryConfig{
						Max:     3,
						Backoff: 250 * time.Millisecond,
					},
				},
			},
			Producer: configkafka.ProducerConfig{
				MaxMessageBytes: 1000000,
				RequiredAcks:    1,
				Compression:     "none",
				CompressionParams: configcompression.CompressionParams{
					Level: 0,
				},
				AllowAutoTopicCreation: true,
			},
		}
	}

	tests := []struct {
		testName string
		cfg      string
		expected kafkaexporter.Config
	}{
		{
			testName: "Defaults",
			cfg: `
				protocol_version = "2.0.0"
			`,
			expected: defaultExpected(),
		},
		{
			testName: "Deprecated topic",
			cfg: `
				protocol_version = "2.0.0"
				topic = "test_default_topic"
				metrics {
					topic = "test_metrics_topic"
				}
			`,
			expected: func() kafkaexporter.Config {
				cfg := defaultExpected()

				cfg.Topic = ""
				cfg.Encoding = ""

				cfg.Logs.Topic = "test_default_topic"
				cfg.Logs.Encoding = "otlp_proto"

				cfg.Metrics.Topic = "test_metrics_topic"
				cfg.Metrics.Encoding = "otlp_proto"

				cfg.Traces.Topic = "test_default_topic"
				cfg.Traces.Encoding = "otlp_proto"

				return cfg
			}(),
		},
		{
			testName: "Deprecated encoding",
			cfg: `
				protocol_version = "2.0.0"
				encoding = "otlp_json"
				traces {
					encoding = "zipkin_thrift"
				}
			`,
			expected: func() kafkaexporter.Config {
				cfg := defaultExpected()

				cfg.Topic = ""
				cfg.Encoding = ""

				cfg.Logs.Topic = "otlp_logs"
				cfg.Logs.Encoding = "otlp_json"

				cfg.Metrics.Topic = "otlp_metrics"
				cfg.Metrics.Encoding = "otlp_json"

				cfg.Traces.Topic = "otlp_spans"
				cfg.Traces.Encoding = "zipkin_thrift"

				return cfg
			}(),
		},
		{
			testName: "Deprecated topic and empty blocks",
			cfg: `
				protocol_version = "2.0.0"

				// Neither "topic" nor "encoding" will be used,
				// because the default values from the enpty blocks should be used.
				// Making those blocks empty means their thefault values should be used,
				// and they have precedence over those deprecared arguments.
				topic = "test_default_topic"
				encoding = "otlp_json"

				metrics {}
				logs {}
				traces {}
			`,
			expected: defaultExpected(),
		},
		{
			testName: "Partition by resource attributes",
			cfg: `
				protocol_version = "2.0.0"
				partition_traces_by_id = true
				partition_metrics_by_resource_attributes = true
				partition_logs_by_resource_attributes = true
			`,
			expected: func() kafkaexporter.Config {
				cfg := defaultExpected()
				cfg.PartitionTracesByID = true
				cfg.PartitionMetricsByResourceAttributes = true
				cfg.PartitionLogsByResourceAttributes = true
				return cfg
			}(),
		},
		{
			testName: "Partition logs by trace id",
			cfg: `
				protocol_version = "2.0.0"
				partition_traces_by_id = true
				partition_metrics_by_resource_attributes = true
				partition_logs_by_trace_id = true
			`,
			expected: func() kafkaexporter.Config {
				cfg := defaultExpected()
				cfg.PartitionTracesByID = true
				cfg.PartitionMetricsByResourceAttributes = true
				cfg.PartitionLogsByTraceID = true
				return cfg
			}(),
		},
		{
			testName: "Explicit",
			cfg: `
				protocol_version = "2.0.0"
				brokers = ["redpanda:123"]
				resolve_canonical_bootstrap_servers_only = true
				client_id = "my-client"
				topic = ""
				topic_from_attribute = "my-attr"
				encoding = "otlp_json"
				partition_traces_by_id = true
				partition_metrics_by_resource_attributes = true
				partition_logs_by_resource_attributes = true
				timeout = "12s"

				authentication {
					plaintext {
						username = "user"
						password = "pass"
					}
				}

				metadata {
					full = false
					refresh_interval = "14s"
					retry {
						max_retries = 5
						backoff = "511ms"
					}
				}

				retry_on_failure {
					enabled = true
					initial_interval = "10s"
					randomization_factor = 0.1
					multiplier = 2.0
					max_interval = "61s"
					max_elapsed_time = "11m"
				}

				sending_queue {
					enabled = true
					num_consumers = 11
					queue_size = 1001
				}

				producer {
					max_message_bytes =  2000001
					required_acks = 0
					compression = "gzip"
					compression_params {
						level = 9
					}
					flush_max_messages = 101
				}

				logs {
					topic = "logs_test_topic"
					encoding = "raw"
				}
				metrics {
					topic = "metrics_test_topic"
					encoding = "otlp_json"
				}
				traces {
					topic = "spans_test_topic"
					encoding = "zipkin_json"
				}
			`,
			expected: kafkaexporter.Config{
				TimeoutSettings: exporterhelper.TimeoutConfig{
					Timeout: 12 * time.Second,
				},
				QueueBatchConfig: configoptional.Some(exporterhelper.QueueBatchConfig{
					NumConsumers: 11,
					QueueSize:    1001,
					Sizer:        exporterhelper.RequestSizerTypeRequests,
					Batch:        exporterhelper.NewDefaultQueueConfig().Batch,
				}),
				Logs: kafkaexporter.SignalConfig{
					Topic:                "logs_test_topic",
					TopicFromMetadataKey: "",
					Encoding:             "raw",
				},
				Metrics: kafkaexporter.SignalConfig{
					Topic:                "metrics_test_topic",
					TopicFromMetadataKey: "",
					Encoding:             "otlp_json",
				},
				Traces: kafkaexporter.SignalConfig{
					Topic:                "spans_test_topic",
					TopicFromMetadataKey: "",
					Encoding:             "zipkin_json",
				},
				Profiles: kafkaexporter.SignalConfig{
					Topic:                "",
					TopicFromMetadataKey: "",
					Encoding:             "",
				},
				BackOffConfig: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     10 * time.Second,
					RandomizationFactor: 0.1,
					Multiplier:          2.0,
					MaxInterval:         61 * time.Second,
					MaxElapsedTime:      11 * time.Minute,
				},
				ClientConfig: configkafka.ClientConfig{
					Brokers:                              []string{"redpanda:123"},
					ProtocolVersion:                      "2.0.0",
					ClientID:                             "my-client",
					ResolveCanonicalBootstrapServersOnly: true,
					Metadata: configkafka.MetadataConfig{
						Full:            false,
						RefreshInterval: 14 * time.Second,
						Retry: configkafka.MetadataRetryConfig{
							Max:     5,
							Backoff: 511 * time.Millisecond,
						},
					},
					Authentication: configkafka.AuthenticationConfig{
						PlainText: &configkafka.PlainTextConfig{
							Username: "user",
							Password: "pass",
						},
					},
				},
				Producer: configkafka.ProducerConfig{
					MaxMessageBytes: 2000001,
					RequiredAcks:    0,
					Compression:     "gzip",
					CompressionParams: configcompression.CompressionParams{
						Level: 9,
					},
					FlushMaxMessages:       101,
					AllowAutoTopicCreation: true,
				},
				Topic:                                "",
				IncludeMetadataKeys:                  []string(nil),
				TopicFromAttribute:                   "my-attr",
				Encoding:                             "",
				PartitionTracesByID:                  true,
				PartitionMetricsByResourceAttributes: true,
				PartitionLogsByResourceAttributes:    true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args kafka.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*kafkaexporter.Config)

			require.Equal(t, tc.expected, *actual)
		})
	}
}

func TestDebugMetricsConfig(t *testing.T) {
	tests := []struct {
		testName string
		agentCfg string
		expected otelcolCfg.DebugMetricsArguments
	}{
		{
			testName: "default",
			agentCfg: `
			protocol_version = "2.0.0"
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_false",
			agentCfg: `
			protocol_version = "2.0.0"
			debug_metrics {
				disable_high_cardinality_metrics = false
			}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_true",
			agentCfg: `
			protocol_version = "2.0.0"
			debug_metrics {
				disable_high_cardinality_metrics = true
			}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args kafka.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tc.agentCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}

func TestGetSignalType(t *testing.T) {
	encodings := []string{
		"otlp_proto",
		"otlp_json",
		"raw",
		"jaeger_proto",
		"jaeger_json",
		"zipkin_proto",
		"zipkin_json",
	}

	for _, encoding := range encodings {
		t.Run(encoding, func(t *testing.T) {
			args := kafka.Arguments{
				Encoding: encoding,
			}
			signalType := kafka.GetSignalType(component.Options{}, args)

			switch encoding {
			case "raw":
				require.Equal(t, exporter.TypeLogs, signalType)
			case "jaeger_proto", "jaeger_json", "zipkin_proto", "zipkin_json":
				require.Equal(t, exporter.TypeTraces, signalType)
			default:
				require.Equal(t, exporter.TypeAll, signalType)
			}
		})
	}

	signalCfgs := []struct {
		logs    *kafka.KafkaExporterSignalConfig
		traces  *kafka.KafkaExporterSignalConfig
		metrics *kafka.KafkaExporterSignalConfig
	}{
		{
			logs: &kafka.KafkaExporterSignalConfig{Encoding: "raw"},
		},
		{
			traces: &kafka.KafkaExporterSignalConfig{Encoding: "zipkin_json"},
		},
		{
			metrics: &kafka.KafkaExporterSignalConfig{Encoding: "otlp_json"},
		},
		{
			metrics: &kafka.KafkaExporterSignalConfig{Encoding: "otlp_json"},
			logs:    &kafka.KafkaExporterSignalConfig{Encoding: "raw"},
		},
		{
			metrics: &kafka.KafkaExporterSignalConfig{Encoding: "otlp_json"},
			logs:    &kafka.KafkaExporterSignalConfig{Encoding: "raw"},
		},
		{
			traces: &kafka.KafkaExporterSignalConfig{Encoding: "zipkin_json"},
			logs:   &kafka.KafkaExporterSignalConfig{Encoding: "raw"},
		},
		{
			metrics: &kafka.KafkaExporterSignalConfig{Encoding: "otlp_json"},
			traces:  &kafka.KafkaExporterSignalConfig{Encoding: "zipkin_json"},
		},
	}

	for _, encoding := range encodings {
		for _, signalCfg := range signalCfgs {
			t.Run(encoding, func(t *testing.T) {
				args := kafka.Arguments{
					Encoding: encoding,
					Logs:     signalCfg.logs,
					Traces:   signalCfg.traces,
					Metrics:  signalCfg.metrics,
				}
				signalType := kafka.GetSignalType(component.Options{}, args)

				var expected exporter.TypeSignal
				expected = 0

				if signalCfg.logs != nil {
					expected |= exporter.TypeLogs
				}
				if signalCfg.metrics != nil {
					expected |= exporter.TypeMetrics
				}
				if signalCfg.traces != nil {
					expected |= exporter.TypeTraces
				}

				switch encoding {
				case "raw":
					expected |= exporter.TypeLogs
				case "jaeger_proto", "jaeger_json", "zipkin_proto", "zipkin_json":
					expected |= exporter.TypeTraces
				default:
					expected |= exporter.TypeAll
				}

				require.Equal(t, expected, signalType)
			})
		}
	}
}
