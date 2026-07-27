package awscloudwatch_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/awscloudwatch"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
	"go.opentelemetry.io/collector/scraper/scraperhelper"
)

// defaultLogsConfig is the logs config produced when no logs block is given.
// Metrics test cases reuse it so they assert only on what they exercise.
func defaultLogsConfig() awscloudwatchreceiver.LogsConfig {
	return awscloudwatchreceiver.LogsConfig{
		PollInterval:        time.Minute,
		MaxEventsPerRequest: 1000,
		Groups: awscloudwatchreceiver.GroupConfig{
			AutodiscoverConfig: &awscloudwatchreceiver.AutodiscoverConfig{
				Limit:   50,
				Streams: awscloudwatchreceiver.StreamConfig{},
			},
			NamedConfigs: map[string]awscloudwatchreceiver.StreamConfig{},
		},
	}
}

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected awscloudwatchreceiver.Config
	}{
		{
			testName: "default configuration",
			cfg: `
				region = "us-west-2"
				output {}
			`,
			expected: awscloudwatchreceiver.Config{
				Region: "us-west-2",
				Logs: awscloudwatchreceiver.LogsConfig{
					StartFrom:           "",
					PollInterval:        time.Minute,
					MaxEventsPerRequest: 1000,
					Groups: awscloudwatchreceiver.GroupConfig{
						AutodiscoverConfig: &awscloudwatchreceiver.AutodiscoverConfig{
							Limit:   50,
							Streams: awscloudwatchreceiver.StreamConfig{},
						},
						NamedConfigs: map[string]awscloudwatchreceiver.StreamConfig{},
					},
				},
			},
		},
		{
			testName: "full configuration with autodiscover",
			cfg: `
				region = "us-east-1"
				profile = "prod"
				imds_endpoint = "http://169.254.169.254"
				logs {
					poll_interval = "1m"
					max_events_per_request = 1000
					groups {
						autodiscover {
							prefix = "app-"
							limit = 10
							streams {
								prefixes = ["api-", "web-"]
								names = ["main", "error"]
							}
						}
					}
				}
				output {}
			`,
			expected: awscloudwatchreceiver.Config{
				Region:       "us-east-1",
				Profile:      "prod",
				IMDSEndpoint: "http://169.254.169.254",
				Logs: awscloudwatchreceiver.LogsConfig{
					PollInterval:        time.Minute,
					MaxEventsPerRequest: 1000,
					Groups: awscloudwatchreceiver.GroupConfig{
						AutodiscoverConfig: &awscloudwatchreceiver.AutodiscoverConfig{
							Prefix: "app-",
							Limit:  10,
							Streams: awscloudwatchreceiver.StreamConfig{
								Prefixes: []*string{ptr("api-"), ptr("web-")},
								Names:    []*string{ptr("main"), ptr("error")},
							},
						},
						NamedConfigs: map[string]awscloudwatchreceiver.StreamConfig{},
					},
				},
			},
		},
		{
			testName: "autodiscover with default limit",
			cfg: `
				region = "us-west-2"
				logs {
					poll_interval = "1m"
					max_events_per_request = 1000
					groups {
						autodiscover {
							prefix = "app-"
							streams {
								prefixes = ["api-"]
							}
						}
					}
				}
				output {}
			`,
			expected: awscloudwatchreceiver.Config{
				Region: "us-west-2",
				Logs: awscloudwatchreceiver.LogsConfig{
					PollInterval:        time.Minute,
					MaxEventsPerRequest: 1000,
					Groups: awscloudwatchreceiver.GroupConfig{
						AutodiscoverConfig: &awscloudwatchreceiver.AutodiscoverConfig{
							Prefix: "app-",
							Limit:  50, // Default value
							Streams: awscloudwatchreceiver.StreamConfig{
								Prefixes: []*string{ptr("api-")},
							},
						},
						NamedConfigs: map[string]awscloudwatchreceiver.StreamConfig{},
					},
				},
			},
		},
		{
			testName: "configuration with named groups",
			cfg: `
				region = "us-west-2"
				logs {
					poll_interval = "1m"
					max_events_per_request = 1000
					groups {
						named {
							group_name = "/aws/eks/dev-0/cluster"
							names = ["api-gateway"]
						}
						named {
							group_name = "/aws/eks/dev-2/cluster"
							prefixes = ["app-", "service-"]
							names = ["main", "error"]
						}
					}
				}
				output {}
			`,
			expected: awscloudwatchreceiver.Config{
				Region: "us-west-2",
				Logs: awscloudwatchreceiver.LogsConfig{
					PollInterval:        time.Minute,
					MaxEventsPerRequest: 1000,
					Groups: awscloudwatchreceiver.GroupConfig{
						NamedConfigs: map[string]awscloudwatchreceiver.StreamConfig{
							"/aws/eks/dev-0/cluster": {
								Names: []*string{ptr("api-gateway")},
							},
							"/aws/eks/dev-2/cluster": {
								Prefixes: []*string{ptr("app-"), ptr("service-")},
								Names:    []*string{ptr("main"), ptr("error")},
							},
						},
					},
				},
			},
		},
		{
			testName: "start_from configuration set",
			cfg: `
				region = "us-west-2"
				logs {
					poll_interval = "1m"
					start_from = "2025-06-25T00:00:00Z"
				}
				output {}
			`,
			expected: awscloudwatchreceiver.Config{
				Region: "us-west-2",
				Logs: awscloudwatchreceiver.LogsConfig{
					StartFrom:           "2025-06-25T00:00:00Z",
					PollInterval:        time.Minute,
					MaxEventsPerRequest: 1000,
					Groups: awscloudwatchreceiver.GroupConfig{
						AutodiscoverConfig: &awscloudwatchreceiver.AutodiscoverConfig{
							Limit:   50,
							Streams: awscloudwatchreceiver.StreamConfig{},
						},
						NamedConfigs: map[string]awscloudwatchreceiver.StreamConfig{},
					},
				},
			},
		},
		{
			testName: "autodiscover with pattern and linked accounts",
			cfg: `
				region = "us-west-2"
				logs {
					groups {
						autodiscover {
							pattern = "/aws/lambda/.*"
							account_identifiers = ["123456789012", "987654321098"]
							include_linked_accounts = true
						}
					}
				}
				output {}
			`,
			expected: awscloudwatchreceiver.Config{
				Region: "us-west-2",
				Logs: awscloudwatchreceiver.LogsConfig{
					PollInterval:        time.Minute,
					MaxEventsPerRequest: 1000,
					Groups: awscloudwatchreceiver.GroupConfig{
						AutodiscoverConfig: &awscloudwatchreceiver.AutodiscoverConfig{
							Pattern:               "/aws/lambda/.*",
							Limit:                 50,
							Streams:               awscloudwatchreceiver.StreamConfig{},
							AccountIdentifiers:    []string{"123456789012", "987654321098"},
							IncludeLinkedAccounts: boolPtr(true),
						},
						NamedConfigs: map[string]awscloudwatchreceiver.StreamConfig{},
					},
				},
			},
		},
		{
			testName: "metrics with explicit queries",
			cfg: `
				region = "us-west-2"
				metrics {
					collection_interval = "1m"
					period = "1m"
					query {
						namespace = "AWS/EC2"
						metric_name = "CPUUtilization"
						dimensions = {
							InstanceId = "i-1234567890abcdef0",
						}
						stats = ["Average", "p99"]
					}
					query {
						namespace = "AWS/DynamoDB"
						metric_name = "SuccessfulRequestLatency"
					}
				}
				output {}
			`,
			expected: awscloudwatchreceiver.Config{
				Region: "us-west-2",
				Logs:   defaultLogsConfig(),
				Metrics: awscloudwatchreceiver.MetricsConfig{
					ControllerConfig: scraperhelper.ControllerConfig{
						CollectionInterval: time.Minute,
						InitialDelay:       time.Second,
					},
					Period: time.Minute,
					Delay:  10 * time.Minute,
					Queries: []awscloudwatchreceiver.MetricQuery{
						{
							Namespace:  "AWS/EC2",
							MetricName: "CPUUtilization",
							Dimensions: map[string]string{"InstanceId": "i-1234567890abcdef0"},
							Stats:      []string{"Average", "p99"},
						},
						{
							Namespace:  "AWS/DynamoDB",
							MetricName: "SuccessfulRequestLatency",
						},
					},
				},
			},
		},
		{
			testName: "metrics with defaults",
			cfg: `
				region = "us-west-2"
				metrics {
					query {
						namespace = "AWS/EC2"
						metric_name = "CPUUtilization"
					}
				}
				output {}
			`,
			expected: awscloudwatchreceiver.Config{
				Region: "us-west-2",
				Logs:   defaultLogsConfig(),
				Metrics: awscloudwatchreceiver.MetricsConfig{
					ControllerConfig: scraperhelper.ControllerConfig{
						// 5m, not the generic otelcol default of 1m.
						CollectionInterval: 5 * time.Minute,
						InitialDelay:       time.Second,
					},
					Period: 5 * time.Minute,
					Delay:  10 * time.Minute,
					Queries: []awscloudwatchreceiver.MetricQuery{
						{Namespace: "AWS/EC2", MetricName: "CPUUtilization"},
					},
				},
			},
		},
		{
			testName: "metrics discovery with filters",
			cfg: `
				region = "us-west-2"
				metrics {
					collection_interval = "5m"
					period = "5m"
					delay = "15m"
					discovery {
						limit = 200
						stats = ["Average"]
						filters {
							namespace = "AWS/EC2"
							metric_name = "CPUUtilization"
						}
					}
				}
				output {}
			`,
			expected: awscloudwatchreceiver.Config{
				Region: "us-west-2",
				Logs:   defaultLogsConfig(),
				Metrics: awscloudwatchreceiver.MetricsConfig{
					ControllerConfig: scraperhelper.ControllerConfig{
						CollectionInterval: 5 * time.Minute,
						InitialDelay:       time.Second,
					},
					Period: 5 * time.Minute,
					Delay:  15 * time.Minute,
					Discovery: &awscloudwatchreceiver.MetricsDiscoveryConfig{
						Filters: configoptional.Some(awscloudwatchreceiver.MetricsDiscoveryFilters{
							Namespace:  "AWS/EC2",
							MetricName: "CPUUtilization",
						}),
						Limit: 200,
						Stats: []string{"Average"},
					},
				},
			},
		},
		{
			testName: "metrics discovery without filters uses default limit",
			cfg: `
				region = "us-west-2"
				metrics {
					discovery {}
				}
				output {}
			`,
			expected: awscloudwatchreceiver.Config{
				Region: "us-west-2",
				Logs:   defaultLogsConfig(),
				Metrics: awscloudwatchreceiver.MetricsConfig{
					ControllerConfig: scraperhelper.ControllerConfig{
						CollectionInterval: 5 * time.Minute,
						InitialDelay:       time.Second,
					},
					Period: 5 * time.Minute,
					Delay:  10 * time.Minute,
					Discovery: &awscloudwatchreceiver.MetricsDiscoveryConfig{
						Filters: configoptional.None[awscloudwatchreceiver.MetricsDiscoveryFilters](),
						Limit:   100,
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args awscloudwatch.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*awscloudwatchreceiver.Config)

			require.Equal(t, tc.expected, *actual)
		})
	}
}

