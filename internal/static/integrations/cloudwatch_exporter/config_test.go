package cloudwatch_exporter

import (
	"io"
	"testing"
	"time"

	"github.com/grafana/regexp"
	"github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/runtime/logging"
)

const configString = `
sts_region: us-east-2
discovery:
  exported_tags:
    AWS/EC2:
      - name
      - type
  jobs:
    - type: AWS/EC2
      search_tags:
        - key: instance_type
          value: spot
      regions:
        - us-east-2
      roles:
        - role_arn: arn:aws:iam::878167871295:role/yace_testing
      custom_tags:
        - key: alias
          value: tesis
      metrics:
        - name: CPUUtilization
          period: 5m
          statistics:
            - Maximum
            - Average
    - type: s3
      regions:
        - us-east-2
      roles:
        - role_arn: arn:aws:iam::878167871295:role/yace_testing
      dimension_name_requirements:
        - BucketName
      metrics:
        - name: BucketSizeBytes
          period: 5m
          length: 1h
          statistics:
            - Sum
static:
  - regions:
      - us-east-2
    name: custom_tesis_metrics
    namespace: CoolApp
    dimensions:
      - name: PURCHASES_SERVICE
        value: CoolService
      - name: APP_VERSION
        value: 1.0
    metrics:
      - name: KPIs
        period: 5m
        statistics:
          - Average
`

// for testing fips_disabled behaviour
const configString2 = `
sts_region: us-east-2
fips_disabled: true
aws_sdk_version_v2: false
discovery:
  exported_tags:
    AWS/EC2:
      - name
      - type
  jobs:
    - type: AWS/EC2
      search_tags:
        - key: instance_type
          value: spot
      regions:
        - us-east-2
      roles:
        - role_arn: arn:aws:iam::878167871295:role/yace_testing
      custom_tags:
        - key: alias
          value: tesis
      metrics:
        - name: CPUUtilization
          period: 5m
          statistics:
            - Maximum
            - Average
    - type: s3
      regions:
        - us-east-2
      roles:
        - role_arn: arn:aws:iam::878167871295:role/yace_testing
      dimension_name_requirements:
        - BucketName
      metrics:
        - name: BucketSizeBytes
          period: 5m
          length: 1h
          statistics:
            - Sum
static:
  - regions:
      - us-east-2
    name: custom_tesis_metrics
    namespace: CoolApp
    dimensions:
      - name: PURCHASES_SERVICE
        value: CoolService
      - name: APP_VERSION
        value: 1.0
    metrics:
      - name: KPIs
        period: 5m
        statistics:
          - Average
`

// for testing nilToZero at the DiscoveryJob, StaticJob, and Metric level
const configString3 = `
sts_region: us-east-2
discovery:
  exported_tags:
    AWS/EC2:
      - name
      - type
  jobs:
    - type: AWS/EC2
      search_tags:
        - key: instance_type
          value: spot
      regions:
        - us-east-2
      roles:
        - role_arn: arn:aws:iam::878167871295:role/yace_testing
      custom_tags:
        - key: alias
          value: tesis
      nil_to_zero: false
      metrics:
        - name: CPUUtilization
          period: 5m
          statistics:
            - Maximum
            - Average
    - type: s3
      regions:
        - us-east-2
      roles:
        - role_arn: arn:aws:iam::878167871295:role/yace_testing
      dimension_name_requirements:
        - BucketName
      nil_to_zero: true
      metrics:
        - name: BucketSizeBytes
          period: 5m
          length: 1h
          nil_to_zero: false
          statistics:
            - Sum
static:
  - regions:
      - us-east-2
    name: custom_tesis_metrics
    namespace: CoolApp
    dimensions:
      - name: PURCHASES_SERVICE
        value: CoolService
      - name: APP_VERSION
        value: 1.0
    nil_to_zero: false
    metrics:
      - name: KPIs
        period: 5m
        statistics:
          - Average
`

