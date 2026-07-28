package awscloudwatch

import (
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"
	"go.opentelemetry.io/collector/config/configoptional"
)

var (
	defaultLogGroupLimit = 50

	// Defaults for the metrics block. These mirror upstream's createDefaultConfig
	// rather than otelcol.DefaultScraperControllerArguments, whose generic
	// collection_interval of 1m would poll CloudWatch five times as often as the
	// equivalent OpenTelemetry Collector configuration.
	defaultMetricsCollectionInterval = 5 * time.Minute
	defaultMetricsPeriod             = 5 * time.Minute
	defaultMetricsDelay              = 10 * time.Minute
	defaultMetricsDiscoveryLimit     = 100
)

// LogsConfig is the configuration for the logs portion of this receiver
type LogsConfig struct {
	PollInterval        time.Duration `alloy:"poll_interval,attr,optional"`
	MaxEventsPerRequest int           `alloy:"max_events_per_request,attr,optional"`
	Groups              GroupConfig   `alloy:"groups,block,optional"`
	StartFrom           string        `alloy:"start_from,attr,optional"`
}

func (args LogsConfig) Convert() awscloudwatchreceiver.LogsConfig {
	return awscloudwatchreceiver.LogsConfig{
		PollInterval:        args.PollInterval,
		MaxEventsPerRequest: args.MaxEventsPerRequest,
		Groups:              args.Groups.Convert(),
		StartFrom:           args.StartFrom,
	}
}

func (args *LogsConfig) SetToDefault() {
	*args = LogsConfig{
		PollInterval:        time.Minute,
		MaxEventsPerRequest: 1000,
	}
}

// MetricsConfig is the configuration for the metrics (GetMetricData) portion of
// this receiver. Either Queries or Discovery may be set, not both.
type MetricsConfig struct {
	Controller otelcol.ScraperControllerArguments `alloy:",squash"`

	Period    time.Duration           `alloy:"period,attr,optional"`
	Delay     time.Duration           `alloy:"delay,attr,optional"`
	Queries   []MetricQuery           `alloy:"query,block,optional"`
	Discovery *MetricsDiscoveryConfig `alloy:"discovery,block,optional"`
}

// Convert returns the zero value when args is nil so that a configuration
// without a metrics block leaves the upstream Metrics config untouched.
func (args *MetricsConfig) Convert() awscloudwatchreceiver.MetricsConfig {
	if args == nil {
		return awscloudwatchreceiver.MetricsConfig{}
	}

	cfg := awscloudwatchreceiver.MetricsConfig{
		ControllerConfig: *args.Controller.Convert(),
		Period:           args.Period,
		Delay:            args.Delay,
		Discovery:        args.Discovery.Convert(),
	}

	for _, q := range args.Queries {
		cfg.Queries = append(cfg.Queries, q.Convert())
	}

	return cfg
}

func (args *MetricsConfig) SetToDefault() {
	if args == nil {
		return
	}

	*args = MetricsConfig{
		Period: defaultMetricsPeriod,
		Delay:  defaultMetricsDelay,
	}
	args.Controller.SetToDefault()
	args.Controller.CollectionInterval = defaultMetricsCollectionInterval
}

// MetricQuery defines a single CloudWatch metric to scrape via GetMetricData.
type MetricQuery struct {
	Namespace  string            `alloy:"namespace,attr"`
	MetricName string            `alloy:"metric_name,attr"`
	Dimensions map[string]string `alloy:"dimensions,attr,optional"`
	Stats      []string          `alloy:"stats,attr,optional"`
}

func (args MetricQuery) Convert() awscloudwatchreceiver.MetricQuery {
	return awscloudwatchreceiver.MetricQuery{
		Namespace:  args.Namespace,
		MetricName: args.MetricName,
		Dimensions: args.Dimensions,
		Stats:      args.Stats,
	}
}

// MetricsDiscoveryConfig configures automatic discovery of metrics via
// ListMetrics. Mutually exclusive with MetricsConfig.Queries.
type MetricsDiscoveryConfig struct {
	Filters *MetricsDiscoveryFilters `alloy:"filters,block,optional"`
	Limit   int                      `alloy:"limit,attr,optional"`
	Stats   []string                 `alloy:"stats,attr,optional"`
}

func (args *MetricsDiscoveryConfig) Convert() *awscloudwatchreceiver.MetricsDiscoveryConfig {
	if args == nil {
		return nil
	}

	return &awscloudwatchreceiver.MetricsDiscoveryConfig{
		Filters: args.Filters.Convert(),
		Limit:   args.Limit,
		Stats:   args.Stats,
	}
}

