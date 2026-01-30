package azure_exporter

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	duration "github.com/channelmeter/iso8601duration"
	"github.com/go-kit/log"
	azure_config "github.com/webdevops/azure-metrics-exporter/config"
	"github.com/webdevops/azure-metrics-exporter/metrics"
	"github.com/webdevops/go-common/azuresdk/cloudconfig"
	"gopkg.in/yaml.v3"

	"github.com/grafana/alloy/internal/static/integrations"
	integrations_v2 "github.com/grafana/alloy/internal/static/integrations/v2"
	"github.com/grafana/alloy/internal/static/integrations/v2/metricsutils"
	"github.com/grafana/alloy/internal/util/zapadapter"
)

func init() {
	integrations.RegisterIntegration(&Config{})
	integrations_v2.RegisterLegacy(&Config{}, integrations_v2.TypeMultiplex, metricsutils.NewNamedShim("azure"))
}

// DefaultConfig holds the default settings for the azure_exporter integration.
var DefaultConfig = Config{
	Timespan:              "PT5M",
	Interval:              "PT1M",
	MetricNameTemplate:    "azure_{type}_{metric}_{aggregation}_{unit}",
	MetricHelpTemplate:    "Azure metric {metric} for {type} with aggregation {aggregation} as {unit}",
	IncludedResourceTags:  []string{"owner"},
	AzureCloudEnvironment: "azurecloud",
	// Dimensions do not always apply to all metrics for a service, which requires you to configure multiple exporters
	//  to fully monitor a service which is tedious. Turning off validation eliminates this complexity. The underlying
	//  sdk will only give back the dimensions which are valid for particular metrics.
	ValidateDimensions: false,
}

type Config struct {
	Subscriptions            []string `yaml:"subscriptions"`               // Required
	ResourceGraphQueryFilter string   `yaml:"resource_graph_query_filter"` // Optional
	Regions                  []string `yaml:"regions"`                     // Optional

	// Valid values can be derived from https://learn.microsoft.com/en-us/azure/azure-monitor/essentials/metrics-supported
	// Required: Root level names ex. Microsoft.DataShare/accounts
	ResourceType string `yaml:"resource_type"`
	// Required: Metric in the table for a ResourceType
	Metrics []string `yaml:"metrics"`
	// Optional: If not provided Aggregation Type is used for the Metric
	// Valid values are minimum, maximum, average, total, and count
	MetricAggregations []string `yaml:"metric_aggregations"`

	// All fields below are optional
	// Must be an ISO8601 Duration - defaults to PT5M if not specified
	Timespan string `yaml:"timespan"`
	// Must be an ISO8601 Duration - defaults to PT1M if not specified
	Interval string `yaml:"interval"`

	IncludedDimensions   []string `yaml:"included_dimensions"`
	IncludedResourceTags []string `yaml:"included_resource_tags"`

	// MetricNamespace is used for ResourceTypes which have multiple levels of metrics
	// As an example the ResourceType Microsoft.Storage/storageAccounts has metrics for
	//  Microsoft.Storage/storageAccounts (generic metrics which apply to all storage accounts)
	//	Microsoft.Storage/storageAccounts/blobServices (generic metrics + metrics which only apply to blob stores)
	// 	Microsoft.Storage/storageAccounts/fileServices (generic metrics + metrics which only apply to file stores)
	//	Microsoft.Storage/storageAccounts/queueServices (generic metrics + metrics which only apply to queue stores)
	//	Microsoft.Storage/storageAccounts/tableServices (generic metrics + metrics which only apply to table stores)
	// If you want blob store metrics you will need to set
	//  ResourceType = Microsoft.Storage/storageAccounts
	//	MetricNamespace = Microsoft.Storage/storageAccounts/blobServices
	MetricNamespace string `yaml:"metric_namespace"`

	MetricNameTemplate string `yaml:"metric_name_template"`
	MetricHelpTemplate string `yaml:"metric_help_template"`
	ValidateDimensions bool   `yaml:"validate_dimensions"`

	AzureCloudEnvironment string `yaml:"azure_cloud_environment"`
}

// UnmarshalYAML implements yaml.Unmarshaler for Config.
func (c *Config) UnmarshalYAML(unmarshal func(any) error) error {
	*c = DefaultConfig

	type plain Config
	return unmarshal((*plain)(c))
}

