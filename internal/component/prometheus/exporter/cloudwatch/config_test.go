package cloudwatch

import (
	"io"
	"testing"

	"github.com/grafana/regexp"
	yaceModel "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/syntax"
)

var (
	truePtr  = true
	falsePtr = false
)

const invalidDiscoveryJobType = `
sts_region = "us-east-2"
debug = true
discovery {
	type = "pizza"
	regions = ["us-east-2"]
	search_tags = {
		"scrape" = "true",
	}
	metric {
		name = "PeperoniSlices"
		statistics = ["Sum", "Average"]
		period = "1m"
	}
}
`

const noJobsInConfig = `
sts_region = "us-east-2"
debug = true
`

const singleStaticJobConfig = `
sts_region = "us-east-2"
debug = true
static "super_ec2_instance_id" {
	regions = ["us-east-2"]
	namespace = "ec2"
	dimensions = {
		"InstanceId" = "i01u29u12ue1u2c",
	}
	metric {
		name = "CPUUsage"
		statistics = ["Sum", "Average"]
		period = "1m"
	}
}
`

const discoveryJobConfig = `
sts_region = "us-east-2"
debug = true
discovery_exported_tags = { "AWS/SQS" = ["name"] }
discovery {
	type = "AWS/SQS"
	regions = ["us-east-2"]
	search_tags = {
		"scrape" = "true",
	}
	metric {
		name = "NumberOfMessagesSent"
		statistics = ["Sum", "Average"]
		period = "1m"
	}
	metric {
		name = "NumberOfMessagesReceived"
		statistics = ["Sum", "Average"]
		period = "1m"
	}
}

discovery {
	type = "AWS/ECS"
	regions = ["us-east-1"]
	role {
		role_arn = "arn:aws:iam::878167871295:role/yace_testing"
	}
	metric {
		name = "CPUUtilization"
		statistics = ["Sum", "Maximum"]
		period = "1m"
	}
}

// the configuration below overrides the length
discovery {
	type = "s3"
	regions = ["us-east-1"]
	role {
		role_arn = "arn:aws:iam::878167871295:role/yace_testing"
	}
	dimension_name_requirements = ["BucketName"]
	recently_active_only = true
	metric {
		name = "BucketSizeBytes"
		statistics = ["Sum"]
		period = "1m"
		length = "1h"
	}
}
`

const customNamespaceJobConfig = `
sts_region = "eu-west-1"

custom_namespace "customEC2Metrics" {
    namespace = "CustomEC2Metrics"
    regions   = ["us-east-1"]

    metric {
        name       = "cpu_usage_idle"
        statistics = ["Average"]
        period     = "5m"
    }

    metric {
        name       = "disk_free"
        statistics = ["Average"]
        period     = "5m"
    }
}
`

const staticJobNilToZeroConfig = `
sts_region = "us-east-2"
debug = true
static "super_ec2_instance_id" {
	regions = ["us-east-2"]
	namespace = "ec2"
	dimensions = {
		"InstanceId" = "i01u29u12ue1u2c",
	}
	metric {
		name = "CPUUsage"
		statistics = ["Sum", "Average"]
		period = "1m"
	}
	// setting nil_to_zero on the job level
	nil_to_zero = false
}
`

const staticJobNilToZeroMetricConfig = `
sts_region = "us-east-2"
debug = true
static "super_ec2_instance_id" {
	regions = ["us-east-2"]
	namespace = "ec2"
	dimensions = {
		"InstanceId" = "i01u29u12ue1u2c",
	}
	metric {
		name = "CPUUsage"
		statistics = ["Sum", "Average"]
		period = "1m"
		// setting nil_to_zero on the metric level
		nil_to_zero = false
	}
}
`

const discoveryJobNilToZeroConfig = `
sts_region = "us-east-2"
debug = true
discovery_exported_tags = { "AWS/SQS" = ["name"] }
discovery {
	type = "AWS/SQS"
	regions = ["us-east-2"]
	search_tags = {
		"scrape" = "true",
	}
	// setting nil_to_zero on the job level
	nil_to_zero = false
	metric {
		name = "NumberOfMessagesSent"
		statistics = ["Sum", "Average"]
		period = "1m"
	}
	metric {
		name = "NumberOfMessagesReceived"
		statistics = ["Sum", "Average"]
		period = "1m"
		// setting nil_to_zero on the metric level
		nil_to_zero = true
	}
}
`

