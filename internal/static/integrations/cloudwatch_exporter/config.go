package cloudwatch_exporter

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/runtime/logging/level"

	"github.com/go-kit/log"
	yaceConf "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/config"
	"github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/logging"
	yaceModel "github.com/nerdswords/yet-another-cloudwatch-exporter/pkg/model"
	"gopkg.in/yaml.v2"

	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
)

const (
	metricsPerQuery                  = 500
	cloudWatchConcurrency            = 5
	tagConcurrency                   = 5
	labelsSnakeCase                  = false
	defaultDecoupledScrapingInterval = time.Minute * 5
)

// Since we are gathering metrics from CloudWatch and writing them in prometheus during each scrape, the timestamp
// used should be the scrape one
var addCloudwatchTimestamp = false

// Avoid producing absence of values in metrics
var defaultNilToZero = true

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("cloudwatch"))
}

// Config is the configuration for the CloudWatch metrics integration
type Config struct {
	STSRegion         string                `yaml:"sts_region"`
	FIPSDisabled      bool                  `yaml:"fips_disabled"`
	Discovery         DiscoveryConfig       `yaml:"discovery"`
	Static            []StaticJob           `yaml:"static"`
	Debug             bool                  `yaml:"debug"`
	DecoupledScrape   DecoupledScrapeConfig `yaml:"decoupled_scraping"`
	UseAWSSDKVersion2 bool                  `yaml:"aws_sdk_version_v2"`
}

// DecoupledScrapeConfig is the configuration for decoupled scraping feature.
type DecoupledScrapeConfig struct {
	Enabled bool `yaml:"enabled"`
	// ScrapeInterval defines the decoupled scraping interval. If left empty, a default interval of 5m is used
	ScrapeInterval *time.Duration `yaml:"scrape_interval,omitempty"`
}

// DiscoveryConfig configures scraping jobs that will auto-discover metrics dimensions for a given service.
type DiscoveryConfig struct {
	ExportedTags TagsPerNamespace `yaml:"exported_tags"`
	Jobs         []*DiscoveryJob  `yaml:"jobs"`
}

// TagsPerNamespace represents for each namespace, a list of tags that will be exported as labels in each metric.
type TagsPerNamespace map[string][]string

// DiscoveryJob configures a discovery job for a given service.
type DiscoveryJob struct {
	InlineRegionAndRoles      `yaml:",inline"`
	InlineCustomTags          `yaml:",inline"`
	SearchTags                []Tag    `yaml:"search_tags"`
	Type                      string   `yaml:"type"`
	DimensionNameRequirements []string `yaml:"dimension_name_requirements"`
	Metrics                   []Metric `yaml:"metrics"`
	NilToZero                 *bool    `yaml:"nil_to_zero,omitempty"`
}

// StaticJob will scrape metrics that match all defined dimensions.
type StaticJob struct {
	InlineRegionAndRoles `yaml:",inline"`
	InlineCustomTags     `yaml:",inline"`
	Name                 string      `yaml:"name"`
	Namespace            string      `yaml:"namespace"`
	Dimensions           []Dimension `yaml:"dimensions"`
	Metrics              []Metric    `yaml:"metrics"`
	NilToZero            *bool       `yaml:"nil_to_zero,omitempty"`
}

// InlineRegionAndRoles exposes for each supported job, the AWS regions and IAM roles in which the agent should perform the
// scrape.
type InlineRegionAndRoles struct {
	Regions []string `yaml:"regions"`
	Roles   []Role   `yaml:"roles"`
}

type InlineCustomTags struct {
	CustomTags []Tag `yaml:"custom_tags"`
}

type Role struct {
	RoleArn    string `yaml:"role_arn"`
	ExternalID string `yaml:"external_id"`
}

type Dimension struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

type Tag struct {
	Key   string `yaml:"key"`
	Value string `yaml:"value"`
}

type Metric struct {
	Name       string        `yaml:"name"`
	Statistics []string      `yaml:"statistics"`
	Period     time.Duration `yaml:"period"`
	Length     time.Duration `yaml:"length"`
	NilToZero  *bool         `yaml:"nil_to_zero,omitempty"`
}

// Name returns the name of the integration this config is for.
func (c *Config) Name() string {
	return "cloudwatch_exporter"
}

