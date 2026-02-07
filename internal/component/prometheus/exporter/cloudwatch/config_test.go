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

const customNamespaceInvalidPeriodConfig = `
sts_region = "eu-west-1"

custom_namespace "testMetrics" {
    namespace = "TestMetrics"
    regions   = ["us-east-1"]
		period = "5m"

    metric {
        name       = "metric1"
        statistics = ["Average"]
        length     = "5m"
    }

    metric {
        name       = "metric2"
        statistics = ["Sum"]
        length     = "2m"
    }
}
`

// ===========
// Single Jobs
//============

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

var expectedSingleStaticJobConfig = yaceModel.JobsConfig{
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
}

const singleCustomNamespaceJobConfig = `
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

var expectedCustomNamespaceJobConfig = yaceModel.JobsConfig{
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
}

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

var expectedDiscoveryJobConfig = yaceModel.JobsConfig{
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
}

// ================
// nil_to_zero jobs
//=================

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

var expectedStaticJobNilToZeroConfig = yaceModel.JobsConfig{
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
}

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

var expectedStaticJobNilToZeroMetricConfig = yaceModel.JobsConfig{
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
}

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

var expectedDiscoveryJobNilToZeroConfig = yaceModel.JobsConfig{
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
}

const customNamespaceNilToZeroJobConfig = `
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
    }
}
`

var expectedCustomNamespaceJobNilToZeroConfig = yaceModel.JobsConfig{
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
}

// ================
// Period jobs
//=================

// Shows that the default period and length of 5m is used. Static jobs do not support job level period and length settings.
const staticJobDefaultPeriodConfig = `
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
	}
}`

var expectedStaticJobDefaultPeriodConfig = yaceModel.JobsConfig{
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
				Period:     300,
				Length:     300,
				Delay:      0,
				NilToZero:  defaultNilToZero,
			}},
		},
	},
}

// Shows that the default period and length of 5m is used when not set on the job or metric level.
const customNamespaceDefaultPeriodConfig = `
sts_region = "eu-west-1"

custom_namespace "customEC2Metrics" {
    namespace = "CustomEC2Metrics"
    regions   = ["us-east-1"]

    metric {
        name       = "cpu_usage_idle"
        statistics = ["Average"]
    }

    metric {
        name       = "disk_free"
        statistics = ["Average"]
    }
}
`

var expectedCustomNamespaceJobDefaultPeriodConfig = yaceModel.JobsConfig{
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
}

// Shows that the default period and length of 5m is used when not set on the job or metric level.
const discoveryJobDefaultPeriodConfig = `
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
	}
	metric {
		name = "NumberOfMessagesReceived"
		statistics = ["Sum", "Average"]
	}
}
`

var expectedDiscoveryJobDefaultPeriodConfig = yaceModel.JobsConfig{
	StsRegion: "us-east-2",
	DiscoveryJobs: []yaceModel.DiscoveryJob{
		{
			Regions: []string{"us-east-2"},
			Roles:   []yaceModel.Role{{}},
			Type:    "AWS/SQS",
			SearchTags: []yaceModel.SearchTag{{
				Key: "scrape", Value: regexp.MustCompile("true"),
			}},
			CustomTags: []yaceModel.Tag{},
			Metrics: []*yaceModel.MetricConfig{
				{
					Name:       "NumberOfMessagesSent",
					Statistics: []string{"Sum", "Average"},
					Period:     300,
					Length:     300,
					Delay:      0,
					NilToZero:  defaultNilToZero,
				},
				{
					Name:       "NumberOfMessagesReceived",
					Statistics: []string{"Sum", "Average"},
					Period:     300,
					Length:     300,
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
	},
}

// Shows we can set period on the job level and override it on the metric level.
const discoveryJobPeriodConfig = `
sts_region = "us-east-2"
debug = true
discovery_exported_tags = { "AWS/SQS" = ["name"] }
discovery {
	type = "AWS/SQS"
	regions = ["us-east-2"]
	search_tags = {
		"scrape" = "true",
	}
	period = "3m"
	metric {
		name = "NumberOfMessagesSent"
		statistics = ["Sum", "Average"]
	}
	metric {
		name = "NumberOfMessagesReceived"
		statistics = ["Sum", "Average"]
		period = "1m"
	}
}
`

var expectedDiscoveryJobPeriodConfig = yaceModel.JobsConfig{
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
					Period:     180,
					Length:     180,
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
	},
}

// Shows we can set period on the job level and override it on the metric level.
const customNameSpaceJobPeriodConfig = `
sts_region = "eu-west-1"

