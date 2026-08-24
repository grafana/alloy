// Package azureblob provides an otelcol.exporter.azureblob component.
// Maintainers for the Grafana Alloy wrapper:
// - @nicholasgibson2
package azureblob

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.azureblob",
		Community: true,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := azureblobexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeSignalConstFunc(exporter.TypeAll))
		},
	})
}

// Arguments configures the otelcol.exporter.azureblob component.
type Arguments struct {
	Timeout time.Duration `alloy:"timeout,attr,optional"`

	Queue otelcol.QueueArguments `alloy:"sending_queue,block,optional"`
	Retry otelcol.RetryArguments `alloy:"retry_on_failure,block,optional"`

	URL            string             `alloy:"url,attr,optional"`
	Format         string             `alloy:"format,attr,optional"`
	Auth           Authentication     `alloy:"auth,block"`
	Container      TelemetryContainer `alloy:"container,block,optional"`
	BlobNameFormat BlobNameFormat     `alloy:"blob_name_format,block,optional"`
	AppendBlob     AppendBlob         `alloy:"append_blob,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var (
	_ exporter.Arguments = Arguments{}
	_ syntax.Defaulter   = (*Arguments)(nil)
	_ syntax.Validator   = (*Arguments)(nil)
)

func (args *Arguments) SetToDefault() {
	args.Timeout = 30 * time.Second
	args.Format = "json"
	args.Queue.SetToDefault()
	args.Retry.SetToDefault()
	args.DebugMetrics.SetToDefault()

	args.Auth.SetToDefault()
	args.Container.SetToDefault()
	args.BlobNameFormat.SetToDefault()
	args.AppendBlob.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	// Prevent upstream panic: azureblobexporter uses rand.IntN with this value,
	// which panics when n <= 0. Error early on invalid configuration.
	if args.BlobNameFormat.SerialNumEnabled && args.BlobNameFormat.SerialNumRange <= 0 {
		return fmt.Errorf("blob_name_format.serial_num_range must be > 0 when serial_num_enabled is true (got %d)", args.BlobNameFormat.SerialNumRange)
	}
	otelCfg, err := args.Convert()
	if err != nil {
		return err
	}
	azCfg := otelCfg.(*azureblobexporter.Config)
	return azCfg.Validate()
}

// Convert translates Alloy arguments into the upstream exporter config.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	var cfg azureblobexporter.Config

	cfg.URL = args.URL
	cfg.Auth = args.Auth.Convert()
	cfg.Container = args.Container.Convert()
	cfg.BlobNameFormat = args.BlobNameFormat.Convert()
	cfg.FormatType = args.Format
	cfg.AppendBlob = args.AppendBlob.Convert()
	cfg.TimeoutSettings = exporterhelper.TimeoutConfig{Timeout: args.Timeout}

	q, err := args.Queue.Convert()
	if err != nil {
		return nil, err
	}
	cfg.QueueSettings = q

	cfg.BackOffConfig = *args.Retry.Convert()

	return &cfg, nil
}

func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return args.Queue.Extensions()
}

func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// Authentication mirrors azureblobexporter.Authentication.
type Authentication struct {
	// Supported: connection_string, service_principal, system_managed_identity,
	// user_managed_identity, workload_identity
	Type               string `alloy:"type,attr,optional"`
	TenantID           string `alloy:"tenant_id,attr,optional"`
	ClientID           string `alloy:"client_id,attr,optional"`
	ClientSecret       string `alloy:"client_secret,attr,optional"`
	ConnectionString   string `alloy:"connection_string,attr,optional"`
	FederatedTokenFile string `alloy:"federated_token_file,attr,optional"`
}

func (a *Authentication) SetToDefault() {
	a.Type = string(azureblobexporter.ConnectionString)
}

func (a Authentication) Convert() azureblobexporter.Authentication {
	return azureblobexporter.Authentication{
		Type:               azureblobexporter.AuthType(a.Type),
		TenantID:           a.TenantID,
		ClientID:           a.ClientID,
		ClientSecret:       a.ClientSecret,
		ConnectionString:   a.ConnectionString,
		FederatedTokenFile: a.FederatedTokenFile,
	}
}

// TelemetryContainer mirrors azureblobexporter.TelemetryConfig.
type TelemetryContainer struct {
	Logs    string `alloy:"logs,attr,optional"`
	Metrics string `alloy:"metrics,attr,optional"`
	Traces  string `alloy:"traces,attr,optional"`
}

func (t *TelemetryContainer) SetToDefault() {
	*t = TelemetryContainer{
		Logs:    "logs",
		Metrics: "metrics",
		Traces:  "traces",
	}
}

func (t TelemetryContainer) Convert() azureblobexporter.TelemetryConfig {
	return azureblobexporter.TelemetryConfig{
		Logs:    t.Logs,
		Metrics: t.Metrics,
		Traces:  t.Traces,
	}
}

// BlobNameFormat mirrors azureblobexporter.BlobNameFormat.
type BlobNameFormat struct {
	MetricsFormat            string            `alloy:"metrics_format,attr,optional"`
	LogsFormat               string            `alloy:"logs_format,attr,optional"`
	TracesFormat             string            `alloy:"traces_format,attr,optional"`
	SerialNumEnabled         bool              `alloy:"serial_num_enabled,attr,optional"`
	SerialNumRange           int64             `alloy:"serial_num_range,attr,optional"`
	SerialNumBeforeExtension bool              `alloy:"serial_num_before_extension,attr,optional"`
	Timezone                 string            `alloy:"timezone,attr,optional"`
	TemplateEnabled          bool              `alloy:"template_enabled,attr,optional"`
	TimeParserEnabled        bool              `alloy:"time_parser_enabled,attr,optional"`
	TimeParserRanges         []string          `alloy:"time_parser_ranges,attr,optional"`
	Params                   map[string]string `alloy:"params,attr,optional"`
}

func (f *BlobNameFormat) SetToDefault() {
	*f = BlobNameFormat{
		MetricsFormat:     "2006/01/02/metrics_15_04_05.json",
		LogsFormat:        "2006/01/02/logs_15_04_05.json",
		TracesFormat:      "2006/01/02/traces_15_04_05.json",
		SerialNumEnabled:  true,
		SerialNumRange:    10000,
		Params:            map[string]string{},
		TemplateEnabled:   false,
		TimeParserEnabled: true,
		TimeParserRanges:  nil,
	}
}

func (f BlobNameFormat) Convert() azureblobexporter.BlobNameFormat {
	return azureblobexporter.BlobNameFormat{
		MetricsFormat:            f.MetricsFormat,
		LogsFormat:               f.LogsFormat,
		TracesFormat:             f.TracesFormat,
		SerialNumEnabled:         f.SerialNumEnabled,
		SerialNumRange:           f.SerialNumRange,
		SerialNumBeforeExtension: f.SerialNumBeforeExtension,
		Timezone:                 f.Timezone,
		TemplateEnabled:          f.TemplateEnabled,
		TimeParserEnabled:        f.TimeParserEnabled,
		TimeParserRanges:         f.TimeParserRanges,
		Params:                   f.Params,
	}
}

// AppendBlob mirrors azureblobexporter.AppendBlob.
type AppendBlob struct {
	Enabled   bool   `alloy:"enabled,attr,optional"`
	Separator string `alloy:"separator,attr,optional"`
}

func (a *AppendBlob) SetToDefault() {
	*a = AppendBlob{
		Enabled:   false,
		Separator: "\n",
	}
}

func (a AppendBlob) Convert() azureblobexporter.AppendBlob {
	return azureblobexporter.AppendBlob{
		Enabled:   a.Enabled,
		Separator: a.Separator,
	}
}
