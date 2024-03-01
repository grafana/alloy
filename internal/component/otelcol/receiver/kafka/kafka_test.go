package kafka_test

import (
	"testing"
	"time"

	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/receiver/kafka"
	"github.com/grafana/river"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/kafkaexporter"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalRiver(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected kafkareceiver.Config
	}{
		{
			testName: "Defaults",
			cfg: `
				brokers = ["10.10.10.10:9092"]
				protocol_version = "2.0.0"
				output {}
			`,
			expected: kafkareceiver.Config{
				Brokers:         []string{"10.10.10.10:9092"},
				ProtocolVersion: "2.0.0",
				Topic:           "otlp_spans",
				Encoding:        "otlp_proto",
				GroupID:         "otel-collector",
				ClientID:        "otel-collector",
				InitialOffset:   "latest",
				Metadata: kafkaexporter.Metadata{
					Full: true,
					Retry: kafkaexporter.MetadataRetry{
						Max:     3,
						Backoff: 250 * time.Millisecond,
					},
				},
				AutoCommit: kafkareceiver.AutoCommit{
					Enable:   true,
					Interval: 1 * time.Second,
				},
				HeaderExtraction: kafkareceiver.HeaderExtraction{
					ExtractHeaders: false,
					Headers:        []string{},
				},
			},
		},
		{
			testName: "ExplicitValues_AuthPlaintext",
			cfg: `
				brokers = ["10.10.10.10:9092"]
				protocol_version = "2.0.0"
				topic = "test_topic"
				encoding = "test_encoding"
				group_id = "test_group_id"
				client_id = "test_client_id"
				initial_offset = "test_offset"
				metadata {
					include_all_topics = true
					retry {
						max_retries = 9
						backoff = "11s"
					}
				}
				autocommit {
					enable = true
					interval = "12s"
				}
				message_marking {
					after_execution = true
					include_unsuccessful = true
				}
				header_extraction {
					extract_headers = true
					headers = ["foo", "bar"]
				}
				output {}
			`,
			expected: kafkareceiver.Config{
				Brokers:         []string{"10.10.10.10:9092"},
				ProtocolVersion: "2.0.0",
				Topic:           "test_topic",
				Encoding:        "test_encoding",
				GroupID:         "test_group_id",
				ClientID:        "test_client_id",
				InitialOffset:   "test_offset",
				Metadata: kafkaexporter.Metadata{
					Full: true,
					Retry: kafkaexporter.MetadataRetry{
						Max:     9,
						Backoff: 11 * time.Second,
					},
				},
				AutoCommit: kafkareceiver.AutoCommit{
					Enable:   true,
					Interval: 12 * time.Second,
				},
				MessageMarking: kafkareceiver.MessageMarking{
					After:   true,
					OnError: true,
				},
				HeaderExtraction: kafkareceiver.HeaderExtraction{
					ExtractHeaders: true,
					Headers:        []string{"foo", "bar"},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args kafka.Arguments
			err := river.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*kafkareceiver.Config)

			require.Equal(t, tc.expected, *actual)
		})
	}
}