// TestArguments_NoMetricsBlock guards backwards compatibility: a configuration
// without a metrics block must leave the upstream Metrics config at its zero
// value, so existing logs-only users gain no CloudWatch GetMetricData calls.
func TestArguments_NoMetricsBlock(t *testing.T) {
	cfg := `
		region = "us-west-2"
		logs {
			poll_interval = "1m"
		}
		output {}
	`

	var args awscloudwatch.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
	require.Nil(t, args.Metrics)

	actual, err := args.Convert()
	require.NoError(t, err)

	require.Equal(t,
		awscloudwatchreceiver.MetricsConfig{},
		actual.(*awscloudwatchreceiver.Config).Metrics,
	)
}

// TestArguments_IncludeLinkedAccountsUnset guards the pointer semantics of
// include_linked_accounts. Upstream only forwards the field to AWS when it is
// non-nil, so omitting the attribute must not send an explicit false.
func TestArguments_IncludeLinkedAccountsUnset(t *testing.T) {
	cfg := `
		region = "us-west-2"
		logs {
			groups {
				autodiscover {
					prefix = "app-"
				}
			}
		}
		output {}
	`

	var args awscloudwatch.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	actual, err := args.Convert()
	require.NoError(t, err)

	autodiscover := actual.(*awscloudwatchreceiver.Config).Logs.Groups.AutodiscoverConfig
	require.NotNil(t, autodiscover)
	require.Nil(t, autodiscover.IncludeLinkedAccounts)
	require.Nil(t, autodiscover.AccountIdentifiers)
}

