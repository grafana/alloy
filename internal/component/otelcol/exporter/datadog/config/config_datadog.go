package datadog_config

import (
	"time"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
)

// DatadogClientArguments holds the configuration settings for the Datadog client.
type DatadogClientArguments struct {
	ReadBufferSize      int            `alloy:"read_buffer_size,attr,optional"`
	WriteBufferSize     int            `alloy:"write_buffer_size,attr,optional"`
	Timeout             time.Duration  `alloy:"timeout,attr,optional"`
	MaxIdleConns        *int           `alloy:"max_idle_conns,attr,optional"`
	MaxIdleConnsPerHost *int           `alloy:"max_idle_conns_per_host,attr,optional"`
	MaxConnsPerHost     *int           `alloy:"max_conns_per_host,attr,optional"`
	IdleConnTimeout     *time.Duration `alloy:"idle_conn_timeout,attr,optional"`
	DisableKeepAlives   bool           `alloy:"disable_keep_alives,attr,optional"`
	InsecureSkipVerify  bool           `alloy:"insecure_skip_verify,attr,optional"`
}

func (args *DatadogClientArguments) Convert() *confighttp.ClientConfig {
	if args == nil {
		return nil
	}

	return &confighttp.ClientConfig{
		Endpoint:            "",
		Headers:             nil,
		ReadBufferSize:      args.ReadBufferSize,
		WriteBufferSize:     args.WriteBufferSize,
		Timeout:             args.Timeout,
		MaxIdleConns:        args.MaxIdleConns,
		MaxIdleConnsPerHost: args.MaxIdleConnsPerHost,
		MaxConnsPerHost:     args.MaxConnsPerHost,
		IdleConnTimeout:     args.IdleConnTimeout,
		DisableKeepAlives:   args.DisableKeepAlives,
		TLSSetting: configtls.ClientConfig{
			InsecureSkipVerify: args.InsecureSkipVerify,
		},
	}
}

