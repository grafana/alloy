package otelcol

import (
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
)

// TracesConfig holds the configuration settings for the Datadog trace exporter
// See https://pkg.go.dev/github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter#TracesConfig for more
type TracesConfig struct {
	confignet.TCPAddrConfig `mapstructure:",squash"`

	IgnoreResources []string `mapstructure:"ignore_resources"`

	SpanNameRemappings map[string]string `mapstructure:"span_name_remappings"`

	SpanNameAsResourceName bool `mapstructure:"span_name_as_resource_name"`

	ComputeStatsBySpanKind bool `mapstructure:"compute_stats_by_span_kind"`

	PeerServiceAggregation bool `mapstructure:"peer_service_aggregation"`

	PeerTagsAggregation bool `mapstructure:"peer_tags_aggregation"`

	PeerTags    []string `mapstructure:"peer_tags"`
	TraceBuffer int      `mapstructure:"trace_buffer"`
}

// MetricsExporterConfig holds the configuration settings for the Datadog metrics exporter
type MetricsConfig struct {
	// DeltaTTL defines the time that previous points of a cumulative monotonic
	// metric are kept in memory to calculate deltas
	DeltaTTL int64 `mapstructure:"delta_ttl"`

	// TCPAddr.Endpoint is the host of the Datadog intake server to send metrics to.
	// If unset, the value is obtained from the Site.
	confignet.TCPAddrConfig `mapstructure:",squash"`

	ExporterConfig MetricsExporterConfig `mapstructure:",squash"`

	// HistConfig defines the export of OTLP Histograms.
	HistConfig HistogramConfig `mapstructure:"histograms"`

	// SumConfig defines the export of OTLP Sums.
	SumConfig SumConfig `mapstructure:"sums"`

	// SummaryConfig defines the export for OTLP Summaries.
	SummaryConfig SummaryConfig `mapstructure:"summaries"`
}

type MetricsExporterConfig struct {
	// ResourceAttributesAsTags, if set to true, will use the exporterhelper feature to transform all
	// resource attributes into metric labels, which are then converted into tags
	ResourceAttributesAsTags bool `mapstructure:"resource_attributes_as_tags"`

	// InstrumentationScopeMetadataAsTags, if set to true, adds the name and version of the
	// instrumentation scope that created a metric to the metric tags
	InstrumentationScopeMetadataAsTags bool `mapstructure:"instrumentation_scope_metadata_as_tags"`
}

type HistogramConfig struct {
	// Mode for exporting histograms. Valid values are 'distributions', 'counters' or 'nobuckets'.
	//  - 'distributions' sends histograms as Datadog distributions (recommended).
	//  - 'counters' sends histograms as Datadog counts, one metric per bucket.
	//  - 'nobuckets' sends no bucket histogram metrics. Aggregation metrics will still be sent
	//    if `send_aggregation_metrics` is enabled.
	//
	// The current default is 'distributions'.
	Mode HistogramMode `mapstructure:"mode"`

	// SendCountSum states if the export should send .sum and .count metrics for histograms.
	// The default is false.
	// Deprecated: [v0.75.0] Use `send_aggregation_metrics` (HistogramConfig.SendAggregations) instead.
	SendCountSum bool `mapstructure:"send_count_sum_metrics"`

	// SendAggregations states if the exporter should send .sum, .count, .min and .max metrics for histograms.
	// The default is false.
	SendAggregations bool `mapstructure:"send_aggregation_metrics"`
}

// DatadogAPISettings holds the configuration settings for the Datadog API.
type DatadogAPISettings struct {
	Key              configopaque.String `alloy:"api_key,attr"`
	Site             string              `alloy:"site,attr,optional"` // Default value of exporter is "datadoghq.com"
	FailOnInvalidKey bool                `alloy:"fail_on_invalid_key,attr,optional"`
}