custom_namespace "customEC2Metrics" {
    namespace = "CustomEC2Metrics"
    regions   = ["us-east-1"]
		period = "3m"
    metric {
        name       = "cpu_usage_idle"
        statistics = ["Average"]
    }

    metric {
        name       = "disk_free"
        statistics = ["Average"]
				// Override on metric level
        period     = "1m"
    }
}
`

var expectedCustomNamespaceJobPeriodConfig = yaceModel.JobsConfig{
	StsRegion: "eu-west-1",
	CustomNamespaceJobs: []yaceModel.CustomNamespaceJob{
		{
			Name:       "customEC2Metrics",
			Namespace:  "CustomEC2Metrics",
			Regions:    []string{"us-east-1"},
			Roles:      []yaceModel.Role{{}},
			CustomTags: []yaceModel.Tag{},
			Metrics: []*yaceModel.MetricConfig{
				{
					Name:       "cpu_usage_idle",
					Statistics: []string{"Average"},
					Period:     180,
					Length:     180,
					Delay:      0,
					NilToZero:  defaultNilToZero,
				},
				{
					Name:       "disk_free",
					Statistics: []string{"Average"},
					Period:     60,
					Length:     60,
					Delay:      0,
					NilToZero:  defaultNilToZero,
				},
			},
		},
	},
}

// =============================
// add_cloudwatch_timestamp jobs
//==============================

// Shows that add_cloudwatch_timestamp is not supported on static jobs will be set to false on metrics.
const staticJobAddCloudwatchTimestampConfig = `
sts_region = "us-east-2"
debug = true
static "test_instance" {
	regions = ["us-east-2"]
	namespace = "AWS/EC2"
	dimensions = {
		"InstanceId" = "i-test",
	}
	metric {
		name = "CPUUtilization"
		statistics = ["Average"]
		period = "5m"
	}
}
`

var expectedStaticJobAddCloudwatchTimestampConfig = yaceModel.JobsConfig{
	StsRegion: "us-east-2",
	StaticJobs: []yaceModel.StaticJob{
		{
			Name:       "test_instance",
			Roles:      []yaceModel.Role{{}},
			Regions:    []string{"us-east-2"},
			Namespace:  "AWS/EC2",
			CustomTags: []yaceModel.Tag{},
			Dimensions: []yaceModel.Dimension{
				{
					Name:  "InstanceId",
					Value: "i-test",
				},
			},
			Metrics: []*yaceModel.MetricConfig{{
				Name:                   "CPUUtilization",
				Statistics:             []string{"Average"},
				Period:                 300,
				Length:                 300,
				Delay:                  0,
				NilToZero:              defaultNilToZero,
				AddCloudwatchTimestamp: falsePtr,
			}},
		},
	},
}

// Shows we can set add_cloudwatch_timestamp on the job level and override it on the metric level.
const customNamespaceAddCloudwatchTimestampConfig = `
sts_region = "eu-west-1"

