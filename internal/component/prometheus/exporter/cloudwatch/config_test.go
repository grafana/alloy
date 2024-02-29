package cloudwatch

import (
	"testing"

	"github.com/grafana/river"
	yaceConf "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/config"
	yaceModel "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/stretchr/testify/require"
)

var truePtr = true
var falsePtr = false

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
	namespace = "AWS/EC2"
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
discovery_exported_tags = { "ec2" = ["name"] }
discovery {
	type = "sqs"
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
	metric {
		name = "BucketSizeBytes"
		statistics = ["Sum"]
		period = "1m"
		length = "1h"
	}
}
`

const staticJobNilToZeroConfig = `
sts_region = "us-east-2"
debug = true
static "super_ec2_instance_id" {
	regions = ["us-east-2"]
	namespace = "AWS/EC2"
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
	namespace = "AWS/EC2"
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
discovery_exported_tags = { "ec2" = ["name"] }
discovery {
	type = "sqs"
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

func TestCloudwatchComponentConfig(t *testing.T) {
	type testcase struct {
		raw                 string
		expected            yaceConf.ScrapeConf
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
			expected: yaceConf.ScrapeConf{
				APIVersion: "v1alpha1",
				StsRegion:  "us-east-2",
				Discovery:  yaceConf.Discovery{},
				Static: []*yaceConf.Static{
					{
						Name: "super_ec2_instance_id",
						// assert an empty role is used as default. IMPORTANT since this
						// is what YACE looks for delegating to the environment role
						Roles:      []yaceConf.Role{{}},
						Regions:    []string{"us-east-2"},
						Namespace:  "AWS/EC2",
						CustomTags: []yaceModel.Tag{},
						Dimensions: []yaceConf.Dimension{
							{
								Name:  "InstanceId",
								Value: "i01u29u12ue1u2c",
							},
						},
						Metrics: []*yaceConf.Metric{{
							Name:                   "CPUUsage",
							Statistics:             []string{"Sum", "Average"},
							Period:                 60,
							Length:                 60,
							Delay:                  0,
							NilToZero:              &defaultNilToZero,
							AddCloudwatchTimestamp: &addCloudwatchTimestamp,
						}},
					},
				},
			},
		},
		"single discovery job config": {
			raw: discoveryJobConfig,
			expected: yaceConf.ScrapeConf{
				APIVersion: "v1alpha1",
				StsRegion:  "us-east-2",
				Discovery: yaceConf.Discovery{
					ExportedTagsOnMetrics: yaceModel.ExportedTagsOnMetrics{
						"ec2": []string{"name"},
					},
					Jobs: []*yaceConf.Job{
						{
							Regions: []string{"us-east-2"},
							// assert an empty role is used as default. IMPORTANT since this
							// is what YACE looks for delegating to the environment role
							Roles: []yaceConf.Role{{}},
							Type:  "sqs",
							SearchTags: []yaceModel.Tag{{
								Key: "scrape", Value: "true",
							}},
							CustomTags: []yaceModel.Tag{},
							Metrics: []*yaceConf.Metric{
								{
									Name:                   "NumberOfMessagesSent",
									Statistics:             []string{"Sum", "Average"},
									Period:                 60,
									Length:                 60,
									Delay:                  0,
									NilToZero:              &defaultNilToZero,
									AddCloudwatchTimestamp: &addCloudwatchTimestamp,
								},
								{
									Name:                   "NumberOfMessagesReceived",
									Statistics:             []string{"Sum", "Average"},
									Period:                 60,
									Length:                 60,
									Delay:                  0,
									NilToZero:              &defaultNilToZero,
									AddCloudwatchTimestamp: &addCloudwatchTimestamp,
								},
							},
							RoundingPeriod: nil,
							JobLevelMetricFields: yaceConf.JobLevelMetricFields{
								Period:                 0,
								Length:                 0,
								Delay:                  0,
								AddCloudwatchTimestamp: &falsePtr,
								NilToZero:              &defaultNilToZero,
							},
						},
						{
							Regions: []string{"us-east-1"},
							Roles: []yaceConf.Role{{
								RoleArn: "arn:aws:iam::878167871295:role/yace_testing",
							}},
							Type:       "AWS/ECS",
							SearchTags: []yaceModel.Tag{},
							CustomTags: []yaceModel.Tag{},
							Metrics: []*yaceConf.Metric{
								{
									Name:                   "CPUUtilization",
									Statistics:             []string{"Sum", "Maximum"},
									Period:                 60,
									Length:                 60,
									Delay:                  0,
									NilToZero:              &defaultNilToZero,
									AddCloudwatchTimestamp: &addCloudwatchTimestamp,
								},
							},
							RoundingPeriod: nil,
							JobLevelMetricFields: yaceConf.JobLevelMetricFields{
								Period:                 0,
								Length:                 0,
								Delay:                  0,
								AddCloudwatchTimestamp: &falsePtr,
								NilToZero:              &defaultNilToZero,
							},
						},
						{
							Regions: []string{"us-east-1"},
							Roles: []yaceConf.Role{{
								RoleArn: "arn:aws:iam::878167871295:role/yace_testing",
							}},
							Type:                      "s3",
							SearchTags:                []yaceModel.Tag{},
							CustomTags:                []yaceModel.Tag{},
							DimensionNameRequirements: []string{"BucketName"},
							Metrics: []*yaceConf.Metric{
								{
									Name:                   "BucketSizeBytes",
									Statistics:             []string{"Sum"},
									Period:                 60,
									Length:                 3600,
									Delay:                  0,
									NilToZero:              &defaultNilToZero,
									AddCloudwatchTimestamp: &addCloudwatchTimestamp,
								},
							},
							RoundingPeriod: nil,
							JobLevelMetricFields: yaceConf.JobLevelMetricFields{
								Period:                 0,
								Length:                 0,
								Delay:                  0,
								AddCloudwatchTimestamp: &falsePtr,
								NilToZero:              &defaultNilToZero,
							},
						},
					},
				},
			},
		},
		"static job nil to zero": {
			raw: staticJobNilToZeroConfig,
			expected: yaceConf.ScrapeConf{
				APIVersion: "v1alpha1",
				StsRegion:  "us-east-2",
				Discovery:  yaceConf.Discovery{},
				Static: []*yaceConf.Static{
					{
						Name: "super_ec2_instance_id",
						// assert an empty role is used as default. IMPORTANT since this
						// is what YACE looks for delegating to the environment role
						Roles:      []yaceConf.Role{{}},
						Regions:    []string{"us-east-2"},
						Namespace:  "AWS/EC2",
						CustomTags: []yaceModel.Tag{},
						Dimensions: []yaceConf.Dimension{
							{
								Name:  "InstanceId",
								Value: "i01u29u12ue1u2c",
							},
						},
						Metrics: []*yaceConf.Metric{{
							Name:                   "CPUUsage",
							Statistics:             []string{"Sum", "Average"},
							Period:                 60,
							Length:                 60,
							Delay:                  0,
							NilToZero:              &falsePtr,
							AddCloudwatchTimestamp: &addCloudwatchTimestamp,
						}},
					},
				},
			},
		},
		"static job nil to zero metric": {
			raw: staticJobNilToZeroMetricConfig,
			expected: yaceConf.ScrapeConf{
				APIVersion: "v1alpha1",
				StsRegion:  "us-east-2",
				Discovery:  yaceConf.Discovery{},
				Static: []*yaceConf.Static{
					{
						Name: "super_ec2_instance_id",
						// assert an empty role is used as default. IMPORTANT since this
						// is what YACE looks for delegating to the environment role
						Roles:      []yaceConf.Role{{}},
						Regions:    []string{"us-east-2"},
						Namespace:  "AWS/EC2",
						CustomTags: []yaceModel.Tag{},
						Dimensions: []yaceConf.Dimension{
							{
								Name:  "InstanceId",
								Value: "i01u29u12ue1u2c",
							},
						},
						Metrics: []*yaceConf.Metric{{
							Name:                   "CPUUsage",
							Statistics:             []string{"Sum", "Average"},
							Period:                 60,
							Length:                 60,
							Delay:                  0,
							NilToZero:              &falsePtr,
							AddCloudwatchTimestamp: &addCloudwatchTimestamp,
						}},
					},
				},
			},
		},
		"discovery job nil to zero config": {
			raw: discoveryJobNilToZeroConfig,
			expected: yaceConf.ScrapeConf{
				APIVersion: "v1alpha1",
				StsRegion:  "us-east-2",
				Discovery: yaceConf.Discovery{
					ExportedTagsOnMetrics: yaceModel.ExportedTagsOnMetrics{
						"ec2": []string{"name"},
					},
					Jobs: []*yaceConf.Job{
						{
							Regions: []string{"us-east-2"},
							// assert an empty role is used as default. IMPORTANT since this
							// is what YACE looks for delegating to the environment role
							Roles: []yaceConf.Role{{}},
							Type:  "sqs",
							SearchTags: []yaceModel.Tag{{
								Key: "scrape", Value: "true",
							}},
							CustomTags: []yaceModel.Tag{},
							Metrics: []*yaceConf.Metric{
								{
									Name:                   "NumberOfMessagesSent",
									Statistics:             []string{"Sum", "Average"},
									Period:                 60,
									Length:                 60,
									Delay:                  0,
									NilToZero:              &falsePtr,
									AddCloudwatchTimestamp: &addCloudwatchTimestamp,
								},
								{
									Name:                   "NumberOfMessagesReceived",
									Statistics:             []string{"Sum", "Average"},
									Period:                 60,
									Length:                 60,
									Delay:                  0,
									NilToZero:              &truePtr,
									AddCloudwatchTimestamp: &addCloudwatchTimestamp,
								},
							},
							RoundingPeriod: nil,
							JobLevelMetricFields: yaceConf.JobLevelMetricFields{
								Period:                 0,
								Length:                 0,
								Delay:                  0,
								AddCloudwatchTimestamp: &falsePtr,
								NilToZero:              &falsePtr,
							},
						},
					},
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			args := Arguments{}
			err := river.Unmarshal([]byte(tc.raw), &args)
			if tc.expectUnmarshallErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			converted, err := ConvertToYACE(args)
			if tc.expectConvertErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.EqualValues(t, tc.expected, converted)
		})
	}
}
