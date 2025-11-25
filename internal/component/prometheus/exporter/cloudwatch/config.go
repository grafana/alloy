package cloudwatch

import (
	"crypto/md5"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/go-kit/log"
	yaceConf "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/config"
	yaceModel "github.com/prometheus-community/yet-another-cloudwatch-exporter/pkg/model"

	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/static/integrations/cloudwatch_exporter"
	"github.com/grafana/alloy/syntax"
)

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
	UseAWSSDKVersion2: false,
}

// Arguments are the Alloy based options to configure the embedded CloudWatch exporter.
type Arguments struct {
	STSRegion             string                `alloy:"sts_region,attr"`
	FIPSDisabled          bool                  `alloy:"fips_disabled,attr,optional"`
	Debug                 bool                  `alloy:"debug,attr,optional"`
	DiscoveryExportedTags TagsPerNamespace      `alloy:"discovery_exported_tags,attr,optional"`
	Discovery             []DiscoveryJob        `alloy:"discovery,block,optional"`
	Static                []StaticJob           `alloy:"static,block,optional"`
	CustomNamespace       []CustomNamespaceJob  `alloy:"custom_namespace,block,optional"`
	DecoupledScrape       DecoupledScrapeConfig `alloy:"decoupled_scraping,block,optional"`
	UseAWSSDKVersion2     bool                  `alloy:"aws_sdk_version_v2,attr,optional"`
}

// DecoupledScrapeConfig is the configuration for decoupled scraping feature.
type DecoupledScrapeConfig struct {
	Enabled bool `alloy:"enabled,attr,optional"`
	// ScrapeInterval defines the decoupled scraping interval. If left empty, a default interval of 5m is used
	ScrapeInterval time.Duration `alloy:"scrape_interval,attr,optional"`
}

type TagsPerNamespace = cloudwatch_exporter.TagsPerNamespace

// DiscoveryJob configures a discovery job for a given service.
type DiscoveryJob struct {
	Auth                      RegionAndRoles `alloy:",squash"`
	CustomTags                Tags           `alloy:"custom_tags,attr,optional"`
	SearchTags                Tags           `alloy:"search_tags,attr,optional"`
	Type                      string         `alloy:"type,attr"`
	DimensionNameRequirements []string       `alloy:"dimension_name_requirements,attr,optional"`
	RecentlyActiveOnly        bool           `alloy:"recently_active_only,attr,optional"`
	Metrics                   []Metric       `alloy:"metric,block"`
	Delay                     time.Duration  `alloy:"delay,attr,optional"`
	//TODO: Remove NilToZero, because it is deprecated upstream.
	NilToZero *bool `alloy:"nil_to_zero,attr,optional"`
}

// Tags represents a series of tags configured on an AWS resource. Each tag is a
// key value pair in the dictionary.
type Tags map[string]string

// StaticJob will scrape metrics that match all defined dimensions.
type StaticJob struct {
	Name       string         `alloy:",label"`
	Auth       RegionAndRoles `alloy:",squash"`
	CustomTags Tags           `alloy:"custom_tags,attr,optional"`
	Namespace  string         `alloy:"namespace,attr"`
	Dimensions Dimensions     `alloy:"dimensions,attr"`
	Metrics    []Metric       `alloy:"metric,block"`
	Delay      time.Duration  `alloy:"delay,attr,optional"`
	//TODO: Remove NilToZero, because it is deprecated upstream.
	NilToZero *bool `alloy:"nil_to_zero,attr,optional"`
}

type CustomNamespaceJob struct {
	Auth                      RegionAndRoles `alloy:",squash"`
	Name                      string         `alloy:",label"`
	CustomTags                Tags           `alloy:"custom_tags,attr,optional"`
	DimensionNameRequirements []string       `alloy:"dimension_name_requirements,attr,optional"`
	Namespace                 string         `alloy:"namespace,attr"`
	RecentlyActiveOnly        bool           `alloy:"recently_active_only,attr,optional"`
	Metrics                   []Metric       `alloy:"metric,block"`
	Delay                     time.Duration  `alloy:"delay,attr,optional"`
	//TODO: Remove NilToZero, because it is deprecated upstream.
	NilToZero *bool `alloy:"nil_to_zero,attr,optional"`
}