// for testing delay at the job level
const configString4 = `
sts_region: us-east-2
discovery:
  exported_tags:
    AWS/EC2:
      - name
      - type
  jobs:
    - type: AWS/EC2
      search_tags:
        - key: instance_type
          value: spot
      regions:
        - us-east-2
      roles:
        - role_arn: arn:aws:iam::878167871295:role/yace_testing
      custom_tags:
        - key: alias
          value: tesis
      delay: 1m
      metrics:
        - name: CPUUtilization
          period: 5m
          statistics:
            - Maximum
            - Average
static:
  - regions:
      - us-east-2
    name: custom_tesis_metrics
    namespace: CoolApp
    dimensions:
      - name: PURCHASES_SERVICE
        value: CoolService
      - name: APP_VERSION
        value: 1.0
    delay: 2m
    metrics:
      - name: KPIs
        period: 5m
        statistics:
          - Average
`

var (
	falsePtr = false
	truePtr  = true
)

var expectedConfig = model.JobsConfig{
	StsRegion: "us-east-2",
	DiscoveryJobs: []model.DiscoveryJob{{
		Regions:                   []string{"us-east-2"},
		Type:                      "AWS/EC2",
		Roles:                     []model.Role{{RoleArn: "arn:aws:iam::878167871295:role/yace_testing", ExternalID: ""}},
		SearchTags:                []model.SearchTag{{Key: "instance_type", Value: regexp.MustCompile("spot")}},
		CustomTags:                []model.Tag{{Key: "alias", Value: "tesis"}},
		DimensionNameRequirements: []string(nil),
		Metrics: []*model.MetricConfig{
			{
				Name:                   "CPUUtilization",
				Statistics:             []string{"Maximum", "Average"},
				Period:                 300,
				Length:                 300,
				Delay:                  0,
				NilToZero:              true,
				AddCloudwatchTimestamp: false,
			},
		},
		RoundingPeriod:              (*int64)(nil),
		RecentlyActiveOnly:          false,
		ExportedTagsOnMetrics:       []string{"name", "type"},
		IncludeContextOnInfoMetrics: false,
		DimensionsRegexps: []model.DimensionsRegexp{{
			Regexp:          regexp.MustCompile("instance/(?P<InstanceId>[^/]+)"),
			DimensionsNames: []string{"InstanceId"},
		}},
	}, {
		Regions:                   []string{"us-east-2"},
		Type:                      "AWS/S3",
		Roles:                     []model.Role{{RoleArn: "arn:aws:iam::878167871295:role/yace_testing", ExternalID: ""}},
		SearchTags:                []model.SearchTag{},
		CustomTags:                []model.Tag{},
		DimensionNameRequirements: []string{"BucketName"},
		Metrics: []*model.MetricConfig{
			{
				Name:                   "BucketSizeBytes",
				Statistics:             []string{"Sum"},
				Period:                 300,
				Length:                 3600,
				Delay:                  0,
				NilToZero:              true,
				AddCloudwatchTimestamp: false,
			},
		},
		RoundingPeriod:              (*int64)(nil),
		RecentlyActiveOnly:          false,
		ExportedTagsOnMetrics:       []string{},
		IncludeContextOnInfoMetrics: false,
		DimensionsRegexps: []model.DimensionsRegexp{{
			Regexp:          regexp.MustCompile("(?P<BucketName>[^:]+)$"),
			DimensionsNames: []string{"BucketName"},
		}},
	}},
	StaticJobs: []model.StaticJob{{
		Name:       "custom_tesis_metrics",
		Regions:    []string{"us-east-2"},
		Roles:      []model.Role{{RoleArn: "", ExternalID: ""}},
		Namespace:  "CoolApp",
		CustomTags: []model.Tag{},
		Dimensions: []model.Dimension{
			{Name: "PURCHASES_SERVICE", Value: "CoolService"},
			{Name: "APP_VERSION", Value: "1.0"},
		},
		Metrics: []*model.MetricConfig{
			{
				Name:                   "KPIs",
				Statistics:             []string{"Average"},
				Period:                 300,
				Length:                 300,
				Delay:                  0,
				NilToZero:              true,
				AddCloudwatchTimestamp: false,
			},
		},
	}},
	CustomNamespaceJobs: []model.CustomNamespaceJob(nil),
}