func (c *Config) InstanceKey(_ string) (string, error) {
	// Running the integration in v2 as a TypeMultiplex requires the instance key is unique per instance
	// There's no good unique identifier in the config, so we use a hash instead
	return getHash(c)
}

func (c *Config) NewIntegration(l log.Logger) (integrations.Integration, error) {
	concurrencyConfig := azure_config.Opts{
		// Necessary to match OSS definition
		Prober: struct {
			// Limits the number of subscriptions which can concurrently be sending metric requests - value taken from OSS exporter
			ConcurrencySubscription int `long:"concurrency.subscription"          env:"CONCURRENCY_SUBSCRIPTION"           description:"Concurrent subscription fetches"                                  default:"5"`
			// Limits the number of concurrent metric requests for a single subscription  - value taken from OSS exporter
			ConcurrencySubscriptionResource int  `long:"concurrency.subscription.resource" env:"CONCURRENCY_SUBSCRIPTION_RESOURCE"  description:"Concurrent requests per resource (inside subscription requests)"  default:"10"`
			Cache                           bool `long:"enable-caching"                    env:"ENABLE_CACHING"                     description:"Enable internal caching"`
		}{5, 10, false},
	}

	return Exporter{
		cfg:               *c,
		logger:            zapadapter.New(l).Sugar(),
		ConcurrencyConfig: concurrencyConfig,
	}, nil
}

func (c *Config) Validate() error {
	var configErrors []string

	if len(c.Subscriptions) == 0 {
		configErrors = append(configErrors, "subscriptions cannot be empty")
	}

	if c.ResourceType == "" {
		configErrors = append(configErrors, "resource_type cannot be empty")
	}

	if len(c.Metrics) == 0 {
		configErrors = append(configErrors, "metrics cannot be empty")
	}

	if len(c.Regions) > 0 && c.ResourceGraphQueryFilter != "" {
		configErrors = append(configErrors, "regions and resource_graph_query_filter cannot be used together. If you want to target specific resources add a region filter to the resource_graph_query_filter. Otherwise, remove your resource_graph_query_filter to get metrics without further filtering.")
	}

	validAggregations := []string{"minimum", "maximum", "average", "total", "count"}

	for _, aggregation := range c.MetricAggregations {
		lowerCaseAggregation := strings.ToLower(aggregation)
		found := false
		for _, validAggregation := range validAggregations {
			if validAggregation == lowerCaseAggregation {
				found = true
				break
			}
		}
		if !found {
			configErrors = append(configErrors, fmt.Sprintf("%s is an invalid value for metric_aggregations. Valid options are %s", aggregation, strings.Join(validAggregations, ",")))
		}
	}

	if _, err := cloudconfig.NewCloudConfig(c.AzureCloudEnvironment); err != nil {
		configErrors = append(configErrors, fmt.Errorf("failed to create an azure cloud configuration from azure cloud environment %s, %v", c.AzureCloudEnvironment, err).Error())
	}

	timespan, tErr := duration.FromString(c.Timespan)
	if tErr != nil {
		configErrors = append(configErrors, fmt.Sprintf("timespan %s is not a valid ISO8601 duration, %v", c.Timespan, tErr))
	}

	interval, iErr := duration.FromString(c.Interval)
	if iErr != nil {
		configErrors = append(configErrors, fmt.Sprintf("timespan %s is not a valid ISO8601 duration, %v", c.Timespan, iErr))
	}

	if tErr == nil && iErr == nil && timespan.ToDuration() < interval.ToDuration() {
		configErrors = append(configErrors, fmt.Sprintf("timespan %s must be greater than or equal to interval %s", c.Timespan, c.Interval))
	}

	if len(configErrors) != 0 {
		return errors.New(strings.Join(configErrors, ","))
	}

	return nil
}

func (c *Config) Name() string {
	return "azure_exporter"
}

