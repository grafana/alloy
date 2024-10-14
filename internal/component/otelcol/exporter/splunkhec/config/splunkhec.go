package splunkhec_config

import (
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/splunkhecexporter"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/config/configtls"
	"go.opentelemetry.io/collector/exporter/exporterbatcher"
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
	MaxIdleConns *int `alloy:"max_idle_conns,attr,optional"`
	// MaxIdleConnsPerHost for the HTTP client.
	MaxIdleConnsPerHost *int `alloy:"max_idle_conns_per_host,attr,optional"`
	// MaxConnsPerHost for the HTTP client.
	MaxConnsPerHost *int `alloy:"max_conns_per_host,attr,optional"`
	// IdleConnTimeout for the HTTP client.
	IdleConnTimeout *time.Duration `alloy:"idle_conn_timeout,attr,optional"`
	// DisableKeepAlives for the HTTP client.
	DisableKeepAlives bool `alloy:"disable_keep_alives,attr,optional"`
	// TLSSetting for the HTTP client.
	InsecureSkipVerify bool `alloy:"insecure_skip_verify,attr,optional"`
}
type SplunkConf struct {
	// until https://github.com/open-telemetry/opentelemetry-collector/issues/8122 is resolved.
	BatcherConfig exporterbatcher.Config `mapstructure:"batcher"`

	// Experimental: This configuration is at the early stage of development and may change without backward compatibility
	// until https://github.com/open-telemetry/opentelemetry-collector/issues/8122 is resolved.
	// LogDataEnabled can be used to disable sending logs by the exporter.
	LogDataEnabled bool `alloy:"log_data_enabled,attr,optional"`
	// ProfilingDataEnabled can be used to disable sending profiling data by the exporter.
	ProfilingDataEnabled bool `alloy:"profiling_data_enabled,attr,optional"`
	// HEC Token is the authentication token provided by Splunk: https://docs.splunk.com/Documentation/Splunk/latest/Data/UsetheHTTPEventCollector.
	Token string `alloy:"token,alloytypes.Secret"`
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
	HecFields splunkhecexporter.OtelToHecFields `alloy:"otel_to_hec_fields,attr,optional"`
	// HealthPath for health API, default is '/services/collector/health'
	HealthPath string `alloy:"health_path,attr,optional"`
	// HecHealthCheckEnabled can be used to verify Splunk HEC health on exporter's startup
	HecHealthCheckEnabled bool `alloy:"health_check_enabled,attr,optional"`
	// ExportRaw to send only the log's body, targeting a Splunk HEC raw endpoint.
	ExportRaw bool `alloy:"export_raw,attr,optional"`
	// UseMultiMetricFormat combines metric events to save space during ingestion.
	UseMultiMetricFormat bool `alloy:"use_multi_metric_format,attr,optional"`
	// Heartbeat is the configuration to enable heartbeat
	Heartbeat splunkhecexporter.HecHeartbeat `alloy:"heartbeat,attr,optional"`
	// Telemetry is the configuration for splunk hec exporter telemetry
	Telemetry splunkhecexporter.HecTelemetry `alloy:"telemetry,attr,optional"`
}

// SplunkHecClientArguments defines the configuration for the Splunk HEC exporter.
type SplunkHecArguments struct {
	SplunkHecClientArguments SplunkHecClientArguments `alloy:"client,block"`
	//	QueueSettings           exporterhelper.QueueSettings `alloy:"queue,block,optional"`
	//configretry.BackOffConfig `alloy:"retry_on_failure,attribute,optional"`
	Splunk SplunkConf `alloy:"splunk,block"`
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
}

func (args *SplunkConf) SetToDefault() {
	// args.BatcherConfig.SetToDefault()
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
	args.Heartbeat = splunkhecexporter.HecHeartbeat{}
	args.Telemetry = splunkhecexporter.HecTelemetry{}
}

// Convert converts args into the upstream type
func (args *SplunkHecArguments) Convert() *splunkhecexporter.Config {
	if args == nil {
		return nil
	}
	return &splunkhecexporter.Config{
		ClientConfig:            *args.SplunkHecClientArguments.Convert(),
		QueueSettings:           exporterhelper.NewDefaultQueueSettings(),
		BackOffConfig:           configretry.NewDefaultBackOffConfig(),
		BatcherConfig:           args.Splunk.BatcherConfig,
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
		HecFields:               args.Splunk.HecFields,
		HealthPath:              args.Splunk.HealthPath,
		HecHealthCheckEnabled:   args.Splunk.HecHealthCheckEnabled,
		ExportRaw:               args.Splunk.ExportRaw,
		UseMultiMetricFormat:    args.Splunk.UseMultiMetricFormat,
		Heartbeat:               args.Splunk.Heartbeat,
		Telemetry:               args.Splunk.Telemetry,
	}
}
