//go:build !freebsd && !openbsd

package datadog_config

import (
	"time"

	"github.com/grafana/alloy/syntax/alloytypes"
	datadogOtelconfig "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/datadog/config"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/confignet"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtls"
)

// DatadogClientArguments holds the configuration settings for the Datadog client.
// Datadog Exporter only supports InsecureSkipVerify for TLS configuration.
// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.105.0/exporter/datadogexporter/examples/collector.yaml#L219
type DatadogClientArguments struct {
	ReadBufferSize      int           `alloy:"read_buffer_size,attr,optional"`
	WriteBufferSize     int           `alloy:"write_buffer_size,attr,optional"`
	Timeout             time.Duration `alloy:"timeout,attr,optional"`
	MaxIdleConns        int           `alloy:"max_idle_conns,attr,optional"`
	MaxIdleConnsPerHost int           `alloy:"max_idle_conns_per_host,attr,optional"`
	MaxConnsPerHost     int           `alloy:"max_conns_per_host,attr,optional"`
	IdleConnTimeout     time.Duration `alloy:"idle_conn_timeout,attr,optional"`
	DisableKeepAlives   bool          `alloy:"disable_keep_alives,attr,optional"`
	InsecureSkipVerify  bool          `alloy:"insecure_skip_verify,attr,optional"`
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
		TLS: configtls.ClientConfig{
			InsecureSkipVerify: args.InsecureSkipVerify,
		},
	}
}

func (args *DatadogClientArguments) SetToDefault() {
	// Additional defaults are set on the OTel side if values aren't provided.
	// These are the defaults listed in the Alloy docs.
	// We leave this to OTel as the types for MaxIdleConns etc are ptrs, which is difficult for Alloy to default.
	// https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/v0.105.0/exporter/datadogexporter/internal/clientutil/http.go#L49
	*args = DatadogClientArguments{
		Timeout:         15 * time.Second,
		MaxIdleConns:    100,
		IdleConnTimeout: 90 * time.Second,
	}
}