custom_namespace "customEC2Metrics" {
    namespace = "CustomEC2Metrics"
    regions   = ["us-east-1"]
		add_cloudwatch_timestamp = true

    metric {
        name       = "cpu_usage_idle"
        statistics = ["Average"]
        period     = "5m"
    }

    metric {
        name       = "disk_free"
        statistics = ["Average"]
        period     = "5m"
				add_cloudwatch_timestamp = false
    }
}
`

var expectedCustomNamespaceJobAddCloudwatchTimestampConfig = yaceModel.JobsConfig{
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
					NilToZero:              defaultNilToZero,
					AddCloudwatchTimestamp: truePtr,
				},
				{
					Name:                   "disk_free",
					Statistics:             []string{"Average"},
					Period:                 300,
					Length:                 300,
					Delay:                  0,
					NilToZero:              defaultNilToZero,
					AddCloudwatchTimestamp: falsePtr,
				},
			},
			RoundingPeriod: nil,
		},
	},
}

// Shows we can set add_cloudwatch_timestamp on the job level and override it on the metric level.
const discoveryJobAddCloudwatchTimestampConfig = `
sts_region = "us-east-2"
debug = true
discovery_exported_tags = { "AWS/SQS" = ["name"] }
discovery {
	type = "AWS/SQS"
	regions = ["us-east-2"]
	search_tags = {
		"scrape" = "true",
	}
	add_cloudwatch_timestamp = true
	metric {
		name = "NumberOfMessagesSent"
		statistics = ["Sum", "Average"]
		period = "1m"
	}
	metric {
		name = "NumberOfMessagesReceived"
		statistics = ["Sum", "Average"]
		period = "1m"
		add_cloudwatch_timestamp = false
	}
}
`

var expectedDiscoveryJobAddCloudwatchTimestampConfig = yaceModel.JobsConfig{
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
					Name:                   "NumberOfMessagesSent",
					Statistics:             []string{"Sum", "Average"},
					Period:                 60,
					Length:                 60,
					Delay:                  0,
					NilToZero:              defaultNilToZero,
					AddCloudwatchTimestamp: truePtr,
				},
				{
					Name:                   "NumberOfMessagesReceived",
					Statistics:             []string{"Sum", "Average"},
					Period:                 60,
					Length:                 60,
					Delay:                  0,
					NilToZero:              defaultNilToZero,
					AddCloudwatchTimestamp: falsePtr,
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
}

// ==========
// delay jobs
//===========

// Shows that job level delay is not supported for static jobs and will be set to zero.
const staticJobDelayConfig = `
sts_region = "us-east-2"
debug = true
static "test_instance" {
	regions = ["us-east-2"]
	namespace = "AWS/EC2"
	dimensions = {
		"InstanceId" = "i-test",
	}
	metric {
		name = "CPUUtilization"
		statistics = ["Average"]
		period = "5m"
	}
}
`

var expectedStaticJobDelayConfig = yaceModel.JobsConfig{
	StsRegion: "us-east-2",
	StaticJobs: []yaceModel.StaticJob{
		{
			Name:       "test_instance",
			Roles:      []yaceModel.Role{{}},
			Regions:    []string{"us-east-2"},
			Namespace:  "AWS/EC2",
			CustomTags: []yaceModel.Tag{},
			Dimensions: []yaceModel.Dimension{
				{
					Name:  "InstanceId",
					Value: "i-test",
				},
			},
			Metrics: []*yaceModel.MetricConfig{{
				Name:       "CPUUtilization",
				Statistics: []string{"Average"},
				Period:     300,
				Length:     300,
				Delay:      0, // Delay not supported for static jobs
				NilToZero:  defaultNilToZero,
			}},
		},
	},
}

// Shows that the job level delay is used for all metrics in the job.
const discoveryJobDelayConfig = `
sts_region = "us-east-2"
debug = true
discovery {
	type = "AWS/EC2"
	regions = ["us-east-2"]
	delay = "2m"
	metric {
		name = "CPUUtilization"
		statistics = ["Average"]
		period = "5m"
	}
	metric {
		name = "NetworkIn"
		statistics = ["Sum"]
		period = "5m"
	}
}
`

var expectedDiscoveryJobDelayConfig = yaceModel.JobsConfig{
	StsRegion: "us-east-2",
	DiscoveryJobs: []yaceModel.DiscoveryJob{
		{
			Regions:    []string{"us-east-2"},
			Roles:      []yaceModel.Role{{}},
			Type:       "AWS/EC2",
			SearchTags: []yaceModel.SearchTag{},
			CustomTags: []yaceModel.Tag{},
			Metrics: []*yaceModel.MetricConfig{
				{
					Name:       "CPUUtilization",
					Statistics: []string{"Average"},
					Period:     300,
					Length:     300,
					Delay:      120,
					NilToZero:  defaultNilToZero,
				},
				{
					Name:       "NetworkIn",
					Statistics: []string{"Sum"},
					Period:     300,
					Length:     300,
					Delay:      120,
					NilToZero:  defaultNilToZero,
				},
			},
			RoundingPeriod:        nil,
			ExportedTagsOnMetrics: []string{},
			DimensionsRegexps: []yaceModel.DimensionsRegexp{
				{
					Regexp:          regexp.MustCompile("instance/(?P<InstanceId>[^/]+)"),
					DimensionsNames: []string{"InstanceId"},
				},
			},
		},
	},
}

const customNamespaceDelayConfig = `
sts_region = "eu-west-1"

