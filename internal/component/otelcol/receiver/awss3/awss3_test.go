package awss3_test

import (
	"testing"

	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awss3receiver"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/otelcol/receiver/awss3"
	"github.com/grafana/alloy/syntax"
)

func TestArguments_Defaults(t *testing.T) {
	// Test to detect if upstream defaults are changed or any other breaking changes introduced.
	expect := awss3.Arguments{
		S3Downloader: awss3.S3DownloaderConfig{
			Region:              "us-east-1",
			EndpointPartitionID: "aws",
			S3PartitionFormat:   "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
		},
	}

	var args awss3.Arguments
	args.SetToDefault()

	require.Equal(t, expect, args)
}

func TestArguments_UnmarshalAlloy(t *testing.T) {
	cases := []struct {
		name     string
		cfg      string
		expected awss3receiver.Config
	}{
		{
			name: "time bounded download",
			cfg: `
				start_time = "2024-01-01T00:00:00Z"
				end_time = "2024-01-02T00:00:00Z"
				s3downloader {
					region = "us-west-2"
					s3_bucket = "grafana-logs"
					s3_prefix = "logs/"
					s3_partition_format = "year=%Y/month=%m/day=%d/hour=%H/minute=%M"
					s3_partition_timezone = "UTC"
					file_prefix = "app_"
					endpoint = "https://s3.us-west-2.amazonaws.com"
					endpoint_partition_id = "aws"
					s3_force_path_style = true
				}
				output {}
			`,
			expected: awss3receiver.Config{
				StartTime: "2024-01-01T00:00:00Z",
				EndTime:   "2024-01-02T00:00:00Z",
				S3Downloader: awss3receiver.S3DownloaderConfig{
					Region:              "us-west-2",
					S3Bucket:            "grafana-logs",
					S3Prefix:            "logs/",
					S3PartitionFormat:   "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
					S3PartitionTimezone: "UTC",
					FilePrefix:          "app_",
					Endpoint:            "https://s3.us-west-2.amazonaws.com",
					EndpointPartitionID: "aws",
					S3ForcePathStyle:    true,
				},
			},
		},
		{
			name: "sqs notifications",
			cfg: `
				s3downloader {
					region = "us-west-2"
					s3_bucket = "grafana-bucket"
					s3_prefix = "ingest/"
					s3_partition_format = "year=%Y/month=%m/day=%d/hour=%H/minute=%M"
					s3_partition_timezone = "UTC"
				}
				sqs {
					queue_url = "https://sqs.us-east-1.amazonaws.com/123456789012/alloy"
					region = "us-east-1"
					endpoint = "http://localhost:4566"
					wait_time_seconds = 15
					max_number_of_messages = 5
				}
				output {}
			`,
			expected: awss3receiver.Config{
				S3Downloader: awss3receiver.S3DownloaderConfig{
					Region:              "us-west-2",
					S3Bucket:            "grafana-bucket",
					S3Prefix:            "ingest/",
					S3PartitionFormat:   "year=%Y/month=%m/day=%d/hour=%H/minute=%M",
					S3PartitionTimezone: "UTC",
					EndpointPartitionID: "aws",
				},
				SQS: &awss3receiver.SQSConfig{
					QueueURL:            "https://sqs.us-east-1.amazonaws.com/123456789012/alloy",
					Region:              "us-east-1",
					Endpoint:            "http://localhost:4566",
					WaitTimeSeconds:     int64Ptr(15),
					MaxNumberOfMessages: int64Ptr(5),
				},
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var args awss3.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			actual, err := args.Convert()
			require.NoError(t, err)

			cfg := actual.(*awss3receiver.Config)
			require.Equal(t, tc.expected, *cfg)
		})
	}
}

func TestArguments_Validate(t *testing.T) {
	cases := []struct {
		name        string
		cfg         string
		expectError bool
	}{
		{
			name: "missing time window or sqs is invalid",
			cfg: `
				s3downloader {
					s3_bucket = "grafana-logs"
					s3_prefix = "logs/"
					s3_partition = "hour"
				}
				output {}
			`,
			expectError: true,
		},
		{
			name: "time bounded download validates",
			cfg: `
				start_time = "2024-01-01T00:00:00Z"
				end_time = "2024-01-01T12:00:00Z"
				s3downloader {
					s3_bucket = "grafana-logs"
					s3_prefix = "logs/"
				}
				output {}
			`,
			expectError: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var args awss3.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			if tc.expectError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