func (c *Config) InstanceKey(_ string) (string, error) {
	return getHash(c)
}

// NewIntegration creates a new integration from the config.
func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	exporterConfig, fipsEnabled, err := ToYACEConfig(c, l)
	if err != nil {
		return nil, fmt.Errorf("invalid cloudwatch exporter configuration: %w", err)
	}
	if c.DecoupledScrape.Enabled {
		scrapeInterval := defaultDecoupledScrapingInterval
		if v := c.DecoupledScrape.ScrapeInterval; v != nil {
			scrapeInterval = *v
		}
		return NewDecoupledCloudwatchExporter(c.Name(), l, exporterConfig, scrapeInterval, fipsEnabled, c.Debug, c.UseAWSSDKVersion2)
	}

	return NewCloudwatchExporter(c.Name(), l, exporterConfig, fipsEnabled, c.Debug, c.UseAWSSDKVersion2)
}

// getHash calculates the MD5 hash of the yaml representation of the config
func getHash(c *Config) (string, error) {
	bytes, err := yaml.Marshal(c)
	if err != nil {
		return "", err
	}
	hash := md5.Sum(bytes)
	return hex.EncodeToString(hash[:]), nil
}

// ToYACEConfig converts a Config into YACE's config model. Note that the conversion is not direct, some values
// have been opinionated to simplify the config model the agent exposes for this integration.
// The returned boolean is whether or not AWS FIPS endpoints will be enabled.
func ToYACEConfig(c *Config, logger log.Logger) (yaceModel.JobsConfig, bool, error) {
	// Once the support for deprecated aliases is dropped, this function (convertAliasesToNamespaces) can be removed.
	convertAliasesToNamespaces(c, logger)
	return toYACEConfig(c)
}

// convertAliasesToNamespaces converts the deprecated service aliases to their corresponding namespaces.
// This function is added for the backward compatibility of the deprecated service aliases. This compatibility
// may be removed in the future.
func convertAliasesToNamespaces(c *Config, logger log.Logger) {
	for i, job := range c.Discovery.Jobs {
		if job.Type != "" {
			if svc := yaceConf.SupportedServices.GetService(job.Type); svc == nil {
				if namespace := getServiceByAlias(job.Type); namespace != "" {
					level.Warn(logger).Log("msg", "service alias is deprecated, use the namespace instead", "alias", job.Type, "namespace", namespace)
					c.Discovery.Jobs[i].Type = namespace
				}
			}
		}
	}

	for i, job := range c.Static {
		if svc := yaceConf.SupportedServices.GetService(job.Namespace); svc == nil {
			if namespace := getServiceByAlias(job.Namespace); namespace != "" {
				level.Warn(logger).Log("msg", "service alias is deprecated, use the namespace instead", "alias", job.Namespace, "namespace", namespace)
				c.Static[i].Namespace = namespace
			}
		}
	}
}

// getServiceByAlias returns the namespace for a given service alias.
func getServiceByAlias(alias string) string {
	for _, supportedServices := range yaceConf.SupportedServices {
		if supportedServices.Alias == alias {
			return supportedServices.Namespace
		}
	}

	return ""
}

func toYACEConfig(c *Config) (yaceModel.JobsConfig, bool, error) {
	discoveryJobs := []*yaceConf.Job{}
	for _, job := range c.Discovery.Jobs {
		discoveryJobs = append(discoveryJobs, toYACEDiscoveryJob(job))
	}
	staticJobs := []*yaceConf.Static{}
	for _, stat := range c.Static {
		staticJobs = append(staticJobs, toYACEStaticJob(stat))
	}
	conf := yaceConf.ScrapeConf{
		APIVersion: "v1alpha1",
		StsRegion:  c.STSRegion,
		Discovery: yaceConf.Discovery{
			ExportedTagsOnMetrics: yaceConf.ExportedTagsOnMetrics(c.Discovery.ExportedTags),
			Jobs:                  discoveryJobs,
		},
		Static: staticJobs,
	}

	// yaceSess expects a default value of True
	fipsEnabled := !c.FIPSDisabled

	// Run the exporter's config validation. Between other things, it will check that the service for which a discovery
	// job is instantiated, it's supported.
	modelConf, err := conf.Validate(logging.NewNopLogger())
	if err != nil {
		return yaceModel.JobsConfig{}, fipsEnabled, err
	}
	PatchYACEDefaults(&modelConf)

	return modelConf, fipsEnabled, nil
}

