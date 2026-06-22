// Package config contains configuration arguments for the
// otelcol.exporter.elasticsearch component.
package config

import (
	"errors"
	"net/http"
	"slices"
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/auth"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/elasticsearchexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelconfighttp "go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/pipeline"
)

// ElasticsearchArguments configures the otelcol.exporter.elasticsearch component.
type ElasticsearchArguments struct {
	Endpoints []string `alloy:"endpoints,attr,optional"`
	CloudID   string   `alloy:"cloudid,attr,optional"`

	NumWorkers int `alloy:"num_workers,attr,optional"`

	LogsIndex    string `alloy:"logs_index,attr,optional"`
	MetricsIndex string `alloy:"metrics_index,attr,optional"`
	TracesIndex  string `alloy:"traces_index,attr,optional"`

	Pipeline string `alloy:"pipeline,attr,optional"`

	IncludeSourceOnError *bool    `alloy:"include_source_on_error,attr,optional"`
	MetadataKeys         []string `alloy:"metadata_keys,attr,optional"`

	Client         ClientArguments         `alloy:"client,block,optional"`
	Authentication AuthenticationArguments `alloy:"authentication,block,optional"`
	Discovery      DiscoveryArguments      `alloy:"discover,block,optional"`
	Retry          RetryArguments          `alloy:"retry,block,optional"`
	Flush          FlushArguments          `alloy:"flush,block,optional"`
	Mapping        MappingArguments        `alloy:"mapping,block,optional"`
	LogstashFormat LogstashFormatArguments `alloy:"logstash_format,block,optional"`
	Telemetry      TelemetryArguments      `alloy:"telemetry,block,optional"`

	LogsDynamicIndex    DynamicIndexArguments    `alloy:"logs_dynamic_index,block,optional"`
	MetricsDynamicIndex DynamicIndexArguments    `alloy:"metrics_dynamic_index,block,optional"`
	TracesDynamicIndex  DynamicIndexArguments    `alloy:"traces_dynamic_index,block,optional"`
	LogsDynamicID       DynamicIDArguments       `alloy:"logs_dynamic_id,block,optional"`
	TracesDynamicID     DynamicIDArguments       `alloy:"traces_dynamic_id,block,optional"`
	LogsDynamicPipeline DynamicPipelineArguments `alloy:"logs_dynamic_pipeline,block,optional"`

	SendingQueue otelcol.QueueArguments `alloy:"sending_queue,block,optional"`

	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// ClientArguments mirrors otelcol.HTTPClientArguments but marks the endpoint
// attribute as optional, because the Elasticsearch exporter accepts the
// endpoint via the top-level `endpoints` list or `cloudid` instead.
type ClientArguments struct {
	Endpoint string `alloy:"endpoint,attr,optional"`

	ProxyURL string `alloy:"proxy_url,attr,optional"`

	Compression       otelcol.CompressionType    `alloy:"compression,attr,optional"`
	CompressionParams *otelcol.CompressionParams `alloy:"compression_params,block,optional"`

	TLS otelcol.TLSClientArguments `alloy:"tls,block,optional"`

	ReadBufferSize       units.Base2Bytes  `alloy:"read_buffer_size,attr,optional"`
	WriteBufferSize      units.Base2Bytes  `alloy:"write_buffer_size,attr,optional"`
	Timeout              time.Duration     `alloy:"timeout,attr,optional"`
	Headers              map[string]string `alloy:"headers,attr,optional"`
	MaxIdleConns         int               `alloy:"max_idle_conns,attr,optional"`
	MaxIdleConnsPerHost  int               `alloy:"max_idle_conns_per_host,attr,optional"`
	MaxConnsPerHost      int               `alloy:"max_conns_per_host,attr,optional"`
	IdleConnTimeout      time.Duration     `alloy:"idle_conn_timeout,attr,optional"`
	DisableKeepAlives    bool              `alloy:"disable_keep_alives,attr,optional"`
	HTTP2ReadIdleTimeout time.Duration     `alloy:"http2_read_idle_timeout,attr,optional"`
	HTTP2PingTimeout     time.Duration     `alloy:"http2_ping_timeout,attr,optional"`
	ForceAttemptHTTP2    bool              `alloy:"force_attempt_http2,attr,optional"`

	Authentication *auth.Handler `alloy:"auth,attr,optional"`

	Cookies *otelcol.Cookies `alloy:"cookies,block,optional"`
}

// AuthenticationArguments configures Elasticsearch basic-auth/API-key credentials.
type AuthenticationArguments struct {
	User     string            `alloy:"user,attr,optional"`
	Password alloytypes.Secret `alloy:"password,attr,optional"`
	APIKey   alloytypes.Secret `alloy:"api_key,attr,optional"`
}

// DiscoveryArguments configures Elasticsearch node discovery (sniffing).
type DiscoveryArguments struct {
	OnStart  bool          `alloy:"on_start,attr,optional"`
	Interval time.Duration `alloy:"interval,attr,optional"`
}

// RetryArguments configures the Elasticsearch exporter's own retry behavior.
// This is distinct from otelcol.RetryArguments because the upstream exporter
// uses its own RetrySettings type, not configretry.BackOffConfig.
type RetryArguments struct {
	Enabled         bool          `alloy:"enabled,attr,optional"`
	MaxRetries      int           `alloy:"max_retries,attr,optional"`
	InitialInterval time.Duration `alloy:"initial_interval,attr,optional"`
	MaxInterval     time.Duration `alloy:"max_interval,attr,optional"`
	RetryOnStatus   []int         `alloy:"retry_on_status,attr,optional"`
}

// FlushArguments configures the (deprecated) flush settings. Retained for
// completeness; new configurations should use sending_queue.batch instead.
type FlushArguments struct {
	Bytes    int           `alloy:"bytes,attr,optional"`
	Interval time.Duration `alloy:"interval,attr,optional"`
}

// MappingArguments configures Elasticsearch document mapping modes.
type MappingArguments struct {
	Mode         string   `alloy:"mode,attr,optional"`
	AllowedModes []string `alloy:"allowed_modes,attr,optional"`
}

// LogstashFormatArguments configures Logstash-compatible index naming.
type LogstashFormatArguments struct {
	Enabled         bool   `alloy:"enabled,attr,optional"`
	PrefixSeparator string `alloy:"prefix_separator,attr,optional"`
	DateFormat      string `alloy:"date_format,attr,optional"`
}

// TelemetryArguments configures experimental telemetry/debug logging.
type TelemetryArguments struct {
	LogRequestBody              bool          `alloy:"log_request_body,attr,optional"`
	LogResponseBody             bool          `alloy:"log_response_body,attr,optional"`
	LogFailedDocsInput          bool          `alloy:"log_failed_docs_input,attr,optional"`
	LogFailedDocsInputRateLimit time.Duration `alloy:"log_failed_docs_input_rate_limit,attr,optional"`
}

// DynamicIndexArguments toggles dynamic-index document routing.
type DynamicIndexArguments struct {
	Enabled bool `alloy:"enabled,attr,optional"`
}

// DynamicIDArguments toggles using the elasticsearch.document_id attribute as
// the document ID.
type DynamicIDArguments struct {
	Enabled bool `alloy:"enabled,attr,optional"`
}

// DynamicPipelineArguments toggles using the elasticsearch.document_pipeline
// attribute as the ingest pipeline.
type DynamicPipelineArguments struct {
	Enabled bool `alloy:"enabled,attr,optional"`
}

var (
	_ syntax.Defaulter = (*ElasticsearchArguments)(nil)
	_ syntax.Validator = (*ElasticsearchArguments)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (args *ElasticsearchArguments) SetToDefault() {
	*args = ElasticsearchArguments{}
	args.Client.SetToDefault()
	args.SendingQueue.SetToDefault()
	args.Retry.SetToDefault()
	args.Mapping.SetToDefault()
	args.LogstashFormat.SetToDefault()
	args.Telemetry.SetToDefault()
	args.DebugMetrics.SetToDefault()

	// Match upstream defaults from createDefaultConfig:
	// queue size 10, BlockOnOverflow=true, batch flush 10s / min 1MB / max 5MB sizer bytes.
	args.SendingQueue.QueueSize = 10
	args.SendingQueue.BlockOnOverflow = true
	args.SendingQueue.Batch = &otelcol.BatchConfig{
		FlushTimeout: 10 * time.Second,
		MinSize:      1_000_000,
		MaxSize:      5_000_000,
		Sizer:        "bytes",
	}
}

// SetToDefault implements syntax.Defaulter.
func (args *ClientArguments) SetToDefault() {
	*args = ClientArguments{
		Timeout:           90 * time.Second,
		Compression:       otelcol.CompressionTypeGzip,
		Headers:           map[string]string{},
		ForceAttemptHTTP2: true,
	}
}

// SetToDefault implements syntax.Defaulter.
func (args *RetryArguments) SetToDefault() {
	*args = RetryArguments{
		Enabled:         true,
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     1 * time.Minute,
		RetryOnStatus:   []int{http.StatusTooManyRequests},
	}
}

// SetToDefault implements syntax.Defaulter.
func (args *MappingArguments) SetToDefault() {
	allModes := []string{
		elasticsearchexporter.MappingNone.String(),
		elasticsearchexporter.MappingECS.String(),
		elasticsearchexporter.MappingOTel.String(),
		elasticsearchexporter.MappingRaw.String(),
		elasticsearchexporter.MappingBodyMap.String(),
	}
	slices.Sort(allModes)
	*args = MappingArguments{
		Mode:         elasticsearchexporter.MappingOTel.String(),
		AllowedModes: allModes,
	}
}

// SetToDefault implements syntax.Defaulter.
func (args *LogstashFormatArguments) SetToDefault() {
	*args = LogstashFormatArguments{
		Enabled:         false,
		PrefixSeparator: "-",
		DateFormat:      "%Y.%m.%d",
	}
}

// SetToDefault implements syntax.Defaulter.
func (args *TelemetryArguments) SetToDefault() {
	*args = TelemetryArguments{
		LogFailedDocsInputRateLimit: time.Second,
	}
}

// Validate implements syntax.Validator.
func (args *ElasticsearchArguments) Validate() error {
	endpointSources := 0
	if args.Client.Endpoint != "" {
		endpointSources++
	}
	if len(args.Endpoints) > 0 {
		endpointSources++
	}
	if args.CloudID != "" {
		endpointSources++
	}
	if endpointSources == 0 {
		return errors.New("at least one of endpoints, cloudid, or client.endpoint must be specified")
	}
	if endpointSources > 1 {
		return errors.New("only one of endpoints, cloudid, or client.endpoint may be specified")
	}

	return nil
}

// Convert implements exporter.Arguments.
func (args ElasticsearchArguments) Convert() (otelcomponent.Config, error) {
	clientCfg, err := args.Client.Convert()
	if err != nil {
		return nil, err
	}

	q, err := args.SendingQueue.Convert()
	if err != nil {
		return nil, err
	}

	cfg := &elasticsearchexporter.Config{
		QueueBatchConfig: q,
		Endpoints:        args.Endpoints,
		CloudID:          args.CloudID,
		NumWorkers:       args.NumWorkers,

		LogsIndex:           args.LogsIndex,
		LogsDynamicIndex:    elasticsearchexporter.DynamicIndexSetting{Enabled: args.LogsDynamicIndex.Enabled},
		MetricsIndex:        args.MetricsIndex,
		MetricsDynamicIndex: elasticsearchexporter.DynamicIndexSetting{Enabled: args.MetricsDynamicIndex.Enabled},
		TracesIndex:         args.TracesIndex,
		TracesDynamicIndex:  elasticsearchexporter.DynamicIndexSetting{Enabled: args.TracesDynamicIndex.Enabled},

		LogsDynamicID:       elasticsearchexporter.DynamicIDSettings{Enabled: args.LogsDynamicID.Enabled},
		TracesDynamicID:     elasticsearchexporter.DynamicIDSettings{Enabled: args.TracesDynamicID.Enabled},
		LogsDynamicPipeline: elasticsearchexporter.DynamicPipelineSettings{Enabled: args.LogsDynamicPipeline.Enabled},

		Pipeline: args.Pipeline,

		ClientConfig: *clientCfg,
		Authentication: elasticsearchexporter.AuthenticationSettings{
			User:     args.Authentication.User,
			Password: configopaque.String(args.Authentication.Password),
			APIKey:   configopaque.String(args.Authentication.APIKey),
		},
		Discovery: elasticsearchexporter.DiscoverySettings{
			OnStart:  args.Discovery.OnStart,
			Interval: args.Discovery.Interval,
		},
		Retry: elasticsearchexporter.RetrySettings{
			Enabled:         args.Retry.Enabled,
			MaxRetries:      args.Retry.MaxRetries,
			InitialInterval: args.Retry.InitialInterval,
			MaxInterval:     args.Retry.MaxInterval,
			RetryOnStatus:   args.Retry.RetryOnStatus,
		},
		Flush: elasticsearchexporter.FlushSettings{
			Bytes:    args.Flush.Bytes,
			Interval: args.Flush.Interval,
		},
		Mapping: elasticsearchexporter.MappingsSettings{
			Mode:         args.Mapping.Mode,
			AllowedModes: args.Mapping.AllowedModes,
		},
		LogstashFormat: elasticsearchexporter.LogstashFormatSettings{
			Enabled:         args.LogstashFormat.Enabled,
			PrefixSeparator: args.LogstashFormat.PrefixSeparator,
			DateFormat:      args.LogstashFormat.DateFormat,
		},
		TelemetrySettings: elasticsearchexporter.TelemetrySettings{
			LogRequestBody:              args.Telemetry.LogRequestBody,
			LogResponseBody:             args.Telemetry.LogResponseBody,
			LogFailedDocsInput:          args.Telemetry.LogFailedDocsInput,
			LogFailedDocsInputRateLimit: args.Telemetry.LogFailedDocsInputRateLimit,
		},
		IncludeSourceOnError: args.IncludeSourceOnError,
		MetadataKeys:         args.MetadataKeys,
	}

	return cfg, nil
}

// Convert converts ClientArguments into the upstream confighttp.ClientConfig.
func (args *ClientArguments) Convert() (*otelconfighttp.ClientConfig, error) {
	hca := otelcol.HTTPClientArguments{
		Endpoint:             args.Endpoint,
		ProxyUrl:             args.ProxyURL,
		Compression:          args.Compression,
		CompressionParams:    args.CompressionParams,
		TLS:                  args.TLS,
		ReadBufferSize:       args.ReadBufferSize,
		WriteBufferSize:      args.WriteBufferSize,
		Timeout:              args.Timeout,
		Headers:              args.Headers,
		MaxIdleConns:         args.MaxIdleConns,
		MaxIdleConnsPerHost:  args.MaxIdleConnsPerHost,
		MaxConnsPerHost:      args.MaxConnsPerHost,
		IdleConnTimeout:      args.IdleConnTimeout,
		DisableKeepAlives:    args.DisableKeepAlives,
		HTTP2ReadIdleTimeout: args.HTTP2ReadIdleTimeout,
		HTTP2PingTimeout:     args.HTTP2PingTimeout,
		ForceAttemptHTTP2:    args.ForceAttemptHTTP2,
		Authentication:       args.Authentication,
		Cookies:              args.Cookies,
	}
	return hca.Convert()
}

// Extensions implements exporter.Arguments.
func (args ElasticsearchArguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	m := make(map[otelcomponent.ID]otelcomponent.Component)
	if ext := args.Client.Extensions(); ext != nil {
		for k, v := range ext {
			m[k] = v
		}
	}
	for k, v := range args.SendingQueue.Extensions() {
		m[k] = v
	}
	return m
}

// Extensions exposes auth extensions used by the client block.
func (args *ClientArguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	hca := otelcol.HTTPClientArguments{Authentication: args.Authentication}
	return hca.Extensions()
}

// Exporters implements exporter.Arguments.
func (args ElasticsearchArguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements exporter.Arguments.
func (args ElasticsearchArguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