func (args *DatadogClientArguments) SetToDefault() {
	*args = DatadogClientArguments{
		Timeout: 15 * time.Second,
	}
}

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
	Enabled        bool     `alloy:"enabled,attr,optional"`
	HostnameSource string   `alloy:"hostname_source,attr,optional"`
	Tags           []string `alloy:"tags,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *DatadogHostMetadataArguments) Convert() *datadogexporter.HostMetadataConfig {
	if args == nil {
		return nil
	}

	return &datadogexporter.HostMetadataConfig{
		Enabled:        args.Enabled,
		HostnameSource: datadogexporter.HostnameSource(args.HostnameSource),
		Tags:           args.Tags,
	}
}

func (args *DatadogHostMetadataArguments) SetToDefault() {
	*args = DatadogHostMetadataArguments{
		Enabled:        true,
		HostnameSource: "config_or_system",
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

func (args *DatadogTracesArguments) Convert(endpoint string) *datadogexporter.TracesConfig {
	if args == nil {
		return nil
	}

	if args.Endpoint != "" {
		endpoint = args.Endpoint
	}

	return &datadogexporter.TracesConfig{
		TCPAddrConfig:             confignet.TCPAddrConfig{Endpoint: endpoint},
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

// DatadogMetricsArguments holds the configuration settings for the Datadog metrics exporter
type DatadogMetricsArguments struct {
	DeltaTTL       int64                           `alloy:"delta_ttl,attr,optional"`
	Endpoint       string                          `alloy:"endpoint,attr,optional"`
	ExporterConfig DatadogMetricsExporterArguments `alloy:"exporter,block,optional"`
	HistConfig     DatadogHistogramArguments       `alloy:"histograms,block,optional"`
	SumConfig      DatadogSumArguments             `alloy:"sums,block,optional"`
	SummaryConfig  DatadogSummaryArguments         `alloy:"summaries,block,optional"`
}

func (args *DatadogMetricsArguments) Convert(endpoint string) *datadogexporter.MetricsConfig {
	if args == nil {
		return nil
	}

	if args.Endpoint != "" {
		endpoint = args.Endpoint
	}

	return &datadogexporter.MetricsConfig{
		DeltaTTL:       args.DeltaTTL,
		TCPAddrConfig:  confignet.TCPAddrConfig{Endpoint: endpoint},
		ExporterConfig: *args.ExporterConfig.Convert(),
		HistConfig:     *args.HistConfig.Convert(),
		SumConfig:      *args.SumConfig.Convert(),
		SummaryConfig:  *args.SummaryConfig.Convert(),
	}
}

func (args *DatadogMetricsArguments) SetToDefault() {
	*args = DatadogMetricsArguments{
		DeltaTTL: 3600,
		ExporterConfig: DatadogMetricsExporterArguments{
			ResourceAttributesAsTags:           false,
			InstrumentationScopeMetadataAsTags: false,
		},
		HistConfig: DatadogHistogramArguments{
			Mode:             "distributions",
			SendAggregations: false,
		},
		SumConfig: DatadogSumArguments{
			CumulativeMonotonicMode:        "to_delta",
			InitialCumulativeMonotonicMode: "auto",
		},
		SummaryConfig: DatadogSummaryArguments{
			Mode: "gauges",
		},
	}
}

// DatadogMetricsExporterArguments holds the configuration settings for the Datadog metrics exporter
type DatadogMetricsExporterArguments struct {
	ResourceAttributesAsTags           bool `alloy:"resource_attributes_as_tags,attr,optional"`
	InstrumentationScopeMetadataAsTags bool `alloy:"instrumentation_scope_metadata_as_tags,attr,optional"`
}

func (args *DatadogMetricsExporterArguments) Convert() *datadogexporter.MetricsExporterConfig {
	if args == nil {
		return nil
	}

	return &datadogexporter.MetricsExporterConfig{
		ResourceAttributesAsTags:           args.ResourceAttributesAsTags,
		InstrumentationScopeMetadataAsTags: args.InstrumentationScopeMetadataAsTags,
	}
}

func (args *DatadogMetricsExporterArguments) SetToDefault() {
	*args = DatadogMetricsExporterArguments{
		ResourceAttributesAsTags:           false,
		InstrumentationScopeMetadataAsTags: false,
	}
}

// HistogramConfig holds Histogram specific configuration settings
type DatadogHistogramArguments struct {
	Mode             string `alloy:"mode,attr,optional"`
	SendAggregations bool   `alloy:"send_aggregation_metrics,attr,optional"`
}

func (args *DatadogHistogramArguments) Convert() *datadogexporter.HistogramConfig {
	if args == nil {
		return nil
	}

	return &datadogexporter.HistogramConfig{
		Mode:             datadogexporter.HistogramMode(args.Mode),
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
	CumulativeMonotonicMode        string `alloy:"cumulative_monotonic_mode,attr,optional"`
	InitialCumulativeMonotonicMode string `alloy:"initial_cumulative_monotonic_value,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *DatadogSumArguments) Convert() *datadogexporter.SumConfig {
	if args == nil {
		return nil
	}

	return &datadogexporter.SumConfig{
		CumulativeMonotonicMode:        datadogexporter.CumulativeMonotonicSumMode(args.CumulativeMonotonicMode),
		InitialCumulativeMonotonicMode: datadogexporter.InitialValueMode(args.InitialCumulativeMonotonicMode),
	}
}

func (args *DatadogSumArguments) SetToDefault() {
	*args = DatadogSumArguments{
		CumulativeMonotonicMode:        string(datadogexporter.CumulativeMonotonicSumModeToDelta),
		InitialCumulativeMonotonicMode: string(datadogexporter.InitialValueModeAuto),
	}
}

// SummaryConfig holds Summary specific configuration settings
type DatadogSummaryArguments struct {
	Mode string `alloy:"mode,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *DatadogSummaryArguments) Convert() *datadogexporter.SummaryConfig {
	if args == nil {
		return nil
	}
	return &datadogexporter.SummaryConfig{
		Mode: datadogexporter.SummaryMode(args.Mode),
	}
}

func (args *DatadogSummaryArguments) SetToDefault() {
	*args = DatadogSummaryArguments{
		Mode: string(datadogexporter.SummaryModeGauges),
	}
}
