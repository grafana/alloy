package kafka_test

import (
	"testing"
	"time"

	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter/kafka"
	"github.com/grafana/alloy/syntax"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected map[string]interface{}
	}{
		{
			testName: "Defaults",
			cfg: `
				protocol_version = "2.0.0"
			`,
			expected: map[string]interface{}{
				"brokers":          []string{"localhost:9092"},
				"protocol_version": "2.0.0",
				"resolve_canonical_bootstrap_servers_only": false,
				"client_id":              "sarama",
				"topic":                  "",
				"topic_from_attribute":   "",
				"encoding":               "otlp_proto",
				"partition_traces_by_id": false,
				"partition_metrics_by_resource_attributes": false,
				"timeout":        5 * time.Second,
				"authentication": map[string]interface{}{},

				"metadata": map[string]interface{}{
					"full": true,
					"retry": map[string]interface{}{
						"max":     3,
						"backoff": 250 * time.Millisecond,
					},
				},
				"retry_on_failure": map[string]interface{}{
					"enabled":              true,
					"initial_interval":     5 * time.Second,
					"randomization_factor": 0.5,
					"multiplier":           1.5,
					"max_interval":         30 * time.Second,
					"max_elapsed_time":     5 * time.Minute,
				},
				"sending_queue": map[string]interface{}{
					"enabled":       true,
					"num_consumers": 10,
					"queue_size":    1000,
				},
				"producer": map[string]interface{}{
					"max_message_bytes":  1000000,
					"required_acks":      1,
					"compression":        "none",
					"flush_max_messages": 0,
				},
			},
		},
		{
			testName: "Explicit",
			cfg: `
				protocol_version = "2.0.0"
				brokers = ["redpanda:123"]
				resolve_canonical_bootstrap_servers_only = true
				client_id = "my-client"
				topic = "my-topic"
				topic_from_attribute = "my-attr"
				encoding = "otlp_json"
				partition_traces_by_id = true
				partition_metrics_by_resource_attributes = true
				timeout = "12s"

				authentication {
					plaintext {
						username = "user"
						password = "pass"
					}
				}

				metadata {
					include_all_topics = false
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
					flush_max_messages = 101
				}
			`,
			expected: map[string]interface{}{
				"brokers":          []string{"redpanda:123"},
				"protocol_version": "2.0.0",
				"resolve_canonical_bootstrap_servers_only": true,
				"client_id":              "my-client",
				"topic":                  "my-topic",
				"topic_from_attribute":   "my-attr",
				"encoding":               "otlp_json",
				"partition_traces_by_id": true,
				"partition_metrics_by_resource_attributes": true,
				"timeout": 12 * time.Second,
				"auth": map[string]interface{}{
					"plain_text": map[string]interface{}{
						"username": "user",
						"password": "pass",
					},
				},

				"metadata": map[string]interface{}{
					"full": false,
					"retry": map[string]interface{}{
						"max":     5,
						"backoff": 511 * time.Millisecond,
					},
				},
				"retry_on_failure": map[string]interface{}{
					"enabled":              true,
					"initial_interval":     10 * time.Second,
					"randomization_factor": 0.1,
					"multiplier":           2.0,
					"max_interval":         61 * time.Second,
					"max_elapsed_time":     11 * time.Minute,
				},
				"sending_queue": map[string]interface{}{
					"enabled":       true,
					"num_consumers": 11,
					"queue_size":    1001,
				},
				"producer": map[string]interface{}{
					"max_message_bytes":  2000001,
					"required_acks":      0,
					"compression":        "gzip",
					"flush_max_messages": 101,
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var expected kafkaexporter.Config
			err := mapstructure.Decode(tc.expected, &expected)
			require.NoError(t, err)

			var args kafka.Arguments
			err = syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*kafkaexporter.Config)

			require.Equal(t, expected, *actual)
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