var expectedConfig3 = model.JobsConfig{
	StsRegion: "us-east-2",
	DiscoveryJobs: []model.DiscoveryJob{
		{
			Regions:                   []string{"us-east-2"},
			Type:                      "AWS/EC2",
			Roles:                     []model.Role{{RoleArn: "arn:aws:iam::878167871295:role/yace_testing", ExternalID: ""}},
			SearchTags:                []model.SearchTag{{Key: "instance_type", Value: regexp.MustCompile("spot")}},
			CustomTags:                []model.Tag{{Key: "alias", Value: "tesis"}},
			DimensionNameRequirements: []string(nil),
			Metrics: []*model.MetricConfig{{
				Name:                   "CPUUtilization",
				Statistics:             []string{"Maximum", "Average"},
				Period:                 300,
				Length:                 300,
				Delay:                  0,
				NilToZero:              false,
				AddCloudwatchTimestamp: false,
			}},
			RoundingPeriod:              (*int64)(nil),
			RecentlyActiveOnly:          false,
			ExportedTagsOnMetrics:       []string{"name", "type"},
			IncludeContextOnInfoMetrics: false,
			DimensionsRegexps: []model.DimensionsRegexp{{
				Regexp:          regexp.MustCompile("instance/(?P<InstanceId>[^/]+)"),
				DimensionsNames: []string{"InstanceId"},
			}},
		},
		{
			Regions: []string{"us-east-2"},
			Type:    "AWS/S3",
			Roles: []model.Role{{
				RoleArn:    "arn:aws:iam::878167871295:role/yace_testing",
				ExternalID: "",
			}},
			SearchTags:                []model.SearchTag{},
			CustomTags:                []model.Tag{},
			DimensionNameRequirements: []string{"BucketName"},
			Metrics: []*model.MetricConfig{{
				Name:                   "BucketSizeBytes",
				Statistics:             []string{"Sum"},
				Period:                 300,
				Length:                 3600,
				Delay:                  0,
				NilToZero:              false,
				AddCloudwatchTimestamp: false,
			}},
			RoundingPeriod:              (*int64)(nil),
			RecentlyActiveOnly:          false,
			ExportedTagsOnMetrics:       []string{},
			IncludeContextOnInfoMetrics: false,
			DimensionsRegexps: []model.DimensionsRegexp{{
				Regexp:          regexp.MustCompile("(?P<BucketName>[^:]+)$"),
				DimensionsNames: []string{"BucketName"},
			}},
		},
	},
	StaticJobs: []model.StaticJob{{
		Name:       "custom_tesis_metrics",
		Regions:    []string{"us-east-2"},
		Roles:      []model.Role{{RoleArn: "", ExternalID: ""}},
		Namespace:  "CoolApp",
		CustomTags: []model.Tag{},
		Dimensions: []model.Dimension{{
			Name:  "PURCHASES_SERVICE",
			Value: "CoolService",
		}, {Name: "APP_VERSION", Value: "1.0"}},
		Metrics: []*model.MetricConfig{
			{
				Name:                   "KPIs",
				Statistics:             []string{"Average"},
				Period:                 300,
				Length:                 300,
				Delay:                  0,
				NilToZero:              false,
				AddCloudwatchTimestamp: false,
			},
		},
	}},
	CustomNamespaceJobs: []model.CustomNamespaceJob(nil),
}

var expectedConfig4 = model.JobsConfig{
	StsRegion: "us-east-2",
	DiscoveryJobs: []model.DiscoveryJob{{
		Regions:                   []string{"us-east-2"},
		Type:                      "AWS/EC2",
		Roles:                     []model.Role{{RoleArn: "arn:aws:iam::878167871295:role/yace_testing", ExternalID: ""}},
		SearchTags:                []model.SearchTag{{Key: "instance_type", Value: regexp.MustCompile("spot")}},
		CustomTags:                []model.Tag{{Key: "alias", Value: "tesis"}},
		DimensionNameRequirements: []string(nil),
		Metrics: []*model.MetricConfig{
			{
				Name:                   "CPUUtilization",
				Statistics:             []string{"Maximum", "Average"},
				Period:                 300,
				Length:                 300,
				Delay:                  60,
				NilToZero:              true,
				AddCloudwatchTimestamp: false,
			},
		},
		RoundingPeriod:              (*int64)(nil),
		RecentlyActiveOnly:          false,
		ExportedTagsOnMetrics:       []string{"name", "type"},
		IncludeContextOnInfoMetrics: false,
		DimensionsRegexps: []model.DimensionsRegexp{{
			Regexp:          regexp.MustCompile("instance/(?P<InstanceId>[^/]+)"),
			DimensionsNames: []string{"InstanceId"},
		}},
	}},
	StaticJobs: []model.StaticJob{{
		Name:       "custom_tesis_metrics",
		Regions:    []string{"us-east-2"},
		Roles:      []model.Role{{RoleArn: "", ExternalID: ""}},
		Namespace:  "CoolApp",
		CustomTags: []model.Tag{},
		Dimensions: []model.Dimension{
			{Name: "PURCHASES_SERVICE", Value: "CoolService"},
			{Name: "APP_VERSION", Value: "1.0"},
		},
		Metrics: []*model.MetricConfig{
			{
				Name:                   "KPIs",
				Statistics:             []string{"Average"},
				Period:                 300,
				Length:                 300,
				Delay:                  120,
				NilToZero:              true,
				AddCloudwatchTimestamp: false,
			},
		},
	}},
	CustomNamespaceJobs: []model.CustomNamespaceJob(nil),
}