// DatadogAPISettings holds the configuration settings for the Datadog API.
type DatadogAPIArguments struct {
	Key              alloytypes.Secret `alloy:"api_key,attr"`
	Site             string            `alloy:"site,attr,optional"` // Default value of exporter is "datadoghq.com"
	FailOnInvalidKey bool              `alloy:"fail_on_invalid_key,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *DatadogAPIArguments) Convert() *datadogOtelconfig.APIConfig {
	if args == nil {
		return nil
	}

	return &datadogOtelconfig.APIConfig{
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
func (args *DatadogHostMetadataArguments) Convert() *datadogOtelconfig.HostMetadataConfig {
	if args == nil {
		return nil
	}

	return &datadogOtelconfig.HostMetadataConfig{
		Enabled:        args.Enabled,
		HostnameSource: datadogOtelconfig.HostnameSource(args.HostnameSource),
		Tags:           args.Tags,
		//TODO: Make ReporterPeriod configurable.
		ReporterPeriod: 30 * time.Minute,
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

func (args *DatadogTracesArguments) Convert(endpoint string) *datadogOtelconfig.TracesExporterConfig {
	if args == nil {
		return nil
	}

	if args.Endpoint != "" {
		endpoint = args.Endpoint
	}

	return &datadogOtelconfig.TracesExporterConfig{
		TCPAddrConfig: confignet.TCPAddrConfig{Endpoint: endpoint},
		TracesConfig: datadogOtelconfig.TracesConfig{
			IgnoreResources:           args.IgnoreResources,
			SpanNameRemappings:        args.SpanNameRemappings,
			SpanNameAsResourceName:    args.SpanNameAsResourceName,
			ComputeStatsBySpanKind:    args.ComputeStatsBySpanKind,
			ComputeTopLevelBySpanKind: args.ComputeTopLevelBySpanKind,
			PeerTagsAggregation:       args.PeerTagsAggregation,
			PeerTags:                  args.PeerTags,
		},
		TraceBuffer: args.TraceBuffer,
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

func (args *DatadogMetricsArguments) Convert(endpoint string) *datadogOtelconfig.MetricsConfig {
	if args == nil {
		return nil
	}

	if args.Endpoint != "" {
		endpoint = args.Endpoint
	}

	return &datadogOtelconfig.MetricsConfig{
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
			InstrumentationScopeMetadataAsTags: true,
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

func (args *DatadogMetricsExporterArguments) Convert() *datadogOtelconfig.MetricsExporterConfig {
	if args == nil {
		return nil
	}

	return &datadogOtelconfig.MetricsExporterConfig{
		ResourceAttributesAsTags:           args.ResourceAttributesAsTags,
		InstrumentationScopeMetadataAsTags: args.InstrumentationScopeMetadataAsTags,
	}
}

func (args *DatadogMetricsExporterArguments) SetToDefault() {
	*args = DatadogMetricsExporterArguments{
		ResourceAttributesAsTags:           false,
		InstrumentationScopeMetadataAsTags: true,
	}
}

// HistogramConfig holds Histogram specific configuration settings
type DatadogHistogramArguments struct {
	Mode             string `alloy:"mode,attr,optional"`
	SendAggregations bool   `alloy:"send_aggregation_metrics,attr,optional"`
}

func (args *DatadogHistogramArguments) Convert() *datadogOtelconfig.HistogramConfig {
	if args == nil {
		return nil
	}

	return &datadogOtelconfig.HistogramConfig{
		Mode:             datadogOtelconfig.HistogramMode(args.Mode),
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
func (args *DatadogSumArguments) Convert() *datadogOtelconfig.SumConfig {
	if args == nil {
		return nil
	}

	return &datadogOtelconfig.SumConfig{
		CumulativeMonotonicMode:        datadogOtelconfig.CumulativeMonotonicSumMode(args.CumulativeMonotonicMode),
		InitialCumulativeMonotonicMode: datadogOtelconfig.InitialValueMode(args.InitialCumulativeMonotonicMode),
	}
}

func (args *DatadogSumArguments) SetToDefault() {
	*args = DatadogSumArguments{
		CumulativeMonotonicMode:        string(datadogOtelconfig.CumulativeMonotonicSumModeToDelta),
		InitialCumulativeMonotonicMode: string(datadogOtelconfig.InitialValueModeAuto),
	}
}

// SummaryConfig holds Summary specific configuration settings
type DatadogSummaryArguments struct {
	Mode string `alloy:"mode,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *DatadogSummaryArguments) Convert() *datadogOtelconfig.SummaryConfig {
	if args == nil {
		return nil
	}
	return &datadogOtelconfig.SummaryConfig{
		Mode: datadogOtelconfig.SummaryMode(args.Mode),
	}
}

func (args *DatadogSummaryArguments) SetToDefault() {
	*args = DatadogSummaryArguments{
		Mode: string(datadogOtelconfig.SummaryModeGauges),
	}
}

// DatadogLogsArguments holds Summary specific configuration settings
type DatadogLogsArguments struct {
	Endpoint         string `alloy:"endpoint,attr,optional"`
	UseCompression   bool   `alloy:"use_compression,attr,optional"`
	CompressionLevel int    `alloy:"compression_level,attr,optional"`
	BatchWait        int    `alloy:"batch_wait,attr,optional"`
}

// Convert converts args into the upstream type.
func (args *DatadogLogsArguments) Convert(endpoint string) *datadogOtelconfig.LogsConfig {
	if args == nil {
		return nil
	}
	if args.Endpoint != "" {
		endpoint = args.Endpoint
	}
	return &datadogOtelconfig.LogsConfig{
		TCPAddrConfig:    confignet.TCPAddrConfig{Endpoint: endpoint},
		UseCompression:   args.UseCompression,
		CompressionLevel: args.CompressionLevel,
		BatchWait:        args.BatchWait,
	}
}

// SetToDefault sets the default values for the DatadogLogsArguments
func (args *DatadogLogsArguments) SetToDefault() {
	*args = DatadogLogsArguments{
		UseCompression:   true,
		CompressionLevel: 6,
		BatchWait:        5,
	}
}