func (args *MetricsDiscoveryConfig) SetToDefault() {
	if args == nil {
		return
	}

	*args = MetricsDiscoveryConfig{
		Limit: defaultMetricsDiscoveryLimit,
	}
}

// MetricsDiscoveryFilters optionally narrows which metrics are discovered. When
// absent, all metrics in all namespaces are discovered.
type MetricsDiscoveryFilters struct {
	Namespace  string `alloy:"namespace,attr,optional"`
	MetricName string `alloy:"metric_name,attr,optional"`
}

func (args *MetricsDiscoveryFilters) Convert() configoptional.Optional[awscloudwatchreceiver.MetricsDiscoveryFilters] {
	if args == nil {
		return configoptional.None[awscloudwatchreceiver.MetricsDiscoveryFilters]()
	}

	return configoptional.Some(awscloudwatchreceiver.MetricsDiscoveryFilters{
		Namespace:  args.Namespace,
		MetricName: args.MetricName,
	})
}

// GroupConfig is the configuration for log group collection
type GroupConfig struct {
	AutodiscoverConfig *AutodiscoverConfig `alloy:"autodiscover,block,optional"`
	NamedConfigs       NamedConfigs        `alloy:"named,block,optional"`
}

func (args GroupConfig) Convert() awscloudwatchreceiver.GroupConfig {
	return awscloudwatchreceiver.GroupConfig{
		AutodiscoverConfig: args.AutodiscoverConfig.Convert(),
		NamedConfigs:       args.NamedConfigs.Convert(),
	}
}

type NamedConfigs []NamedConfig

type NamedConfig struct {
	GroupName string    `alloy:"group_name,attr"`
	Prefixes  []*string `alloy:"prefixes,attr,optional"`
	Names     []*string `alloy:"names,attr,optional"`
}

func (args NamedConfigs) Convert() map[string]awscloudwatchreceiver.StreamConfig {
	ret := make(map[string]awscloudwatchreceiver.StreamConfig)
	for _, c := range args {
		ret[c.GroupName] = awscloudwatchreceiver.StreamConfig{
			Prefixes: c.Prefixes,
			Names:    c.Names,
		}
	}
	return ret
}

// AutodiscoverConfig is the configuration for the autodiscovery functionality of log groups
type AutodiscoverConfig struct {
	Prefix  string       `alloy:"prefix,attr,optional"`
	Pattern string       `alloy:"pattern,attr,optional"`
	Limit   *int         `alloy:"limit,attr,optional"`
	Streams StreamConfig `alloy:"streams,block,optional"`

	AccountIdentifiers []string `alloy:"account_identifiers,attr,optional"`
	// IncludeLinkedAccounts must stay a pointer. Upstream only sets the field on
	// the AWS DescribeLogGroups request when it is non-nil, otherwise leaving
	// AWS's own default to apply. A plain bool would send an explicit false for
	// every existing configuration that omits the attribute.
	IncludeLinkedAccounts *bool `alloy:"include_linked_accounts,attr,optional"`
}

func (args *AutodiscoverConfig) Convert() *awscloudwatchreceiver.AutodiscoverConfig {
	if args == nil {
		return nil
	}

	// SetToDefault populates Limit when the block is present, but an explicit
	// `limit = null` leaves it nil, so fall back rather than dereferencing.
	limit := defaultLogGroupLimit
	if args.Limit != nil {
		limit = *args.Limit
	}

	return &awscloudwatchreceiver.AutodiscoverConfig{
		Prefix:                args.Prefix,
		Pattern:               args.Pattern,
		Limit:                 limit,
		Streams:               args.Streams.Convert(),
		AccountIdentifiers:    args.AccountIdentifiers,
		IncludeLinkedAccounts: args.IncludeLinkedAccounts,
	}
}

func (args *AutodiscoverConfig) SetToDefault() {
	if args == nil {
		return
	}
	defaultLimit := defaultLogGroupLimit
	*args = AutodiscoverConfig{
		Limit: &defaultLimit,
	}
}

// StreamConfig represents the configuration for the log stream filtering
type StreamConfig struct {
	Prefixes []*string `alloy:"prefixes,attr,optional"`
	Names    []*string `alloy:"names,attr,optional"`
}

func (args StreamConfig) Convert() awscloudwatchreceiver.StreamConfig {
	return awscloudwatchreceiver.StreamConfig{
		Prefixes: args.Prefixes,
		Names:    args.Names,
	}
}
