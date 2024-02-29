package cloudwatch

import (
	"crypto/md5"
	"encoding/hex"
	"time"

	"github.com/grafana/agent/internal/static/integrations/cloudwatch_exporter"
	"github.com/grafana/river"
	yaceConf "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/config"
	yaceModel "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/model"
)

// Since we are gathering metrics from CloudWatch and writing them in prometheus during each scrape, the timestamp
// used should be the scrape one
var addCloudwatchTimestamp = false

// Avoid producing absence of values in metrics
var defaultNilToZero = true

var defaults = Arguments{
	Debug:                 false,
	DiscoveryExportedTags: nil,
	FIPSDisabled:          true,
	DecoupledScrape: DecoupledScrapeConfig{
		Enabled:        false,
		ScrapeInterval: 5 * time.Minute,
	},
}

// Arguments are the river based options to configure the embedded CloudWatch exporter.
type Arguments struct {
	STSRegion             string                `river:"sts_region,attr"`
	FIPSDisabled          bool                  `river:"fips_disabled,attr,optional"`
	Debug                 bool                  `river:"debug,attr,optional"`
	DiscoveryExportedTags TagsPerNamespace      `river:"discovery_exported_tags,attr,optional"`
	Discovery             []DiscoveryJob        `river:"discovery,block,optional"`
	Static                []StaticJob           `river:"static,block,optional"`
	DecoupledScrape       DecoupledScrapeConfig `river:"decoupled_scraping,block,optional"`
}

// DecoupledScrapeConfig is the configuration for decoupled scraping feature.
type DecoupledScrapeConfig struct {
	Enabled bool `river:"enabled,attr,optional"`
	// ScrapeInterval defines the decoupled scraping interval. If left empty, a default interval of 5m is used
	ScrapeInterval time.Duration `river:"scrape_interval,attr,optional"`
}

type TagsPerNamespace = cloudwatch_exporter.TagsPerNamespace

// DiscoveryJob configures a discovery job for a given service.
type DiscoveryJob struct {
	Auth                      RegionAndRoles `river:",squash"`
	CustomTags                Tags           `river:"custom_tags,attr,optional"`
	SearchTags                Tags           `river:"search_tags,attr,optional"`
	Type                      string         `river:"type,attr"`
	DimensionNameRequirements []string       `river:"dimension_name_requirements,attr,optional"`
	Metrics                   []Metric       `river:"metric,block"`
	NilToZero                 *bool          `river:"nil_to_zero,attr,optional"`
}

// Tags represents a series of tags configured on an AWS resource. Each tag is a
// key value pair in the dictionary.
type Tags map[string]string

// StaticJob will scrape metrics that match all defined dimensions.
type StaticJob struct {
	Name       string         `river:",label"`
	Auth       RegionAndRoles `river:",squash"`
	CustomTags Tags           `river:"custom_tags,attr,optional"`
	Namespace  string         `river:"namespace,attr"`
	Dimensions Dimensions     `river:"dimensions,attr"`
	Metrics    []Metric       `river:"metric,block"`
	NilToZero  *bool          `river:"nil_to_zero,attr,optional"`
}

// RegionAndRoles exposes for each supported job, the AWS regions and IAM roles in which the agent should perform the
// scrape.
type RegionAndRoles struct {
	Regions []string `river:"regions,attr"`
	Roles   []Role   `river:"role,block,optional"`
}

type Role struct {
	RoleArn    string `river:"role_arn,attr"`
	ExternalID string `river:"external_id,attr,optional"`
}

// Dimensions are the label values used to identify a unique metric stream in CloudWatch.
// Each key value pair in the dictionary corresponds to a label value pair.
type Dimensions map[string]string

type Metric struct {
	Name       string        `river:"name,attr"`
	Statistics []string      `river:"statistics,attr"`
	Period     time.Duration `river:"period,attr"`
	Length     time.Duration `river:"length,attr,optional"`
	NilToZero  *bool         `river:"nil_to_zero,attr,optional"`
}

// SetToDefault implements river.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = defaults
}

// ConvertToYACE converts the river config into YACE config model. Note that the conversion is
// not direct, some values have been opinionated to simplify the config model the agent exposes
// for this integration.
func ConvertToYACE(a Arguments) (yaceConf.ScrapeConf, error) {
	var discoveryJobs []*yaceConf.Job
	for _, job := range a.Discovery {
		discoveryJobs = append(discoveryJobs, toYACEDiscoveryJob(job))
	}
	var staticJobs []*yaceConf.Static
	for _, stat := range a.Static {
		staticJobs = append(staticJobs, toYACEStaticJob(stat))
	}
	conf := yaceConf.ScrapeConf{
		APIVersion: "v1alpha1",
		StsRegion:  a.STSRegion,
		Discovery: yaceConf.Discovery{
			ExportedTagsOnMetrics: yaceModel.ExportedTagsOnMetrics(a.DiscoveryExportedTags),
			Jobs:                  discoveryJobs,
		},
		Static: staticJobs,
	}

	// Run the exporter's config validation. Between other things, it will check that the service for which a discovery
	// job is instantiated, it's supported.
	if err := conf.Validate(); err != nil {
		return yaceConf.ScrapeConf{}, err
	}
	cloudwatch_exporter.PatchYACEDefaults(&conf)

	return conf, nil
}

