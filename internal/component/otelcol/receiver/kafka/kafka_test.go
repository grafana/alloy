package kafka_test

import (
	"testing"
	"time"

	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/kafka"
	"github.com/grafana/alloy/syntax"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/kafka/configkafka"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/kafkareceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configretry"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	defaultExpected := func() kafkareceiver.Config {
		return kafkareceiver.Config{
			ClientConfig: configkafka.ClientConfig{
				Brokers:         []string{"10.10.10.10:9092"},
				ProtocolVersion: "2.0.0",
				ClientID:        "otel-collector",
				RackID:          "",
				UseLeaderEpoch:  true,
				Metadata: configkafka.MetadataConfig{
					Full:            true,
					RefreshInterval: 10 * time.Minute,
					Retry: configkafka.MetadataRetryConfig{
						Max:     3,
						Backoff: 250 * time.Millisecond,
					},
				},
			},
			ConsumerConfig: configkafka.ConsumerConfig{
				SessionTimeout:    10 * time.Second,
				HeartbeatInterval: 3 * time.Second,
				GroupID:           "otel-collector",
				InitialOffset:     "latest",
				AutoCommit: configkafka.AutoCommitConfig{
					Enable:   true,
					Interval: 1 * time.Second,
				},
				MinFetchSize:           1,
				DefaultFetchSize:       1048576,
				MaxFetchSize:           0,
				MaxPartitionFetchSize:  1048576,
				MaxFetchWait:           250 * time.Millisecond,
				GroupRebalanceStrategy: "range",
			},
			Logs: kafkareceiver.TopicEncodingConfig{
				Topics:   []string{"otlp_logs"},
				Encoding: "otlp_proto",
			},
			Metrics: kafkareceiver.TopicEncodingConfig{
				Topics:   []string{"otlp_metrics"},
				Encoding: "otlp_proto",
			},
			Traces: kafkareceiver.TopicEncodingConfig{
				Topics:   []string{"otlp_spans"},
				Encoding: "otlp_proto",
			},
			HeaderExtraction: kafkareceiver.HeaderExtraction{
				ExtractHeaders: false,
				Headers:        []string{},
			},
			ErrorBackOff: configretry.BackOffConfig{
				Enabled:             false,
				InitialInterval:     0,
				RandomizationFactor: 0,
				Multiplier:          0,
				MaxInterval:         0,
				MaxElapsedTime:      0,
			},
		}
	}

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
			expected: defaultExpected(),
		},
		{
			testName: "ExplicitValues_AuthPlaintext",
			cfg: `
				brokers = ["10.10.10.10:9092"]
				protocol_version = "2.0.0"
				session_timeout = "11s"
				heartbeat_interval = "4s"
				group_id = "test_group_id"
				client_id = "test_client_id"
				initial_offset = "test_offset"
				group_rebalance_strategy = "roundrobin"
				max_fetch_wait = "2s"
				logs {
					topics = ["^logs-.*"]
					encoding = "raw"
					exclude_topics = ["^logs-(test|dev)$"]
				}
				metrics {
					topics = ["^metrics-.*"]
					encoding = "otlp_json"
					exclude_topics = ["^metrics-internal-.*$"]
				}
				traces {
					topics = ["^traces-.*"]
					encoding = "zipkin_json"
					exclude_topics = ["^traces-debug-.*$"]
				}
				metadata {
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
				error_backoff {
					enabled = true
					initial_interval = "1s"
					randomization_factor = 0.1
					multiplier = 1.2
					max_interval = "1s"
					max_elapsed_time = "1m"
				}
				min_fetch_size = 2
				default_fetch_size = 10000
				max_fetch_size = 20
				max_partition_fetch_size = 30000
				rack_id = "test-rack"
				output {}
			`,
			expected: kafkareceiver.Config{
				Logs: kafkareceiver.TopicEncodingConfig{
					Topics:        []string{"^logs-.*"},
					Encoding:      "raw",
					ExcludeTopics: []string{"^logs-(test|dev)$"},
				},
				Metrics: kafkareceiver.TopicEncodingConfig{
					Topics:        []string{"^metrics-.*"},
					Encoding:      "otlp_json",
					ExcludeTopics: []string{"^metrics-internal-.*$"},
				},
				Traces: kafkareceiver.TopicEncodingConfig{
					Topics:        []string{"^traces-.*"},
					Encoding:      "zipkin_json",
					ExcludeTopics: []string{"^traces-debug-.*$"},
				},
				ClientConfig: configkafka.ClientConfig{
					Brokers:         []string{"10.10.10.10:9092"},
					ProtocolVersion: "2.0.0",
					ClientID:        "test_client_id",
					RackID:          "test-rack",
					UseLeaderEpoch:  true,
					Metadata: configkafka.MetadataConfig{
						Full:            true,
						RefreshInterval: 10 * time.Minute,
						Retry: configkafka.MetadataRetryConfig{
							Max:     9,
							Backoff: 11 * time.Second,
						},
					},
				},
				ConsumerConfig: configkafka.ConsumerConfig{
					SessionTimeout:    11 * time.Second,
					HeartbeatInterval: 4 * time.Second,
					GroupID:           "test_group_id",
					InitialOffset:     "test_offset",
					AutoCommit: configkafka.AutoCommitConfig{
						Enable:   true,
						Interval: 12 * time.Second,
					},
					MinFetchSize:           2,
					DefaultFetchSize:       10000,
					MaxFetchSize:           20,
					MaxPartitionFetchSize:  30000,
					MaxFetchWait:           2 * time.Second,
					GroupRebalanceStrategy: "roundrobin",
				},
				MessageMarking: kafkareceiver.MessageMarking{
					After:   true,
					OnError: true,
				},
				HeaderExtraction: kafkareceiver.HeaderExtraction{
					ExtractHeaders: true,
					Headers:        []string{"foo", "bar"},
				},
				ErrorBackOff: configretry.BackOffConfig{
					Enabled:             true,
					InitialInterval:     1 * time.Second,
					RandomizationFactor: 0.1,
					Multiplier:          1.2,
					MaxInterval:         1 * time.Second,
					MaxElapsedTime:      1 * time.Minute,
				},
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

			actual := actualPtr.(*kafkareceiver.Config)

			require.Equal(t, tc.expected, *actual)
		})
	}
}