// RegionAndRoles exposes for each supported job, the AWS regions and IAM roles
// in which Alloy should perform the scrape.
type RegionAndRoles struct {
	Regions []string `alloy:"regions,attr"`
	Roles   []Role   `alloy:"role,block,optional"`
}

type Role struct {
	RoleArn    string `alloy:"role_arn,attr"`
	ExternalID string `alloy:"external_id,attr,optional"`
}

// Dimensions are the label values used to identify a unique metric stream in CloudWatch.
// Each key value pair in the dictionary corresponds to a label value pair.
type Dimensions map[string]string

type Metric struct {
	Name                   string        `alloy:"name,attr"`
	Statistics             []string      `alloy:"statistics,attr"`
	Period                 time.Duration `alloy:"period,attr"`
	Length                 time.Duration `alloy:"length,attr,optional"`
	NilToZero              *bool         `alloy:"nil_to_zero,attr,optional"`
	AddCloudwatchTimestamp *bool         `alloy:"add_cloudwatch_timestamp,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = defaults
}

// ConvertToYACE converts the Alloy config into YACE config model. Note that
// the conversion is not direct, some values have been opinionated to simplify
// the config model Alloy exposes for this integration.
func ConvertToYACE(a Arguments, logger log.Logger) (yaceModel.JobsConfig, error) {
	// Once the support for deprecated aliases is dropped, this function (convertAliasesToNamespaces) can be removed.
	convertAliasesToNamespaces(&a, logger)

	return convertToYACE(a)
}

// convertAliasesToNamespaces converts the deprecated service aliases to their corresponding namespaces.
// This function is added for the backward compatibility of the deprecated service aliases. This compatibility
// may be removed in the future.
func convertAliasesToNamespaces(a *Arguments, logger log.Logger) {
	for i, job := range a.Discovery {
		if job.Type != "" {
			if svc := yaceConf.SupportedServices.GetService(job.Type); svc == nil {
				if namespace := getServiceByAlias(job.Type); namespace != "" {
					level.Warn(logger).Log("msg", "service alias is deprecated, use the namespace instead", "alias", job.Type, "namespace", namespace)
					a.Discovery[i].Type = namespace
				}
			}
		}
	}

	for i, job := range a.Static {
		if svc := yaceConf.SupportedServices.GetService(job.Namespace); svc == nil {
			if namespace := getServiceByAlias(job.Namespace); namespace != "" {
				level.Warn(logger).Log("msg", "service alias is deprecated, use the namespace instead", "alias", job.Namespace, "namespace", namespace)
				a.Static[i].Namespace = namespace
			}
		}
	}

	if len(a.DiscoveryExportedTags) > 0 {
		var newDiscoveryExportedTags TagsPerNamespace = make(map[string][]string, len(a.DiscoveryExportedTags))

		for namespace, tags := range a.DiscoveryExportedTags {
			if svc := yaceConf.SupportedServices.GetService(namespace); svc == nil {
				if ns := getServiceByAlias(namespace); ns != "" {
					level.Warn(logger).Log("msg", "service alias is deprecated, use the namespace instead", "alias", namespace, "namespace", ns)
					newDiscoveryExportedTags[ns] = tags
				}
			} else {
				newDiscoveryExportedTags[svc.Namespace] = tags
			}
		}

		a.DiscoveryExportedTags = newDiscoveryExportedTags
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

func convertToYACE(a Arguments) (yaceModel.JobsConfig, error) {
	var discoveryJobs []*yaceConf.Job
	for _, job := range a.Discovery {
		discoveryJobs = append(discoveryJobs, toYACEDiscoveryJob(job))
	}
	var staticJobs []*yaceConf.Static
	for _, stat := range a.Static {
		staticJobs = append(staticJobs, toYACEStaticJob(stat))
	}
	var customNamespaceJobs []*yaceConf.CustomNamespace
	for _, cn := range a.CustomNamespace {
		customNamespaceJobs = append(customNamespaceJobs, toYACECustomNamespaceJob(cn))
	}
	conf := yaceConf.ScrapeConf{
		APIVersion: "v1alpha1",
		StsRegion:  a.STSRegion,
		Discovery: yaceConf.Discovery{
			ExportedTagsOnMetrics: yaceConf.ExportedTagsOnMetrics(a.DiscoveryExportedTags),
			Jobs:                  discoveryJobs,
		},
		Static:          staticJobs,
		CustomNamespace: customNamespaceJobs,
	}

	// Run the exporter's config validation. Between other things, it will check that the service for which a discovery
	// job is instantiated, it's supported.
	modelConf, err := conf.Validate(slog.New(slog.DiscardHandler))
	if err != nil {
		return yaceModel.JobsConfig{}, err
	}

	return modelConf, nil
}

func (tags Tags) toYACE() []yaceConf.Tag {
	yaceTags := []yaceConf.Tag{}
	for key, value := range tags {
		yaceTags = append(yaceTags, yaceConf.Tag{Key: key, Value: value})
	}
	return yaceTags
}

func toYACERoles(rs []Role) []yaceConf.Role {
	yaceRoles := []yaceConf.Role{}
	// YACE defaults to an empty role, which means the environment configured role is used
	// https://github.com/prometheus-community/yet-another-cloudwatch-exporter/blob/30aeceb2324763cdd024a1311045f83a09c1df36/pkg/config/config.go#L111
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
		// If length is other than zero, that is, it is configured, override the default period value
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
			// by Alloy every so often, dictated by the scrapedInterval, CloudWatch should return a single datapoint
			// for each requested metric. That is if Period >= Length, but is Period > Length, we will be getting not enough
			// data to fill the whole aggregation bucket. Therefore, Period == Length.
			Period: periodSeconds,
			Length: lengthSeconds,

			NilToZero:              nilToZero,
			AddCloudwatchTimestamp: m.AddCloudwatchTimestamp,
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
		RecentlyActiveOnly: rj.RecentlyActiveOnly,
		JobLevelMetricFields: yaceConf.JobLevelMetricFields{
			Delay: int64(rj.Delay.Seconds()),
		},
		Metrics: toYACEMetrics(rj.Metrics, nilToZero),
	}
	return job
}

func toYACECustomNamespaceJob(cn CustomNamespaceJob) *yaceConf.CustomNamespace {
	nilToZero := cn.NilToZero
	if nilToZero == nil {
		nilToZero = &defaultNilToZero
	}
	return &yaceConf.CustomNamespace{
		Name:                      cn.Name,
		Namespace:                 cn.Namespace,
		Regions:                   cn.Auth.Regions,
		Roles:                     toYACERoles(cn.Auth.Roles),
		CustomTags:                cn.CustomTags.toYACE(),
		DimensionNameRequirements: cn.DimensionNameRequirements,
		// By setting RoundingPeriod to nil, the exporter will align the start and end times for retrieving CloudWatch
		// metrics, with the smallest period in the retrieved batch.
		RoundingPeriod: nil,
		RecentlyActiveOnly: cn.RecentlyActiveOnly,
		JobLevelMetricFields: yaceConf.JobLevelMetricFields{
			Delay: int64(cn.Delay.Seconds()),
		},
		Metrics: toYACEMetrics(cn.Metrics, nilToZero),
	}
}

// getHash calculates the MD5 hash of the Alloy representation of the config.
func getHash(a Arguments) string {
	bytes, err := syntax.Marshal(a)
	if err != nil {
		return "<unknown>"
	}
	hash := md5.Sum(bytes)
	return hex.EncodeToString(hash[:])
}