func (tags Tags) toYACE() []yaceModel.Tag {
	yaceTags := []yaceModel.Tag{}
	for key, value := range tags {
		yaceTags = append(yaceTags, yaceModel.Tag{Key: key, Value: value})
	}
	return yaceTags
}

func toYACERoles(rs []Role) []yaceConf.Role {
	yaceRoles := []yaceConf.Role{}
	// YACE defaults to an empty role, which means the environment configured role is used
	// https://github.com/nerdswords/yet-another-cloudwatch-exporter/blob/30aeceb2324763cdd024a1311045f83a09c1df36/pkg/config/config.go#L111
	if len(rs) == 0 {
		yaceRoles = append(yaceRoles, yaceConf.Role{})
	}
	for _, r := range rs {
		yaceRoles = append(yaceRoles, yaceConf.Role{RoleArn: r.RoleArn, ExternalID: r.ExternalID})
	}
	return yaceRoles
}

func toYACEMetrics(ms []Metric, jobNilToZero *bool) []*yaceConf.Metric {
	yaceMetrics := []*yaceConf.Metric{}
	for _, m := range ms {
		periodSeconds := int64(m.Period.Seconds())
		lengthSeconds := periodSeconds
		// If length is other than zero, that is, is configured, override the default period vaue
		if m.Length != 0 {
			lengthSeconds = int64(m.Length.Seconds())
		}
		nilToZero := m.NilToZero
		if nilToZero == nil {
			nilToZero = jobNilToZero
		}
		yaceMetrics = append(yaceMetrics, &yaceConf.Metric{
			Name:       m.Name,
			Statistics: m.Statistics,

			// Length dictates the size of the window for whom we request metrics, that is, endTime - startTime. Period
			// dictates the size of the buckets in which we aggregate data, inside that window. Since data will be scraped
			// by the agent every so often, dictated by the scrapedInterval, CloudWatch should return a single datapoint
			// for each requested metric. That is if Period >= Length, but is Period > Length, we will be getting not enough
			// data to fill the whole aggregation bucket. Therefore, Period == Length.
			Period: periodSeconds,
			Length: lengthSeconds,

			// Delay moves back the time window for whom CloudWatch is requested data. Since we are already adjusting
			// this with RoundingPeriod (see toYACEDiscoveryJob), we should omit this setting.
			Delay: 0,

			NilToZero:              nilToZero,
			AddCloudwatchTimestamp: &addCloudwatchTimestamp,
		})
	}
	return yaceMetrics
}

func toYACEStaticJob(sj StaticJob) *yaceConf.Static {
	dims := []yaceConf.Dimension{}
	for name, value := range sj.Dimensions {
		dims = append(dims, yaceConf.Dimension{
			Name:  name,
			Value: value,
		})
	}
	nilToZero := sj.NilToZero
	if nilToZero == nil {
		nilToZero = &defaultNilToZero
	}
	return &yaceConf.Static{
		Name:       sj.Name,
		Regions:    sj.Auth.Regions,
		Roles:      toYACERoles(sj.Auth.Roles),
		Namespace:  sj.Namespace,
		CustomTags: sj.CustomTags.toYACE(),
		Dimensions: dims,
		Metrics:    toYACEMetrics(sj.Metrics, nilToZero),
	}
}

func toYACEDiscoveryJob(rj DiscoveryJob) *yaceConf.Job {
	nilToZero := rj.NilToZero
	if nilToZero == nil {
		nilToZero = &defaultNilToZero
	}
	job := &yaceConf.Job{
		Regions:                   rj.Auth.Regions,
		Roles:                     toYACERoles(rj.Auth.Roles),
		Type:                      rj.Type,
		CustomTags:                rj.CustomTags.toYACE(),
		SearchTags:                rj.SearchTags.toYACE(),
		DimensionNameRequirements: rj.DimensionNameRequirements,
		// By setting RoundingPeriod to nil, the exporter will align the start and end times for retrieving CloudWatch
		// metrics, with the smallest period in the retrieved batch.
		RoundingPeriod: nil,
		JobLevelMetricFields: yaceConf.JobLevelMetricFields{
			// Set to zero job-wide scraping time settings. This should be configured at the metric level to make the data
			// being fetched more explicit.
			Period:                 0,
			Length:                 0,
			Delay:                  0,
			NilToZero:              nilToZero,
			AddCloudwatchTimestamp: &addCloudwatchTimestamp,
		},
		Metrics: toYACEMetrics(rj.Metrics, nilToZero),
	}
	return job
}

// getHash calculates the MD5 hash of the river representation of the config.
func getHash(a Arguments) string {
	bytes, err := river.Marshal(a)
	if err != nil {
		return "<unknown>"
	}
	hash := md5.Sum(bytes)
	return hex.EncodeToString(hash[:])
}