func (c *Config) ToScrapeSettings() (*metrics.RequestMetricSettings, error) {
	settings := metrics.RequestMetricSettings{
		Subscriptions:      c.Subscriptions,
		Metrics:            c.Metrics,
		ResourceType:       c.ResourceType,
		Aggregations:       c.MetricAggregations,
		Filter:             c.ResourceGraphQueryFilter,
		MetricNamespace:    c.MetricNamespace,
		MetricTemplate:     c.MetricNameTemplate,
		HelpTemplate:       c.MetricHelpTemplate,
		ValidateDimensions: c.ValidateDimensions,
		Regions:            c.Regions,

		// Timespan controls query start and end time
		Timespan: c.Timespan,
		// Interval controls the resolution of the data returned
		Interval: to.Ptr(c.Interval),

		// Unused settings just here to capture they are intentionally not set
		Name:  "",
		Cache: nil,
	}

	// Dimensions can only be retrieved via an obscure manner of including a "metric filter" on the query
	// This isn't documented in the Azure API only the exporter: https://github.com/webdevops/azure-metrics-exporter#virtualnetworkgateway-connections-dimension-support
	if len(c.IncludedDimensions) > 0 {
		builder := strings.Builder{}
		for index, dimension := range c.IncludedDimensions {
			if _, err := builder.WriteString(dimension); err != nil {
				return nil, err
			}
			if _, err := builder.WriteString(" eq '*'"); err != nil {
				return nil, err
			}
			// Not the last dimension add an `and`
			if index != (len(c.IncludedDimensions) - 1) {
				if _, err := builder.WriteString(" and "); err != nil {
					return nil, err
				}
			}
		}
		settings.MetricFilter = builder.String()

		// The metric filter introduces a secondary complexity where data is limited by a "top" parameter (default 10)
		// We don't get any knowledge if the result is cut off and there's no support for paging, so we set the value as
		// high as possible to hopefully prevent issues. The API doesn't have a documented limit but any higher values
		// cause an OOM from the API
		settings.MetricTop = to.Ptr[int32](100_000_000)
		settings.MetricOrderBy = "" // Order is only relevant if top won't return all the results our high value should prevent this
	}

	// Using regions is at subscription scope which adds a default filter that respects top
	if len(settings.Regions) > 0 && settings.MetricTop == nil {
		settings.MetricTop = to.Ptr[int32](100_000_000)
	}

	return &settings, nil
}

// MergeConfigWithQueryParams will map values from params which where the key
// matches a yaml tag of the Config struct
func MergeConfigWithQueryParams(cfg Config, params url.Values) (Config, error) {
	if subscriptions, exists := params["subscriptions"]; exists {
		cfg.Subscriptions = subscriptions
	}

	graphQueryFilters := params.Get("resource_graph_query_filter")
	if len(graphQueryFilters) != 0 {
		cfg.ResourceGraphQueryFilter = graphQueryFilters
	}

	resourceType := params.Get("resource_type")
	if len(resourceType) != 0 {
		cfg.ResourceType = resourceType
	}

	if metricsToScrape, exists := params["metrics"]; exists {
		cfg.Metrics = metricsToScrape
	}

	if aggregations, exists := params["metric_aggregations"]; exists {
		cfg.MetricAggregations = aggregations
	}

	timespan := params.Get("timespan")
	if len(timespan) != 0 {
		cfg.Timespan = timespan
	}

	interval := params.Get("interval")
	if len(interval) != 0 {
		cfg.Interval = interval
	}

	if dimensions, exists := params["included_dimensions"]; exists {
		cfg.IncludedDimensions = dimensions
	}

	if tags, exists := params["included_resource_tags"]; exists {
		cfg.IncludedResourceTags = tags
	}

	namespace := params.Get("metric_namespace")
	if len(namespace) != 0 {
		cfg.MetricNamespace = namespace
	}

	nameTemplate := params.Get("metric_name_template")
	if len(nameTemplate) != 0 {
		cfg.MetricNameTemplate = nameTemplate
	}

	helpTemplate := params.Get("metric_help_template")
	if len(helpTemplate) != 0 {
		cfg.MetricHelpTemplate = helpTemplate
	}

	if regions, exists := params["regions"]; exists {
		cfg.Regions = regions
	}

	validateDimensions := params.Get("validate_dimensions")
	if len(validateDimensions) != 0 {
		v, err := strconv.ParseBool(validateDimensions)
		if err != nil {
			return Config{}, fmt.Errorf("invalid boolean value %s for validate_dimensions", validateDimensions)
		}
		cfg.ValidateDimensions = v
	}

	return cfg, nil
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