func TestArguments_Auth(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected map[string]interface{}
	}{
		{
			testName: "plain_text",
			cfg: `
				brokers = ["10.10.10.10:9092"]
				protocol_version = "2.0.0"

				authentication {
					plaintext {
						username = "test_username"
						password = "test_password"
					}
				}

				output {}
			`,
			expected: map[string]interface{}{
				"brokers":          []string{"10.10.10.10:9092"},
				"protocol_version": "2.0.0",
				"topic":            "otlp_spans",
				"encoding":         "otlp_proto",
				"group_id":         "otel-collector",
				"client_id":        "otel-collector",
				"initial_offset":   "latest",
				"metadata": kafkaexporter.Metadata{
					Full: true,
					Retry: kafkaexporter.MetadataRetry{
						Max:     3,
						Backoff: 250 * time.Millisecond,
					},
				},
				"autocommit": kafkareceiver.AutoCommit{
					Enable:   true,
					Interval: 1 * time.Second,
				},
				"header_extraction": kafkareceiver.HeaderExtraction{
					ExtractHeaders: false,
					Headers:        []string{},
				},
				"auth": map[string]interface{}{
					"plain_text": map[string]interface{}{
						"username": "test_username",
						"password": "test_password",
					},
				},
			},
		},
		{
			testName: "sasl",
			cfg: `
				brokers = ["10.10.10.10:9092"]
				protocol_version = "2.0.0"

				authentication {
					sasl {
						username = "test_username"
						password = "test_password"
						mechanism = "test_mechanism"
						version = 9
						aws_msk {
							region = "test_region"
							broker_addr = "test_broker_addr"
						}
					}
				}

				output {}
			`,
			expected: map[string]interface{}{
				"brokers":          []string{"10.10.10.10:9092"},
				"protocol_version": "2.0.0",
				"topic":            "otlp_spans",
				"encoding":         "otlp_proto",
				"group_id":         "otel-collector",
				"client_id":        "otel-collector",
				"initial_offset":   "latest",
				"metadata": kafkaexporter.Metadata{
					Full: true,
					Retry: kafkaexporter.MetadataRetry{
						Max:     3,
						Backoff: 250 * time.Millisecond,
					},
				},
				"autocommit": kafkareceiver.AutoCommit{
					Enable:   true,
					Interval: 1 * time.Second,
				},
				"header_extraction": kafkareceiver.HeaderExtraction{
					ExtractHeaders: false,
					Headers:        []string{},
				},
				"auth": map[string]interface{}{
					"sasl": map[string]interface{}{
						"username":  "test_username",
						"password":  "test_password",
						"mechanism": "test_mechanism",
						"version":   9,
						"aws_msk": map[string]interface{}{
							"region":      "test_region",
							"broker_addr": "test_broker_addr",
						},
					},
				},
			},
		},
		{
			testName: "tls",
			cfg: `
				brokers = ["10.10.10.10:9092"]
				protocol_version = "2.0.0"

				authentication {
					tls {
						insecure = true
						insecure_skip_verify = true
						server_name = "test_server_name_override"
						ca_pem = "test_ca_pem"
						cert_pem = "test_cert_pem"
						key_pem = "test_key_pem"
						min_version = "1.1"
						reload_interval = "11s"
					}
				}

				output {}
			`,
			expected: map[string]interface{}{
				"brokers":          []string{"10.10.10.10:9092"},
				"protocol_version": "2.0.0",
				"topic":            "otlp_spans",
				"encoding":         "otlp_proto",
				"group_id":         "otel-collector",
				"client_id":        "otel-collector",
				"initial_offset":   "latest",
				"metadata": kafkaexporter.Metadata{
					Full: true,
					Retry: kafkaexporter.MetadataRetry{
						Max:     3,
						Backoff: 250 * time.Millisecond,
					},
				},
				"autocommit": kafkareceiver.AutoCommit{
					Enable:   true,
					Interval: 1 * time.Second,
				},
				"header_extraction": kafkareceiver.HeaderExtraction{
					ExtractHeaders: false,
					Headers:        []string{},
				},
				"auth": map[string]interface{}{
					"tls": map[string]interface{}{
						"insecure":             true,
						"insecure_skip_verify": true,
						"server_name_override": "test_server_name_override",
						"ca_pem":               "test_ca_pem",
						"cert_pem":             "test_cert_pem",
						"key_pem":              "test_key_pem",
						"min_version":          "1.1",
						"reload_interval":      11 * time.Second,
					},
				},
			},
		},
		{
			testName: "kerberos",
			cfg: `
				brokers = ["10.10.10.10:9092"]
				protocol_version = "2.0.0"

				authentication {
					kerberos {
						service_name = "test_service_name"
						realm = "test_realm"
						use_keytab = true
						username = "test_username"
						password = "test_password"
						config_file = "test_config_filem"
						keytab_file = "test_keytab_file"
					}
				}

				output {}
			`,
			expected: map[string]interface{}{
				"brokers":          []string{"10.10.10.10:9092"},
				"protocol_version": "2.0.0",
				"topic":            "otlp_spans",
				"encoding":         "otlp_proto",
				"group_id":         "otel-collector",
				"client_id":        "otel-collector",
				"initial_offset":   "latest",
				"metadata": kafkaexporter.Metadata{
					Full: true,
					Retry: kafkaexporter.MetadataRetry{
						Max:     3,
						Backoff: 250 * time.Millisecond,
					},
				},
				"autocommit": kafkareceiver.AutoCommit{
					Enable:   true,
					Interval: 1 * time.Second,
				},
				"header_extraction": kafkareceiver.HeaderExtraction{
					ExtractHeaders: false,
					Headers:        []string{},
				},
				"auth": map[string]interface{}{
					"kerberos": map[string]interface{}{
						"service_name": "test_service_name",
						"realm":        "test_realm",
						"use_keytab":   true,
						"username":     "test_username",
						"password":     "test_password",
						"config_file":  "test_config_filem",
						"keytab_file":  "test_keytab_file",
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args kafka.Arguments
			err := river.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*kafkareceiver.Config)

			var expected kafkareceiver.Config
			err = mapstructure.Decode(tc.expected, &expected)
			require.NoError(t, err)

			require.Equal(t, expected, *actual)
		})
	}
}

func TestDebugMetricsConfig(t *testing.T) {
	tests := []struct {
		testName string
		agentCfg string
		expected otelcol.DebugMetricsArguments
	}{
		{
			testName: "default",
			agentCfg: `
			brokers = ["10.10.10.10:9092"]
			protocol_version = "2.0.0"
			output {}
			`,
			expected: otelcol.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
			},
		},
		{
			testName: "explicit_false",
			agentCfg: `
			brokers = ["10.10.10.10:9092"]
			protocol_version = "2.0.0"
			debug_metrics {
				disable_high_cardinality_metrics = false
			}
			output {}
			`,
			expected: otelcol.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
			},
		},
		{
			testName: "explicit_true",
			agentCfg: `
			brokers = ["10.10.10.10:9092"]
			protocol_version = "2.0.0"
			debug_metrics {
				disable_high_cardinality_metrics = true
			}
			output {}
			`,
			expected: otelcol.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args kafka.Arguments
			require.NoError(t, river.Unmarshal([]byte(tc.agentCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}