func TestArguments_Validate(t *testing.T) {
	tests := []struct {
		testName      string
		cfg           string
		expectedError string
	}{
		{
			testName: "invalid poll interval",
			cfg: `
				region = "us-west-2"
				logs {
					poll_interval = "500ms"
				}
				output {}
			`,
			expectedError: "poll interval is incorrect, it must be a duration greater than one second",
		},
		{
			testName: "invalid max_events_per_request",
			cfg: `
				region = "us-west-2"
				logs {
					max_events_per_request = 0
				}
				output {}
			`,
			expectedError: "event limit is improperly configured, value must be greater than 0",
		},
		{
			testName: "invalid imds_endpoint",
			cfg: `
				region = "us-west-2"
				imds_endpoint = "not-a-url"
				logs {
					poll_interval = "1m"
				}
				output {}
			`,
			expectedError: "unable to parse URI for imds_endpoint",
		},
		{
			testName: "both autodiscover and named configs",
			cfg: `
				region = "us-west-2"
				logs {
					groups {
						autodiscover {
							prefix = "app-"
						}
						named {
							group_name = "group1"
							prefixes = ["api-"]
						}
					}
				}
				output {}
			`,
			expectedError: "both autodiscover and named configs are configured, Only one or the other is permitted",
		},
		{
			testName: "invalid start_from configuration set",
			cfg: `
				region = "us-west-2"
				logs {
					poll_interval = "1m"
					start_from = "earliest"
				}
				output {}
			`,
			expectedError: "invalid start_from time format",
		},
		{
			testName: "both prefix and pattern",
			cfg: `
				region = "us-west-2"
				logs {
					groups {
						autodiscover {
							prefix = "app-"
							pattern = "app-.*"
						}
					}
				}
				output {}
			`,
			expectedError: "cannot specify both prefix and pattern",
		},
		{
			testName: "both queries and discovery",
			cfg: `
				region = "us-west-2"
				metrics {
					query {
						namespace = "AWS/EC2"
						metric_name = "CPUUtilization"
					}
					discovery {
						limit = 100
					}
				}
				output {}
			`,
			expectedError: "metrics and discovery are mutually exclusive",
		},
		{
			testName: "collection_interval less than period",
			cfg: `
				region = "us-west-2"
				metrics {
					collection_interval = "1m"
					period = "5m"
					query {
						namespace = "AWS/EC2"
						metric_name = "CPUUtilization"
					}
				}
				output {}
			`,
			expectedError: "metrics collection_interval must be greater than or equal to period",
		},
		{
			testName: "empty stat name",
			cfg: `
				region = "us-west-2"
				metrics {
					query {
						namespace = "AWS/EC2"
						metric_name = "CPUUtilization"
						stats = [""]
					}
				}
				output {}
			`,
			expectedError: "stat name must not be empty",
		},
		{
			testName: "discovery limit not positive",
			cfg: `
				region = "us-west-2"
				metrics {
					discovery {
						limit = 0
					}
				}
				output {}
			`,
			expectedError: "metrics discovery limit must be greater than 0",
		},
		{
			testName: "query missing namespace",
			cfg: `
				region = "us-west-2"
				metrics {
					query {
						metric_name = "CPUUtilization"
					}
				}
				output {}
			`,
			expectedError: `missing required attribute "namespace"`,
		},
		{
			testName: "query missing metric_name",
			cfg: `
				region = "us-west-2"
				metrics {
					query {
						namespace = "AWS/EC2"
					}
				}
				output {}
			`,
			expectedError: `missing required attribute "metric_name"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args awscloudwatch.Arguments
			require.ErrorContains(t, syntax.Unmarshal([]byte(tc.cfg), &args), tc.expectedError)
		})
	}
}

// Helper function to create string pointers
func ptr(s string) *string {
	return &s
}

// Helper function to create bool pointers
func boolPtr(b bool) *bool {
	return &b
}
