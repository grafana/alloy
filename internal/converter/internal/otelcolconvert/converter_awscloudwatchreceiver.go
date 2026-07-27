package otelcolconvert

import (
	"fmt"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/awscloudwatch"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/receiver/awscloudwatchreceiver"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componentstatus"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	converters = append(converters, awsCloudWatchReceiverConverter{})
}

type awsCloudWatchReceiverConverter struct{}

func (awsCloudWatchReceiverConverter) Factory() component.Factory {
	return awscloudwatchreceiver.NewFactory()
}

func (awsCloudWatchReceiverConverter) InputComponentName() string { return "" }

func (awsCloudWatchReceiverConverter) ConvertAndAppend(state *State, id componentstatus.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toAwsCloudWatchReceiver(state, id, cfg.(*awscloudwatchreceiver.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "receiver", "awscloudwatch"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toAwsCloudWatchReceiver(state *State, id componentstatus.InstanceID, cfg *awscloudwatchreceiver.Config) *awscloudwatch.Arguments {
	nextMetrics := state.Next(id, pipeline.SignalMetrics)
	nextLogs := state.Next(id, pipeline.SignalLogs)

	return &awscloudwatch.Arguments{
		Region:       cfg.Region,
		Profile:      cfg.Profile,
		IMDSEndpoint: cfg.IMDSEndpoint,
		Logs:         toLogsConfig(cfg.Logs),
		Metrics:      toMetricsConfig(cfg.Metrics),
		DebugMetrics: common.DefaultValue[awscloudwatch.Arguments]().DebugMetrics,
		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
			Logs:    ToTokenizedConsumers(nextLogs),
		},
	}
}

func toLogsConfig(cfg awscloudwatchreceiver.LogsConfig) awscloudwatch.LogsConfig {
	return awscloudwatch.LogsConfig{
		PollInterval:        cfg.PollInterval,
		MaxEventsPerRequest: cfg.MaxEventsPerRequest,
		Groups:              toGroupConfig(cfg.Groups),
		StartFrom:           cfg.StartFrom,
	}
}

// toMetricsConfig returns nil when metrics collection is not configured, so that
// the generated Alloy configuration omits the metrics block entirely.
func toMetricsConfig(cfg awscloudwatchreceiver.MetricsConfig) *awscloudwatch.MetricsConfig {
	if len(cfg.Queries) == 0 && cfg.Discovery == nil {
		return nil
	}

	out := &awscloudwatch.MetricsConfig{
		Controller: otelcol.ScraperControllerArguments{
			CollectionInterval: cfg.CollectionInterval,
			InitialDelay:       cfg.InitialDelay,
			Timeout:            cfg.Timeout,
		},
		Period:    cfg.Period,
		Delay:     cfg.Delay,
		Discovery: toMetricsDiscoveryConfig(cfg.Discovery),
	}

	for _, q := range cfg.Queries {
		out.Queries = append(out.Queries, awscloudwatch.MetricQuery{
			Namespace:  q.Namespace,
			MetricName: q.MetricName,
			Dimensions: q.Dimensions,
			Stats:      q.Stats,
		})
	}

	return out
}

func toMetricsDiscoveryConfig(cfg *awscloudwatchreceiver.MetricsDiscoveryConfig) *awscloudwatch.MetricsDiscoveryConfig {
	if cfg == nil {
		return nil
	}

	out := &awscloudwatch.MetricsDiscoveryConfig{
		Limit: cfg.Limit,
		Stats: cfg.Stats,
	}

	if cfg.Filters.HasValue() {
		filters := cfg.Filters.Get()
		out.Filters = &awscloudwatch.MetricsDiscoveryFilters{
			Namespace:  filters.Namespace,
			MetricName: filters.MetricName,
		}
	}

	return out
}

func toGroupConfig(cfg awscloudwatchreceiver.GroupConfig) awscloudwatch.GroupConfig {
	return awscloudwatch.GroupConfig{
		AutodiscoverConfig: toAutodiscoverConfig(cfg.AutodiscoverConfig),
		NamedConfigs:       toNamedConfigs(cfg.NamedConfigs),
	}
}

func toAutodiscoverConfig(cfg *awscloudwatchreceiver.AutodiscoverConfig) *awscloudwatch.AutodiscoverConfig {
	if cfg == nil {
		return nil
	}

	limit := new(int)
	*limit = cfg.Limit

	return &awscloudwatch.AutodiscoverConfig{
		Prefix:                cfg.Prefix,
		Pattern:               cfg.Pattern,
		Limit:                 limit,
		Streams:               toStreamConfig(cfg.Streams),
		AccountIdentifiers:    cfg.AccountIdentifiers,
		IncludeLinkedAccounts: cfg.IncludeLinkedAccounts,
	}
}

func toNamedConfigs(cfg map[string]awscloudwatchreceiver.StreamConfig) awscloudwatch.NamedConfigs {
	var configs []awscloudwatch.NamedConfig
	for groupName, streamCfg := range cfg {
		configs = append(configs, awscloudwatch.NamedConfig{
			GroupName: groupName,
			Prefixes:  streamCfg.Prefixes,
			Names:     streamCfg.Names,
		})
	}
	return configs
}

func toStreamConfig(cfg awscloudwatchreceiver.StreamConfig) awscloudwatch.StreamConfig {
	return awscloudwatch.StreamConfig{
		Prefixes: cfg.Prefixes,
		Names:    cfg.Names,
	}
}