const customNamespacebNilToZeroJobConfig = `
sts_region = "eu-west-1"

custom_namespace "customEC2Metrics" {
    namespace = "CustomEC2Metrics"
    regions   = ["us-east-1"]
	// setting nil_to_zero on the job level
	nil_to_zero = false

    metric {
        name       = "cpu_usage_idle"
        statistics = ["Average"]
        period     = "5m"
		add_cloudwatch_timestamp = true
    }

    metric {
        name       = "disk_free"
        statistics = ["Average"]
        period     = "5m"
		// setting nil_to_zero on the metric level
		nil_to_zero = true
		add_cloudwatch_timestamp = false
    }
}
`

func TestCloudwatchComponentConfig(t *testing.T) {
	type testcase struct {
		raw                 string
		expected            yaceModel.JobsConfig
		expectUnmarshallErr bool
		expectConvertErr    bool
	}

	for name, tc := range map[string]testcase{
		"error unmarshalling": {
			raw:                 ``,
			expectUnmarshallErr: true,
		},
		"error converting": {
			raw:              invalidDiscoveryJobType,
			expectConvertErr: true,
		},
		"at least one static or discovery job is required": {
			raw:              noJobsInConfig,
			expectConvertErr: true,
		},
		"single static job config": {
			raw: singleStaticJobConfig,
			expected: yaceModel.JobsConfig{
				StsRegion: "us-east-2",
				StaticJobs: []yaceModel.StaticJob{
					{
						Name: "super_ec2_instance_id",
						// assert an empty role is used as default. IMPORTANT since this
						// is what YACE looks for delegating to the environment role
						Roles:      []yaceModel.Role{{}},
						Regions:    []string{"us-east-2"},
						Namespace:  "AWS/EC2",
						CustomTags: []yaceModel.Tag{},
						Dimensions: []yaceModel.Dimension{
							{
								Name:  "InstanceId",
								Value: "i01u29u12ue1u2c",
							},
						},
						Metrics: []*yaceModel.MetricConfig{{
							Name:       "CPUUsage",
							Statistics: []string{"Sum", "Average"},
							Period:     60,
							Length:     60,
							Delay:      0,
							NilToZero:  defaultNilToZero,
						}},
					},
				},
			},
		},
		"single discovery job config": {
			raw: discoveryJobConfig,
			expected: yaceModel.JobsConfig{
				StsRegion: "us-east-2",
				DiscoveryJobs: []yaceModel.DiscoveryJob{
					{
						Regions: []string{"us-east-2"},
						// assert an empty role is used as default. IMPORTANT since this
						// is what YACE looks for delegating to the environment role
						Roles: []yaceModel.Role{{}},
						Type:  "AWS/SQS",
						SearchTags: []yaceModel.SearchTag{{
							Key: "scrape", Value: regexp.MustCompile("true"),
						}},
						CustomTags: []yaceModel.Tag{},
						Metrics: []*yaceModel.MetricConfig{
							{
								Name:       "NumberOfMessagesSent",
								Statistics: []string{"Sum", "Average"},
								Period:     60,
								Length:     60,
								Delay:      0,
								NilToZero:  defaultNilToZero,
							},
							{
								Name:       "NumberOfMessagesReceived",
								Statistics: []string{"Sum", "Average"},
								Period:     60,
								Length:     60,
								Delay:      0,
								NilToZero:  defaultNilToZero,
							},
						},
						RoundingPeriod:        nil,
						ExportedTagsOnMetrics: []string{"name"},
						DimensionsRegexps: []yaceModel.DimensionsRegexp{
							{
								Regexp:          regexp.MustCompile("(?P<QueueName>[^:]+)$"),
								DimensionsNames: []string{"QueueName"},
							},
						},
					},
					{
						Regions: []string{"us-east-1"},
						Roles: []yaceModel.Role{{
							RoleArn: "arn:aws:iam::878167871295:role/yace_testing",
						}},
						Type:       "AWS/ECS",
						SearchTags: []yaceModel.SearchTag{},
						CustomTags: []yaceModel.Tag{},
						Metrics: []*yaceModel.MetricConfig{
							{
								Name:       "CPUUtilization",
								Statistics: []string{"Sum", "Maximum"},
								Period:     60,
								Length:     60,
								Delay:      0,
								NilToZero:  defaultNilToZero,
							},
						},
						RoundingPeriod:        nil,
						ExportedTagsOnMetrics: []string{},
						DimensionsRegexps: []yaceModel.DimensionsRegexp{
							{
								Regexp:          regexp.MustCompile(":cluster/(?P<ClusterName>[^/]+)$"),
								DimensionsNames: []string{"ClusterName"},
							},
							{
								Regexp:          regexp.MustCompile(":service/(?P<ClusterName>[^/]+)/(?P<ServiceName>[^/]+)$"),
								DimensionsNames: []string{"ClusterName", "ServiceName"},
							},
						},
					},
					{
						Regions: []string{"us-east-1"},
						Roles: []yaceModel.Role{{
							RoleArn: "arn:aws:iam::878167871295:role/yace_testing",
						}},
						Type:                      "AWS/S3",
						SearchTags:                []yaceModel.SearchTag{},
						CustomTags:                []yaceModel.Tag{},
						DimensionNameRequirements: []string{"BucketName"},
						Metrics: []*yaceModel.MetricConfig{
							{
								Name:       "BucketSizeBytes",
								Statistics: []string{"Sum"},
								Period:     60,
								Length:     3600,
								Delay:      0,
								NilToZero:  defaultNilToZero,
							},
						},
						RoundingPeriod:        nil,
						ExportedTagsOnMetrics: []string{},
						RecentlyActiveOnly:    true,
						DimensionsRegexps: []yaceModel.DimensionsRegexp{
							{
								Regexp:          regexp.MustCompile("(?P<BucketName>[^:]+)$"),
								DimensionsNames: []string{"BucketName"},
							},
						},
					},
				},
			},
		},
		"single custom namespace job config": {
			raw: customNamespaceJobConfig,
			expected: yaceModel.JobsConfig{
				StsRegion: "eu-west-1",
				CustomNamespaceJobs: []yaceModel.CustomNamespaceJob{
					{
						Name:    "customEC2Metrics",
						Regions: []string{"us-east-1"},
						// assert an empty role is used as default. IMPORTANT since this
						// is what YACE looks for delegating to the environment role
						Roles:      []yaceModel.Role{{}},
						CustomTags: []yaceModel.Tag{},
						Namespace:  "CustomEC2Metrics",
						Metrics: []*yaceModel.MetricConfig{
							{
								Name:       "cpu_usage_idle",
								Statistics: []string{"Average"},
								Period:     300,
								Length:     300,
								Delay:      0,
								NilToZero:  defaultNilToZero,
							},
							{
								Name:       "disk_free",
								Statistics: []string{"Average"},
								Period:     300,
								Length:     300,
								Delay:      0,
								NilToZero:  defaultNilToZero,
							},
						},
						RoundingPeriod: nil,
					},
				},
			},
		},
		"static job nil to zero": {
			raw: staticJobNilToZeroConfig,
			expected: yaceModel.JobsConfig{
				StsRegion: "us-east-2",
				StaticJobs: []yaceModel.StaticJob{
					{
						Name: "super_ec2_instance_id",
						// assert an empty role is used as default. IMPORTANT since this
						// is what YACE looks for delegating to the environment role
						Roles:      []yaceModel.Role{{}},
						Regions:    []string{"us-east-2"},
						Namespace:  "AWS/EC2",
						CustomTags: []yaceModel.Tag{},
						Dimensions: []yaceModel.Dimension{
							{
								Name:  "InstanceId",
								Value: "i01u29u12ue1u2c",
							},
						},
						Metrics: []*yaceModel.MetricConfig{{
							Name:       "CPUUsage",
							Statistics: []string{"Sum", "Average"},
							Period:     60,
							Length:     60,
							Delay:      0,
							NilToZero:  falsePtr,
						}},
					},
				},
			},
		},
		"static job nil to zero metric": {
			raw: staticJobNilToZeroMetricConfig,
			expected: yaceModel.JobsConfig{
				StsRegion: "us-east-2",
				StaticJobs: []yaceModel.StaticJob{
					{
						Name: "super_ec2_instance_id",
						// assert an empty role is used as default. IMPORTANT since this
						// is what YACE looks for delegating to the environment role
						Roles:      []yaceModel.Role{{}},
						Regions:    []string{"us-east-2"},
						Namespace:  "AWS/EC2",
						CustomTags: []yaceModel.Tag{},
						Dimensions: []yaceModel.Dimension{
							{
								Name:  "InstanceId",
								Value: "i01u29u12ue1u2c",
							},
						},
						Metrics: []*yaceModel.MetricConfig{{
							Name:       "CPUUsage",
							Statistics: []string{"Sum", "Average"},
							Period:     60,
							Length:     60,
							Delay:      0,
							NilToZero:  falsePtr,
						}},
					},
				},
			},
		},
		"discovery job nil to zero config": {
			raw: discoveryJobNilToZeroConfig,
			expected: yaceModel.JobsConfig{
				StsRegion: "us-east-2",
				DiscoveryJobs: []yaceModel.DiscoveryJob{
					{
						Regions: []string{"us-east-2"},
						// assert an empty role is used as default. IMPORTANT since this
						// is what YACE looks for delegating to the environment role
						Roles: []yaceModel.Role{{}},
						Type:  "AWS/SQS",
						SearchTags: []yaceModel.SearchTag{{
							Key: "scrape", Value: regexp.MustCompile("true"),
						}},
						CustomTags: []yaceModel.Tag{},
						Metrics: []*yaceModel.MetricConfig{
							{
								Name:       "NumberOfMessagesSent",
								Statistics: []string{"Sum", "Average"},
								Period:     60,
								Length:     60,
								Delay:      0,
								NilToZero:  falsePtr,
							},
							{
								Name:       "NumberOfMessagesReceived",
								Statistics: []string{"Sum", "Average"},
								Period:     60,
								Length:     60,
								Delay:      0,
								NilToZero:  truePtr,
							},
						},
						RoundingPeriod:        nil,
						ExportedTagsOnMetrics: []string{"name"},
						DimensionsRegexps: []yaceModel.DimensionsRegexp{
							{
								Regexp:          regexp.MustCompile("(?P<QueueName>[^:]+)$"),
								DimensionsNames: []string{"QueueName"},
							},
						},
					},
				},
			},
		},
		"custom namespace job nil to zero config": {
			raw: customNamespacebNilToZeroJobConfig,
			expected: yaceModel.JobsConfig{
				StsRegion: "eu-west-1",
				CustomNamespaceJobs: []yaceModel.CustomNamespaceJob{
					{
						Name:    "customEC2Metrics",
						Regions: []string{"us-east-1"},
						// assert an empty role is used as default. IMPORTANT since this
						// is what YACE looks for delegating to the environment role
						Roles:      []yaceModel.Role{{}},
						CustomTags: []yaceModel.Tag{},
						Namespace:  "CustomEC2Metrics",
						Metrics: []*yaceModel.MetricConfig{
							{
								Name:                   "cpu_usage_idle",
								Statistics:             []string{"Average"},
								Period:                 300,
								Length:                 300,
								Delay:                  0,
								NilToZero:              falsePtr,
								AddCloudwatchTimestamp: truePtr,
							},
							{
								Name:                   "disk_free",
								Statistics:             []string{"Average"},
								Period:                 300,
								Length:                 300,
								Delay:                  0,
								NilToZero:              truePtr,
								AddCloudwatchTimestamp: falsePtr,
							},
						},
						RoundingPeriod: nil,
					},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			args := Arguments{}
			err := syntax.Unmarshal([]byte(tc.raw), &args)
			if tc.expectUnmarshallErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			logger, err := logging.New(io.Discard, logging.DefaultOptions)
			require.NoError(t, err)

			converted, err := ConvertToYACE(args, logger)
			if tc.expectConvertErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.EqualValues(t, tc.expected, converted)
		})
	}
}