// PatchYACEDefaults overrides some default values YACE applies after validation.
func PatchYACEDefaults(yc *yaceModel.JobsConfig) {
	// YACE doesn't allow during validation a zero-delay in each metrics scrape. Override this behaviour since it's taken
	// into account by the rounding period.
	// https://github.com/nerdswords/yet-another-cloudwatch-exporter/blob/7e5949124bb5f26353eeff298724a5897de2a2a4/pkg/config/config.go#L320
	for _, job := range yc.DiscoveryJobs {
		for _, metric := range job.Metrics {
			metric.Delay = 0
		}
	}
	for _, staticConf := range yc.StaticJobs {
		for _, metric := range staticConf.Metrics {
			metric.Delay = 0
		}
	}
}

func toYACEStaticJob(job StaticJob) *yaceConf.Static {
	nilToZero := job.NilToZero
	if nilToZero == nil {
		nilToZero = &defaultNilToZero
	}
	return &yaceConf.Static{
		Name:       job.Name,
		Regions:    job.Regions,
		Roles:      toYACERoles(job.Roles),
		Namespace:  job.Namespace,
		CustomTags: toYACETags(job.CustomTags),
		Dimensions: toYACEDimensions(job.Dimensions),
		Metrics:    toYACEMetrics(job.Metrics, nilToZero),
	}
}

func toYACEDimensions(dim []Dimension) []yaceConf.Dimension {
	yaceDims := []yaceConf.Dimension{}
	for _, d := range dim {
		yaceDims = append(yaceDims, yaceConf.Dimension{
			Name:  d.Name,
			Value: d.Value,
		})
	}
	return yaceDims
}

func toYACEDiscoveryJob(job *DiscoveryJob) *yaceConf.Job {
	roles := toYACERoles(job.Roles)
	nilToZero := job.NilToZero
	if nilToZero == nil {
		nilToZero = &defaultNilToZero
	}
	yaceJob := yaceConf.Job{
		Regions:                   job.Regions,
		Roles:                     roles,
		CustomTags:                toYACETags(job.CustomTags),
		Type:                      job.Type,
		Metrics:                   toYACEMetrics(job.Metrics, nilToZero),
		SearchTags:                toYACETags(job.SearchTags),
		DimensionNameRequirements: job.DimensionNameRequirements,

		// By setting RoundingPeriod to nil, the exporter will align the start and end times for retrieving CloudWatch
		// metrics, with the smallest period in the retrieved batch.
		RoundingPeriod: nil,
	}
	return &yaceJob
}

func toYACEMetrics(metrics []Metric, jobNilToZero *bool) []*yaceConf.Metric {
	yaceMetrics := []*yaceConf.Metric{}
	for _, metric := range metrics {
		periodSeconds := int64(metric.Period.Seconds())
		lengthSeconds := periodSeconds
		// If `length` is configured, override default
		if metric.Length != 0 {
			lengthSeconds = int64(metric.Length.Seconds())
		}
		nilToZero := metric.NilToZero
		if nilToZero == nil {
			nilToZero = jobNilToZero
		}

		yaceMetrics = append(yaceMetrics, &yaceConf.Metric{
			Name:       metric.Name,
			Statistics: metric.Statistics,

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

func toYACERoles(roles []Role) []yaceConf.Role {
	yaceRoles := []yaceConf.Role{}
	// YACE defaults to an empty role, which means the environment configured role is used
	// https://github.com/nerdswords/yet-another-cloudwatch-exporter/blob/30aeceb2324763cdd024a1311045f83a09c1df36/pkg/config/config.go#L111
	if len(roles) == 0 {
		yaceRoles = append(yaceRoles, yaceConf.Role{})
	}
	for _, role := range roles {
		yaceRoles = append(yaceRoles, yaceConf.Role{
			RoleArn:    role.RoleArn,
			ExternalID: role.ExternalID,
		})
	}
	return yaceRoles
}

func toYACETags(tags []Tag) []yaceConf.Tag {
	outTags := []yaceConf.Tag{}
	for _, t := range tags {
		outTags = append(outTags, yaceConf.Tag{
			Key:   t.Key,
			Value: t.Value,
		})
	}
	return outTags
}
