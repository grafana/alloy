package awscloudwatch_test

import (
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/awscloudwatch"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"
	"github.com/stretchr/testify/require"
)

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
				Logs: &awscloudwatchreceiver.LogsConfig{
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
				Logs: &awscloudwatchreceiver.LogsConfig{
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
				Logs: &awscloudwatchreceiver.LogsConfig{
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
				Logs: &awscloudwatchreceiver.LogsConfig{
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
