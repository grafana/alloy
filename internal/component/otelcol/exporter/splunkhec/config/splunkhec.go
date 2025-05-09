package splunkhec_config

import (
	"errors"
	"time"

	"github.com/grafana/alloy/syntax/alloytypes"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

type SplunkHecClientArguments struct {
	// Endpoint is the Splunk HEC endpoint to send data to.
	Endpoint string `alloy:"endpoint,attr"`
	// ReadBufferSize for the HTTP client.
	ReadBufferSize int `alloy:"read_buffer_size,attr,optional"`
	// WriteBufferSize for the HTTP client.
	WriteBufferSize int `alloy:"write_buffer_size,attr,optional"`
	// Timeout for the HTTP client. Defaults to 15 seconds.
	Timeout time.Duration `alloy:"timeout,attr,optional"`
	// MaxIdleConns for the HTTP client.
	MaxIdleConns int `alloy:"max_idle_conns,attr,optional"`
	// MaxIdleConnsPerHost for the HTTP client.
	MaxIdleConnsPerHost int `alloy:"max_idle_conns_per_host,attr,optional"`
	// MaxConnsPerHost for the HTTP client.
	MaxConnsPerHost int `alloy:"max_conns_per_host,attr,optional"`
	// IdleConnTimeout for the HTTP client.
	IdleConnTimeout time.Duration `alloy:"idle_conn_timeout,attr,optional"`
	// DisableKeepAlives for the HTTP client.
	DisableKeepAlives bool `alloy:"disable_keep_alives,attr,optional"`
	// TLSSetting for the HTTP client.
	InsecureSkipVerify bool `alloy:"insecure_skip_verify,attr,optional"`
}
type SplunkConf struct {
	// until https://github.com/open-telemetry/opentelemetry-collector/issues/8122 is resolved.
	BatcherConfig BatcherConfig `alloy:"batcher,block,optional"`

	// Experimental: This configuration is at the early stage of development and may change without backward compatibility
	// until https://github.com/open-telemetry/opentelemetry-collector/issues/8122 is resolved.
	// LogDataEnabled can be used to disable sending logs by the exporter.
	LogDataEnabled bool `alloy:"log_data_enabled,attr,optional"`
	// ProfilingDataEnabled can be used to disable sending profiling data by the exporter.
	ProfilingDataEnabled bool `alloy:"profiling_data_enabled,attr,optional"`
	// HEC Token is the authentication token provided by Splunk: https://docs.splunk.com/Documentation/Splunk/latest/Data/UsetheHTTPEventCollector.
	Token alloytypes.Secret `alloy:"token,attr"`
	// Optional Splunk source: https://docs.splunk.com/Splexicon:Source.
	// Sources identify the incoming data.
	Source string `alloy:"source,attr,optional"`
	// Optional Splunk source type: https://docs.splunk.com/Splexicon:Sourcetype.
	SourceType string `alloy:"sourcetype,attr,optional"`
	// Splunk index, optional name of the Splunk index.
	Index string `alloy:"index,attr,optional"`
	// Disable GZip compression. Defaults to false.
	DisableCompression bool `alloy:"disable_compression,attr,optional"`
	// Maximum log payload size in bytes. Default value is 2097152 bytes (2MiB).
	// Maximum allowed value is 838860800 (~ 800 MB).
	MaxContentLengthLogs uint `alloy:"max_content_length_logs,attr,optional"`
	// Maximum metric payload size in bytes. Default value is 2097152 bytes (2MiB).
	// Maximum allowed value is 838860800 (~ 800 MB).
	MaxContentLengthMetrics uint `alloy:"max_content_length_metrics,attr,optional"`
	// Maximum trace payload size in bytes. Default value is 2097152 bytes (2MiB).
	// Maximum allowed value is 838860800 (~ 800 MB).
	MaxContentLengthTraces uint `alloy:"max_content_length_traces,attr,optional"`
	// Maximum payload size, raw uncompressed. Default value is 5242880 bytes (5MiB).
	// Maximum allowed value is 838860800 (~ 800 MB).
	MaxEventSize uint `alloy:"max_event_size,attr,optional"`
	// App name is used to track telemetry information for Splunk App's using HEC by App name. Defaults to "OpenTelemetry Collector Contrib".
	SplunkAppName string `alloy:"splunk_app_name,attr,optional"`
	// App version is used to track telemetry information for Splunk App's using HEC by App version. Defaults to the current OpenTelemetry Collector Contrib build version.
	SplunkAppVersion string `alloy:"splunk_app_version,attr,optional"`
	// HecFields creates a mapping from attributes to HEC fields.
	HecFields HecFields `alloy:"otel_to_hec_fields,block,optional"`
	// HealthPath for health API, default is '/services/collector/health'
	HealthPath string `alloy:"health_path,attr,optional"`
	// HecHealthCheckEnabled can be used to verify Splunk HEC health on exporter's startup
	HecHealthCheckEnabled bool `alloy:"health_check_enabled,attr,optional"`
	// ExportRaw to send only the log's body, targeting a Splunk HEC raw endpoint.
	ExportRaw bool `alloy:"export_raw,attr,optional"`
	// UseMultiMetricFormat combines metric events to save space during ingestion.
	UseMultiMetricFormat bool `alloy:"use_multi_metric_format,attr,optional"`
	// Heartbeat is the configuration to enable heartbeat
	Heartbeat SplunkHecHeartbeat `alloy:"heartbeat,block,optional"`
	// Telemetry is the configuration for splunk hec exporter telemetry
	Telemetry SplunkHecTelemetry `alloy:"telemetry,block,optional"`
}

type BatcherConfig struct {
	// Enabled indicates whether to not enqueue batches before sending to the consumerSender.
	Enabled bool `alloy:"enabled,attr,optional"`

	// FlushTimeout sets the time after which a batch will be sent regardless of its size.
	FlushTimeout time.Duration `alloy:"flush_timeout,attr,optional"`

	MinSize int64  `alloy:"min_size,attr,optional"`
	MaxSize int64  `alloy:"max_size,attr,optional"`
	Sizer   string `alloy:"sizer,attr,optional"`
}

func (args *BatcherConfig) Convert() *exporterhelper.BatcherConfig {
	if args == nil {
		return nil
	}
	sizer := exporterhelper.RequestSizerType{}
	// ignore error here because we check for valid sizer in Validate()
	_ = sizer.UnmarshalText([]byte(args.Sizer))
	return &exporterhelper.BatcherConfig{
		Enabled:      args.Enabled,
		FlushTimeout: args.FlushTimeout,
		SizeConfig: exporterhelper.SizeConfig{
			Sizer:   sizer,
			MinSize: args.MinSize,
			MaxSize: args.MaxSize,
		},
	}
}

type HecFields struct {
	// SeverityText informs the exporter to map the severity text field to a specific HEC field.
	SeverityText string `alloy:"severity_text,attr,optional"`
	// SeverityNumber informs the exporter to map the severity number field to a specific HEC field.
	SeverityNumber string `alloy:"severity_number,attr,optional"`
}

func (args *HecFields) Convert() *splunkhecexporter.OtelToHecFields {
	if args == nil {
		return nil
	}
	return &splunkhecexporter.OtelToHecFields{
		SeverityText:   args.SeverityText,
		SeverityNumber: args.SeverityNumber,
	}
}

// SplunkHecHeartbeat defines the configuration for the Splunk HEC exporter heartbeat.
type SplunkHecHeartbeat struct {
	// Interval represents the time interval for the heartbeat interval. If nothing or 0 is set,
	// heartbeat is not enabled.
	// A heartbeat is an event sent to _internal index with metadata for the current collector/host.
	// In seconds
	Interval time.Duration `alloy:"interval,attr,optional"`

	// Startup is used to send heartbeat events on exporter's startup.
	Startup bool `alloy:"startup,attr,optional"`
}

func (args *SplunkHecHeartbeat) Convert() *splunkhecexporter.HecHeartbeat {
	if args == nil {
		return nil
	}
	return &splunkhecexporter.HecHeartbeat{
		Interval: args.Interval,
		Startup:  args.Startup,
	}
}

// SplunkHecTelemetry defines the configuration for the Splunk HEC exporter internal telemetry.
type SplunkHecTelemetry struct {
	// Enabled can be used to disable sending telemetry data by the exporter.
	Enabled bool `alloy:"enabled,attr,optional"`
	// Override metrics names for telemetry.
	OverrideMetricsNames map[string]string `alloy:"override_metrics_names,attr,optional"`
	// extra attributes to be added to telemetry data.
	ExtraAttributes map[string]string `alloy:"extra_attributes,attr,optional"`
}

func (args *SplunkHecTelemetry) Convert() *splunkhecexporter.HecTelemetry {
	if args == nil {
		return nil
	}
	return &splunkhecexporter.HecTelemetry{
		Enabled:              args.Enabled,
		OverrideMetricsNames: args.OverrideMetricsNames,
		ExtraAttributes:      args.ExtraAttributes,
	}
}

// SplunkHecClientArguments defines the configuration for the Splunk HEC exporter.
type SplunkHecArguments struct {
	SplunkHecClientArguments SplunkHecClientArguments   `alloy:"client,block"`
	QueueSettings            exporterhelper.QueueConfig `alloy:"queue,block,optional"`
	RetrySettings            configretry.BackOffConfig  `alloy:"retry_on_failure,block,optional"`
	Splunk                   SplunkConf                 `alloy:"splunk,block"`
}

func (args *SplunkHecClientArguments) Convert() *confighttp.ClientConfig {
	if args == nil {
		return nil
	}
	return &confighttp.ClientConfig{
		Endpoint:            args.Endpoint,
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

func (args *SplunkHecClientArguments) SetToDefault() {
	args.Timeout = 15 * time.Second
	args.MaxIdleConns = 100
	args.IdleConnTimeout = 90 * time.Second
}

func (args *SplunkHecClientArguments) Validate() error {
	if args.Endpoint == "" {
		return errors.New("missing Splunk hec endpoint")
	}
	return nil
}

func (args *SplunkConf) SetToDefault() {
	args.BatcherConfig = BatcherConfig{
		Enabled:      false,
		FlushTimeout: 200 * time.Millisecond,
		MinSize:      8192,
		MaxSize:      0,
		Sizer:        "items",
	}
	args.LogDataEnabled = true
	args.ProfilingDataEnabled = true
	args.Source = ""
	args.SourceType = ""
	args.Index = ""
	args.DisableCompression = false
	args.MaxContentLengthLogs = 2097152
	args.MaxContentLengthMetrics = 2097152
	args.MaxContentLengthTraces = 2097152
	args.MaxEventSize = 5242880
	args.SplunkAppName = "Alloy"
	args.SplunkAppVersion = ""
	args.HealthPath = "/services/collector/health"
	args.HecHealthCheckEnabled = false
	args.ExportRaw = false
	args.UseMultiMetricFormat = false
	args.Heartbeat = SplunkHecHeartbeat{}
	args.Telemetry = SplunkHecTelemetry{}
}

func (args *SplunkConf) Validate() error {
	if args.Token == "" {
		return errors.New("missing splunk token")
	}
	if !args.LogDataEnabled && !args.ProfilingDataEnabled {
		return errors.New("at least one of log_data_enabled or profiling_data_enabled must be enabled")
	}
	if args.MaxContentLengthLogs > 838860800 {
		return errors.New("max_content_length_logs must be less than 838860800")
	}
	if args.MaxContentLengthMetrics > 838860800 {
		return errors.New("max_content_length_metrics must be less than 838860800")
	}
	if args.MaxContentLengthTraces > 838860800 {
		return errors.New("max_content_length_traces must be less than 838860800")
	}
	if args.BatcherConfig.Sizer != "items" && args.BatcherConfig.Sizer != "bytes" && args.BatcherConfig.Sizer != "requests" {
		return errors.New("sizer must be one of items, bytes, or requests")
	}

	return nil
}

// Convert converts args into the upstream type
func (args *SplunkHecArguments) Convert() *splunkhecexporter.Config {
	if args == nil {
		return nil
	}
	return &splunkhecexporter.Config{
		ClientConfig:            *args.SplunkHecClientArguments.Convert(),
		QueueSettings:           args.QueueSettings,
		BackOffConfig:           args.RetrySettings,
		BatcherConfig:           *args.Splunk.BatcherConfig.Convert(),
		LogDataEnabled:          args.Splunk.LogDataEnabled,
		ProfilingDataEnabled:    args.Splunk.ProfilingDataEnabled,
		Token:                   configopaque.String(args.Splunk.Token),
		Source:                  args.Splunk.Source,
		SourceType:              args.Splunk.SourceType,
		Index:                   args.Splunk.Index,
		DisableCompression:      args.Splunk.DisableCompression,
		MaxContentLengthLogs:    args.Splunk.MaxContentLengthLogs,
		MaxContentLengthMetrics: args.Splunk.MaxContentLengthMetrics,
		MaxContentLengthTraces:  args.Splunk.MaxContentLengthTraces,
		MaxEventSize:            args.Splunk.MaxEventSize,
		SplunkAppName:           args.Splunk.SplunkAppName,
		SplunkAppVersion:        args.Splunk.SplunkAppVersion,
		HecFields:               *args.Splunk.HecFields.Convert(),
		HealthPath:              args.Splunk.HealthPath,
		HecHealthCheckEnabled:   args.Splunk.HecHealthCheckEnabled,
		ExportRaw:               args.Splunk.ExportRaw,
		UseMultiMetricFormat:    args.Splunk.UseMultiMetricFormat,
		Heartbeat:               *args.Splunk.Heartbeat.Convert(),
		Telemetry:               *args.Splunk.Telemetry.Convert(),
	}
}

func (args *SplunkHecArguments) SetToDefault() {
	args.SplunkHecClientArguments.SetToDefault()
	args.QueueSettings = exporterhelper.NewDefaultQueueConfig()
	args.RetrySettings = configretry.NewDefaultBackOffConfig()
	args.Splunk.SetToDefault()
}