func TestTranslateConfigToYACEConfig(t *testing.T) {
	c := Config{}
	err := yaml.Unmarshal([]byte(configString), &c)
	require.NoError(t, err, "failed to unmarshall config")

	logger, err := logging.New(io.Discard, logging.DefaultOptions)
	require.NoError(t, err)

	yaceConf, fipsEnabled, err := ToYACEConfig(&c, logger)
	require.NoError(t, err, "failed to translate to YACE configuration")

	require.EqualValues(t, expectedConfig, yaceConf)
	require.EqualValues(t, truePtr, fipsEnabled)

	err = yaml.Unmarshal([]byte(configString2), &c)
	require.NoError(t, err, "failed to unmarshall config")

	yaceConf, fipsEnabled2, err := ToYACEConfig(&c, logger)
	require.NoError(t, err, "failed to translate to YACE configuration")

	require.EqualValues(t, expectedConfig, yaceConf)
	require.EqualValues(t, falsePtr, fipsEnabled2)
}

func TestTranslateNilToZeroConfigToYACEConfig(t *testing.T) {
	c := Config{}
	err := yaml.Unmarshal([]byte(configString3), &c)
	require.NoError(t, err, "failed to unmarshal config")

	logger, err := logging.New(io.Discard, logging.DefaultOptions)
	require.NoError(t, err)

	yaceConf, fipsEnabled, err := ToYACEConfig(&c, logger)
	require.NoError(t, err, "failed to translate to YACE configuration")

	require.EqualValues(t, expectedConfig3.DiscoveryJobs, yaceConf.DiscoveryJobs)
	require.EqualValues(t, truePtr, fipsEnabled)
}

func TestTranslateDelayConfigToYACEConfig(t *testing.T) {
	c := Config{}
	err := yaml.Unmarshal([]byte(configString4), &c)
	require.NoError(t, err, "failed to unmarshal config")

	require.Equal(t, time.Minute, c.Discovery.Jobs[0].Delay)
	require.Equal(t, 2*time.Minute, c.Static[0].Delay)

	logger, err := logging.New(io.Discard, logging.DefaultOptions)
	require.NoError(t, err)

	yaceConf, fipsEnabled, err := ToYACEConfig(&c, logger)
	require.NoError(t, err, "failed to translate to YACE configuration")

	require.EqualValues(t, expectedConfig4, yaceConf)
	require.EqualValues(t, truePtr, fipsEnabled)
}

func TestCloudwatchExporterConfigInstanceKey(t *testing.T) {
	cfg1 := &Config{
		STSRegion: "us-east-2",
	}
	cfg2 := &Config{
		STSRegion: "us-east-3",
	}

	cfg1Hash, err := cfg1.InstanceKey("")
	require.NoError(t, err)
	cfg2Hash, err := cfg2.InstanceKey("")
	require.NoError(t, err)

	assert.NotEqual(t, cfg1Hash, cfg2Hash)

	// test that making them equal in values leads to the same instance key
	cfg2.STSRegion = "us-east-2"
	cfg2Hash, err = cfg2.InstanceKey("")
	require.NoError(t, err)

	assert.Equal(t, cfg1Hash, cfg2Hash)
}
