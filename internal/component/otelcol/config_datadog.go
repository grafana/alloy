package otelcol

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	"go.opentelemetry.io/collector/config/configopaque"
)

// DatadogAPISettings holds the configuration settings for the Datadog API.
type DatadogAPISettings struct {
	Key              configopaque.String `alloy:"api_key,attr"`
	Site             string              `alloy:"site,attr,optional"` // Default value of exporter is "datadoghq.com"
	FailOnInvalidKey bool                `alloy:"fail_on_invalid_key,attr,optional"`
}

// TracesConfig holds the configuration settings for the Datadog trace exporter
// See https://pkg.go.dev/github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter#TracesConfig for more
type TracesConfig struct {
	Endpoint               string            `alloy:"endpoint,attr,optional"`
	IgnoreResources        []string          `alloy:"ignore_resources,attr,optional"`
	SpanNameRemappings     map[string]string `alloy:"span_name_remappings,attr,optional"`
	SpanNameAsResourceName bool              `alloy:"span_name_as_resource_name,attr,optional"`
	ComputeStatsBySpanKind bool              `alloy:"compute_stats_by_span_kind,attr,optional"`
	PeerTagsAggregation    bool              `alloy:"peer_tags_aggregation,attr,optional"`
	PeerTags               []string          `alloy:"peer_tags,attr,optional"`
	TraceBuffer            int               `alloy:"trace_buffer,attr,optional"`
}

// MetricsExporterConfig holds the configuration settings for the Datadog metrics exporter
type MetricsConfig struct {
	DeltaTTL                           int64           `alloy:"delta_ttl,attr,optional"`
	Endpoint                           string          `alloy:"endpoint,attr,optional"`
	ResourceAttributesAsTags           bool            `alloy:"resource_attributes_as_tags,attr,optional"`
	InstrumentationScopeMetadataAsTags bool            `alloy:"instrumentation_scope_metadata_as_tags,attr,optional"`
	HistConfig                         HistogramConfig `alloy:"histograms,block,optional"`
	SumConfig                          SumConfig       `alloy:"sums,block,optional"`
	SummaryConfig                      SummaryConfig   `alloy:"summaries,block,optional"`
}

type HistogramConfig struct {
	Mode             datadogexporter.HistogramMode `alloy:"mode,attr,optional"`
	SendAggregations bool                          `alloy:"send_aggregation_metrics,attr,optional"`
}

type SumConfig struct {
	CumulativeMonotonicMode        datadogexporter.CumulativeMonotonicSumMode `alloy:"cumulative_monotonic_mode,attr,optional"`
	InitialCumulativeMonotonicMode datadogexporter.InitialValueMode           `alloy:"initial_cumulative_monotonic_value,attr,optional"`
}

type SummaryConfig struct {
	Mode datadogexporter.SummaryMode `alloy:"mode,attr,optional"`
}