custom_namespace "testMetrics" {
    namespace = "TestMetrics"
    regions   = ["us-east-1"]
		delay = "30s"

    metric {
        name       = "metric1"
        statistics = ["Average"]
        period     = "1m"
    }

    metric {
        name       = "metric2"
        statistics = ["Sum"]
        period     = "1m"
    }
}
`

var expectedCustomNamespaceDelayConfig = yaceModel.JobsConfig{
	StsRegion: "eu-west-1",
	CustomNamespaceJobs: []yaceModel.CustomNamespaceJob{
		{
			Name:       "testMetrics",
			Regions:    []string{"us-east-1"},
			Roles:      []yaceModel.Role{{}},
			CustomTags: []yaceModel.Tag{},
			Namespace:  "TestMetrics",
			Metrics: []*yaceModel.MetricConfig{
				{
					Name:       "metric1",
					Statistics: []string{"Average"},
					Period:     60,
					Length:     60,
					Delay:      30,
					NilToZero:  defaultNilToZero,
				},
				{
					Name:       "metric2",
					Statistics: []string{"Sum"},
					Period:     60,
					Length:     60,
					Delay:      30,
					NilToZero:  defaultNilToZero,
				},
			},
			RoundingPeriod: nil,
		},
	},
}

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
		"period cannot be greater than length": {
			raw:              customNamespaceInvalidPeriodConfig,
			expectConvertErr: true,
		},
		"single static job config": {
			raw:      singleStaticJobConfig,
			expected: expectedSingleStaticJobConfig,
		},
		"single custom namespace job config": {
			raw:      singleCustomNamespaceJobConfig,
			expected: expectedCustomNamespaceJobConfig,
		},
		"multiple discovery job config": {
			raw:      discoveryJobConfig,
			expected: expectedDiscoveryJobConfig,
		},
		"static job nil to zero": {
			raw:      staticJobNilToZeroConfig,
			expected: expectedStaticJobNilToZeroConfig,
		},
		"static job nil to zero metric": {
			raw:      staticJobNilToZeroMetricConfig,
			expected: expectedStaticJobNilToZeroMetricConfig,
		},
		"discovery job nil to zero config": {
			raw:      discoveryJobNilToZeroConfig,
			expected: expectedDiscoveryJobNilToZeroConfig,
		},
		"custom namespace job nil to zero config": {
			raw:      customNamespaceNilToZeroJobConfig,
			expected: expectedCustomNamespaceJobNilToZeroConfig,
		},
		"static job default period config": {
			raw:      staticJobDefaultPeriodConfig,
			expected: expectedStaticJobDefaultPeriodConfig,
		},
		"custom namespace job default period config": {
			raw:      customNamespaceDefaultPeriodConfig,
			expected: expectedCustomNamespaceJobDefaultPeriodConfig,
		},
		"discovery job default period config": {
			raw:      discoveryJobDefaultPeriodConfig,
			expected: expectedDiscoveryJobDefaultPeriodConfig,
		},
		"discovery job period config": {
			raw:      discoveryJobPeriodConfig,
			expected: expectedDiscoveryJobPeriodConfig,
		},
		"custom namespace job period config": {
			raw:      customNameSpaceJobPeriodConfig,
			expected: expectedCustomNamespaceJobPeriodConfig,
		},
		"static job add cloudwatch timestamp config": {
			raw:      staticJobAddCloudwatchTimestampConfig,
			expected: expectedStaticJobAddCloudwatchTimestampConfig,
		},
		"custom namespace job add cloudwatch timestamp config": {
			raw:      customNamespaceAddCloudwatchTimestampConfig,
			expected: expectedCustomNamespaceJobAddCloudwatchTimestampConfig,
		},
		"discovery job add cloudwatch timestamp config": {
			raw:      discoveryJobAddCloudwatchTimestampConfig,
			expected: expectedDiscoveryJobAddCloudwatchTimestampConfig,
		},
		"static job with delay": {
			raw:      staticJobDelayConfig,
			expected: expectedStaticJobDelayConfig,
		},
		"discovery job with delay": {
			raw:      discoveryJobDelayConfig,
			expected: expectedDiscoveryJobDelayConfig,
		},
		"custom namespace job with delay": {
			raw:      customNamespaceDelayConfig,
			expected: expectedCustomNamespaceDelayConfig,
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
