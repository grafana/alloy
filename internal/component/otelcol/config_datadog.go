package otelcol

import (
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
)

// DatadogAPISettings holds the configuration settings for the Datadog API.
type DatadogAPIArguments struct {
	Key              alloytypes.Secret `alloy:"api_key,attr"`
	Site             string            `alloy:"site,attr,optional"` // Default value of exporter is "datadoghq.com"
	FailOnInvalidKey bool              `alloy:"fail_on_invalid_key,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *DatadogAPIArguments) Convert() *datadogexporter.APIConfig {
	if args == nil {
		return nil
	}

	return &datadogexporter.APIConfig{
		Key:              configopaque.String(args.Key),
		Site:             args.Site,
		FailOnInvalidKey: args.FailOnInvalidKey,
	}
}

func (args *DatadogAPIArguments) SetToDefault() {
	*args = DatadogAPIArguments{
		Site: "datadoghq.com",
	}
}

// HostMetadataConfig holds information used for populating the infrastructure list,
// the host map and providing host tags functionality within the Datadog app.
// see https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.102.0/exporter/datadogexporter/config.go#L391
// for more
type DatadogHostMetadataArguments struct {
	Enabled        bool                           `alloy:"enabled,attr,optional"`
	HostnameSource datadogexporter.HostnameSource `alloy:"hostname_source,attr,optional"`
	Tags           []string                       `alloy:"tags,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *DatadogHostMetadataArguments) Convert() *datadogexporter.HostMetadataConfig {
	if args == nil {
		return nil
	}

	return &datadogexporter.HostMetadataConfig{
		Enabled:        args.Enabled,
		HostnameSource: args.HostnameSource,
		Tags:           args.Tags,
	}
}

func (args *DatadogHostMetadataArguments) SetToDefault() {
	*args = DatadogHostMetadataArguments{
		Enabled:        true,
		HostnameSource: datadogexporter.HostnameSourceConfigOrSystem,
	}
}

// TracesConfig holds the configuration settings for the Datadog trace exporter
// See https://pkg.go.dev/github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter#TracesConfig for more
type DatadogTracesArguments struct {
	Endpoint                  string            `alloy:"endpoint,attr,optional"`
	IgnoreResources           []string          `alloy:"ignore_resources,attr,optional"`
	SpanNameRemappings        map[string]string `alloy:"span_name_remappings,attr,optional"`
	SpanNameAsResourceName    bool              `alloy:"span_name_as_resource_name,attr,optional"`
	ComputeStatsBySpanKind    bool              `alloy:"compute_stats_by_span_kind,attr,optional"`
	ComputeTopLevelBySpanKind bool              `alloy:"compute_top_level_by_span_kind,attr,optional"`
	PeerTagsAggregation       bool              `alloy:"peer_tags_aggregation,attr,optional"`
	PeerTags                  []string          `alloy:"peer_tags,attr,optional"`
	TraceBuffer               int               `alloy:"trace_buffer,attr,optional"`
}

func (args *DatadogTracesArguments) Convert() *datadogexporter.TracesConfig {
	if args == nil {
		return nil
	}

	return &datadogexporter.TracesConfig{
		TCPAddrConfig:             confignet.TCPAddrConfig{Endpoint: args.Endpoint},
		IgnoreResources:           args.IgnoreResources,
		SpanNameRemappings:        args.SpanNameRemappings,
		SpanNameAsResourceName:    args.SpanNameAsResourceName,
		ComputeStatsBySpanKind:    args.ComputeStatsBySpanKind,
		ComputeTopLevelBySpanKind: args.ComputeTopLevelBySpanKind,
		PeerTagsAggregation:       args.PeerTagsAggregation,
		PeerTags:                  args.PeerTags,
		TraceBuffer:               args.TraceBuffer,
	}
}

func (args *DatadogTracesArguments) SetToDefault() {
	*args = DatadogTracesArguments{
		IgnoreResources: []string{},
	}
}

