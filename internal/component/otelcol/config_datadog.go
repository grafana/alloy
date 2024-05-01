package otelcol

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
)

// TracesConfig holds the configuration settings for the Datadog trace exporter
// See https://pkg.go.dev/github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter#TracesConfig for more
type TracesConfig struct {
	confignet.TCPAddrConfig `mapstructure:",squash"`

	IgnoreResources []string `alloy:"ignore_resources"`

	SpanNameRemappings map[string]string `alloy:"span_name_remappings"`

	SpanNameAsResourceName bool `alloy:"span_name_as_resource_name"`

	ComputeStatsBySpanKind bool `alloy:"compute_stats_by_span_kind"`

	PeerServiceAggregation bool `alloy:"peer_service_aggregation"`

	PeerTagsAggregation bool `alloy:"peer_tags_aggregation"`

	PeerTags    []string `alloy:"peer_tags"`
	TraceBuffer int      `alloy:"trace_buffer"`
}

// MetricsExporterConfig holds the configuration settings for the Datadog metrics exporter
type MetricsConfig struct {
	// DeltaTTL defines the time that previous points of a cumulative monotonic
	// metric are kept in memory to calculate deltas
	DeltaTTL int64 `alloy:"delta_ttl"`

	// TCPAddr.Endpoint is the host of the Datadog intake server to send metrics to.
	// If unset, the value is obtained from the Site.
	confignet.TCPAddrConfig `alloy:"block,squash"`

	ExporterConfig MetricsExporterConfig `mapstructure:",squash"`

	// HistConfig defines the export of OTLP Histograms.
	HistConfig HistogramConfig `mapstructure:"histograms"`

	// SumConfig defines the export of OTLP Sums.
	SumConfig SumConfig `mapstructure:"sums"`

	// SummaryConfig defines the export for OTLP Summaries.
	SummaryConfig SummaryConfig `mapstructure:"summaries"`
}

type MetricsExporterConfig struct {
	ResourceAttributesAsTags bool `mapstructure:"resource_attributes_as_tags"`

	InstrumentationScopeMetadataAsTags bool `mapstructure:"instrumentation_scope_metadata_as_tags"`
}

type HistogramConfig struct {
	Mode             datadogexporter.HistogramMode `alloy:"mode"`
	SendCountSum     bool                          `alloy:"send_count_sum_metrics"`
	SendAggregations bool                          `alloy:"send_aggregation_metrics"`
}

type SumConfig struct {
	CumulativeMonotonicMode datadogexporter.CumulativeMonotonicSumMode `mapstructure:"cumulative_monotonic_mode"`

	InitialCumulativeMonotonicMode datadogexporter.InitialValueMode `mapstructure:"initial_cumulative_monotonic_value"`
}

type SummaryConfig struct {
	Mode datadogexporter.SummaryMode `mapstructure:"mode"`
}

type TCPAddrConfig struct {
	// Endpoint configures the address for this network connection.
	// The address has the form "host:port". The host must be a literal IP address, or a host name that can be
	// resolved to IP addresses. The port must be a literal port number or a service name.
	// If the host is a literal IPv6 address it must be enclosed in square brackets, as in "[2001:db8::1]:80" or
	// "[fe80::1%zone]:80". The zone specifies the scope of the literal IPv6 address as defined in RFC 4007.
	Endpoint string `alloy:"endpoint"`

	// DialerConfig contains options for connecting to an address.
	DialerConfig DialerConfig `alloy:"dialer"`
}

type DialerConfig struct {
	// Timeout is the maximum amount of time a dial will wait for
	// a connect to complete. The default is no timeout.
	Timeout time.Duration `alloy:"timeout"`
}

// DatadogAPISettings holds the configuration settings for the Datadog API.
type DatadogAPISettings struct {
	Key              configopaque.String `alloy:"api_key,attr"`
	Site             string              `alloy:"site,attr,optional"` // Default value of exporter is "datadoghq.com"
	FailOnInvalidKey bool                `alloy:"fail_on_invalid_key,attr,optional"`
}