func TestArguments_Auth(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected map[string]any
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
			expected: map[string]any{
				"brokers":                  []string{"10.10.10.10:9092"},
				"protocol_version":         "2.0.0",
				"session_timeout":          10 * time.Second,
				"heartbeat_interval":       3 * time.Second,
				"encoding":                 "",
				"group_id":                 "otel-collector",
				"client_id":                "otel-collector",
				"initial_offset":           "latest",
				"min_fetch_size":           1,
				"default_fetch_size":       1048576,
				"max_fetch_size":           0,
				"max_partition_fetch_size": 1048576,
				"max_fetch_wait":           250 * time.Millisecond,
				"group_rebalance_strategy": "range",
				"rack_id":                  "",
				"use_leader_epoch":         true,
				"metadata": configkafka.MetadataConfig{
					Full:            true,
					RefreshInterval: 10 * time.Minute,
					Retry: configkafka.MetadataRetryConfig{
						Max:     3,
						Backoff: 250 * time.Millisecond,
					},
				},
				"logs": kafkareceiver.TopicEncodingConfig{
					Topics:   []string{"otlp_logs"},
					Encoding: "otlp_proto",
				},
				"metrics": kafkareceiver.TopicEncodingConfig{
					Topics:   []string{"otlp_metrics"},
					Encoding: "otlp_proto",
				},
				"traces": kafkareceiver.TopicEncodingConfig{
					Topics:   []string{"otlp_spans"},
					Encoding: "otlp_proto",
				},
				"autocommit": configkafka.AutoCommitConfig{
					Enable:   true,
					Interval: 1 * time.Second,
				},
				"header_extraction": kafkareceiver.HeaderExtraction{
					ExtractHeaders: false,
					Headers:        []string{},
				},
				"error_backoff": configretry.BackOffConfig{
					Enabled:             false,
					InitialInterval:     0,
					RandomizationFactor: 0,
					Multiplier:          0,
					MaxInterval:         0,
					MaxElapsedTime:      0,
				},
				"auth": map[string]any{
					"plain_text": map[string]any{
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
						}
					}
				}

				output {}
			`,
			expected: map[string]any{
				"brokers":                  []string{"10.10.10.10:9092"},
				"protocol_version":         "2.0.0",
				"session_timeout":          10 * time.Second,
				"heartbeat_interval":       3 * time.Second,
				"encoding":                 "",
				"group_id":                 "otel-collector",
				"client_id":                "otel-collector",
				"initial_offset":           "latest",
				"min_fetch_size":           1,
				"default_fetch_size":       1048576,
				"max_fetch_size":           0,
				"max_partition_fetch_size": 1048576,
				"max_fetch_wait":           250 * time.Millisecond,
				"group_rebalance_strategy": "range",
				"rack_id":                  "",
				"use_leader_epoch":         true,
				"metadata": configkafka.MetadataConfig{
					Full:            true,
					RefreshInterval: 10 * time.Minute,
					Retry: configkafka.MetadataRetryConfig{
						Max:     3,
						Backoff: 250 * time.Millisecond,
					},
				},
				"logs": kafkareceiver.TopicEncodingConfig{
					Topics:   []string{"otlp_logs"},
					Encoding: "otlp_proto",
				},
				"metrics": kafkareceiver.TopicEncodingConfig{
					Topics:   []string{"otlp_metrics"},
					Encoding: "otlp_proto",
				},
				"traces": kafkareceiver.TopicEncodingConfig{
					Topics:   []string{"otlp_spans"},
					Encoding: "otlp_proto",
				},
				"autocommit": configkafka.AutoCommitConfig{
					Enable:   true,
					Interval: 1 * time.Second,
				},
				"header_extraction": kafkareceiver.HeaderExtraction{
					ExtractHeaders: false,
					Headers:        []string{},
				},
				"error_backoff": configretry.BackOffConfig{
					Enabled:             false,
					InitialInterval:     0,
					RandomizationFactor: 0,
					Multiplier:          0,
					MaxInterval:         0,
					MaxElapsedTime:      0,
				},
				"auth": map[string]any{
					"sasl": map[string]any{
						"username":  "test_username",
						"password":  "test_password",
						"mechanism": "test_mechanism",
						"version":   9,
						"aws_msk": map[string]any{
							"region": "test_region",
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
			expected: map[string]any{
				"brokers":                  []string{"10.10.10.10:9092"},
				"protocol_version":         "2.0.0",
				"session_timeout":          10 * time.Second,
				"heartbeat_interval":       3 * time.Second,
				"encoding":                 "",
				"group_id":                 "otel-collector",
				"client_id":                "otel-collector",
				"initial_offset":           "latest",
				"min_fetch_size":           1,
				"default_fetch_size":       1048576,
				"max_fetch_size":           0,
				"max_partition_fetch_size": 1048576,
				"max_fetch_wait":           250 * time.Millisecond,
				"group_rebalance_strategy": "range",
				"rack_id":                  "",
				"use_leader_epoch":         true,
				"metadata": configkafka.MetadataConfig{
					Full:            true,
					RefreshInterval: 10 * time.Minute,
					Retry: configkafka.MetadataRetryConfig{
						Max:     3,
						Backoff: 250 * time.Millisecond,
					},
				},
				"logs": kafkareceiver.TopicEncodingConfig{
					Topics:   []string{"otlp_logs"},
					Encoding: "otlp_proto",
				},
				"metrics": kafkareceiver.TopicEncodingConfig{
					Topics:   []string{"otlp_metrics"},
					Encoding: "otlp_proto",
				},
				"traces": kafkareceiver.TopicEncodingConfig{
					Topics:   []string{"otlp_spans"},
					Encoding: "otlp_proto",
				},
				"autocommit": configkafka.AutoCommitConfig{
					Enable:   true,
					Interval: 1 * time.Second,
				},
				"header_extraction": kafkareceiver.HeaderExtraction{
					ExtractHeaders: false,
					Headers:        []string{},
				},
				"error_backoff": configretry.BackOffConfig{
					Enabled:             false,
					InitialInterval:     0,
					RandomizationFactor: 0,
					Multiplier:          0,
					MaxInterval:         0,
					MaxElapsedTime:      0,
				},
				"auth": map[string]any{
					"tls": map[string]any{
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
						disable_fast_negotiation = true
					}
				}

				output {}
			`,
			expected: map[string]any{
				"brokers":                  []string{"10.10.10.10:9092"},
				"protocol_version":         "2.0.0",
				"session_timeout":          10 * time.Second,
				"heartbeat_interval":       3 * time.Second,
				"encoding":                 "",
				"group_id":                 "otel-collector",
				"client_id":                "otel-collector",
				"initial_offset":           "latest",
				"min_fetch_size":           1,
				"default_fetch_size":       1048576,
				"max_fetch_size":           0,
				"max_partition_fetch_size": 1048576,
				"max_fetch_wait":           250 * time.Millisecond,
				"group_rebalance_strategy": "range",
				"rack_id":                  "",
				"use_leader_epoch":         true,
				"metadata": configkafka.MetadataConfig{
					Full:            true,
					RefreshInterval: 10 * time.Minute,
					Retry: configkafka.MetadataRetryConfig{
						Max:     3,
						Backoff: 250 * time.Millisecond,
					},
				},
				"logs": kafkareceiver.TopicEncodingConfig{
					Topics:   []string{"otlp_logs"},
					Encoding: "otlp_proto",
				},
				"metrics": kafkareceiver.TopicEncodingConfig{
					Topics:   []string{"otlp_metrics"},
					Encoding: "otlp_proto",
				},
				"traces": kafkareceiver.TopicEncodingConfig{
					Topics:   []string{"otlp_spans"},
					Encoding: "otlp_proto",
				},
				"autocommit": configkafka.AutoCommitConfig{
					Enable:   true,
					Interval: 1 * time.Second,
				},
				"header_extraction": kafkareceiver.HeaderExtraction{
					ExtractHeaders: false,
					Headers:        []string{},
				},
				"error_backoff": configretry.BackOffConfig{
					Enabled:             false,
					InitialInterval:     0,
					RandomizationFactor: 0,
					Multiplier:          0,
					MaxInterval:         0,
					MaxElapsedTime:      0,
				},
				"auth": map[string]any{
					"kerberos": map[string]any{
						"service_name":             "test_service_name",
						"realm":                    "test_realm",
						"use_keytab":               true,
						"username":                 "test_username",
						"password":                 "test_password",
						"config_file":              "test_config_filem",
						"keytab_file":              "test_keytab_file",
						"disable_fast_negotiation": true,
					},
				},
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
		alloyCfg string
		expected otelcolCfg.DebugMetricsArguments
	}{
		{
			testName: "default",
			alloyCfg: `
			brokers = ["10.10.10.10:9092"]
			protocol_version = "2.0.0"
			output {}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: true,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_false",
			alloyCfg: `
			brokers = ["10.10.10.10:9092"]
			protocol_version = "2.0.0"
			debug_metrics {
				disable_high_cardinality_metrics = false
			}
			output {}
			`,
			expected: otelcolCfg.DebugMetricsArguments{
				DisableHighCardinalityMetrics: false,
				Level:                         otelcolCfg.LevelDetailed,
			},
		},
		{
			testName: "explicit_true",
			alloyCfg: `
			brokers = ["10.10.10.10:9092"]
			protocol_version = "2.0.0"
			debug_metrics {
				disable_high_cardinality_metrics = true
			}
			output {}
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
			require.NoError(t, syntax.Unmarshal([]byte(tc.alloyCfg), &args))
			_, err := args.Convert()
			require.NoError(t, err)

			require.Equal(t, tc.expected, args.DebugMetricsConfig())
		})
	}
}