// MetricsExporterConfig holds the configuration settings for the Datadog metrics exporter
type DatadogMetricsArguments struct {
	DeltaTTL                           int64                     `alloy:"delta_ttl,attr,optional"`
	Endpoint                           string                    `alloy:"endpoint,attr,optional"`
	ResourceAttributesAsTags           bool                      `alloy:"resource_attributes_as_tags,attr,optional"`
	InstrumentationScopeMetadataAsTags bool                      `alloy:"instrumentation_scope_metadata_as_tags,attr,optional"`
	HistConfig                         DatadogHistogramArguments `alloy:"histograms,block,optional"`
	SumConfig                          DatadogSumArguments       `alloy:"sums,block,optional"`
	SummaryConfig                      DatadogSummaryArguments   `alloy:"summaries,block,optional"`
}

func (args *DatadogMetricsArguments) Convert() *datadogexporter.MetricsConfig {
	if args == nil {
		return nil
	}

	return &datadogexporter.MetricsConfig{
		DeltaTTL:      args.DeltaTTL,
		TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: args.Endpoint},
		ExporterConfig: datadogexporter.MetricsExporterConfig{
			ResourceAttributesAsTags:           args.ResourceAttributesAsTags,
			InstrumentationScopeMetadataAsTags: args.InstrumentationScopeMetadataAsTags,
		},
		HistConfig:    *args.HistConfig.Convert(),
		SumConfig:     *args.SumConfig.Convert(),
		SummaryConfig: *args.SummaryConfig.Convert(),
	}
}

func (args *DatadogMetricsArguments) SetToDefault() {
	*args = DatadogMetricsArguments{
		DeltaTTL:                           3600,
		ResourceAttributesAsTags:           false,
		InstrumentationScopeMetadataAsTags: false,
		HistConfig: DatadogHistogramArguments{
			Mode:             "distributions",
			SendAggregations: false,
		},
		SumConfig: DatadogSumArguments{
			CumulativeMonotonicMode:        datadogexporter.CumulativeMonotonicSumModeToDelta,
			InitialCumulativeMonotonicMode: datadogexporter.InitialValueModeAuto,
		},
		SummaryConfig: DatadogSummaryArguments{
			Mode: datadogexporter.SummaryModeGauges,
		},
	}
}

// HistogramConfig holds Histogram specific configuration settings
type DatadogHistogramArguments struct {
	Mode             datadogexporter.HistogramMode `alloy:"mode,attr,optional"`
	SendAggregations bool                          `alloy:"send_aggregation_metrics,attr,optional"`
}

func (args *DatadogHistogramArguments) Convert() *datadogexporter.HistogramConfig {
	if args == nil {
		return nil
	}

	return &datadogexporter.HistogramConfig{
		Mode:             args.Mode,
		SendAggregations: args.SendAggregations,
	}
}

func (args *DatadogHistogramArguments) SetToDefault() {
	*args = DatadogHistogramArguments{
		Mode:             "distributions",
		SendAggregations: false,
	}
}

// SumConfig holds Sum specific configuration settings
type DatadogSumArguments struct {
	CumulativeMonotonicMode        datadogexporter.CumulativeMonotonicSumMode `alloy:"cumulative_monotonic_mode,attr,optional"`
	InitialCumulativeMonotonicMode datadogexporter.InitialValueMode           `alloy:"initial_cumulative_monotonic_value,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *DatadogSumArguments) Convert() *datadogexporter.SumConfig {
	if args == nil {
		return nil
	}

	return &datadogexporter.SumConfig{
		CumulativeMonotonicMode:        args.CumulativeMonotonicMode,
		InitialCumulativeMonotonicMode: args.InitialCumulativeMonotonicMode,
	}
}

func (args *DatadogSumArguments) SetToDefault() {
	*args = DatadogSumArguments{
		CumulativeMonotonicMode:        datadogexporter.CumulativeMonotonicSumModeToDelta,
		InitialCumulativeMonotonicMode: datadogexporter.InitialValueModeAuto,
	}
}

// SummaryConfig holds Summary specific configuration settings
type DatadogSummaryArguments struct {
	Mode datadogexporter.SummaryMode `alloy:"mode,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *DatadogSummaryArguments) Convert() *datadogexporter.SummaryConfig {
	if args == nil {
		return nil
	}

	return &datadogexporter.SummaryConfig{
		Mode: args.Mode,
	}
}

func (args *DatadogSummaryArguments) SetToDefault() {
	*args = DatadogSummaryArguments{
		Mode: datadogexporter.SummaryModeGauges,
	}
}
