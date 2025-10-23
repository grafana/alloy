// Package azureblob provides an otelcol.exporter.azureblob component.
package azureblob

import (
	"fmt"
	"strings"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/azureblobexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.azureblob",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := azureblobexporter.NewFactory()

			typeSignalFunc := func(_ component.Options, args component.Arguments) exporter.TypeSignal {
				switch args.(Arguments).MarshalerName.Type {
				case "sumo_ic":
					return exporter.TypeLogs
				default:
					return exporter.TypeAll
				}
			}

			return exporter.New(opts, fact, args.(Arguments), typeSignalFunc)
		},
	})
}

// Arguments configures the otelcol.exporter.azureblob component.
type Arguments struct {
	Queue otelcol.QueueArguments `alloy:"sending_queue,block,optional"`
	Retry otelcol.RetryArguments `alloy:"retry_on_failure,block,optional"`

	BlobUploader  BlobUploader  `alloy:"blob_uploader,block"`
	MarshalerName MarshalerType `alloy:"marshaler,block,optional"`
	AppendBlob    AppendBlob    `alloy:"append_blob,block,optional"`
	Encodings     Encodings     `alloy:"encodings,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var _ exporter.Arguments = Arguments{}

func (args *Arguments) SetToDefault() {
	args.Queue.SetToDefault()
	args.Retry.SetToDefault()
	args.DebugMetrics.SetToDefault()

	args.MarshalerName.SetToDefault()
	args.BlobUploader.SetToDefault()
	args.AppendBlob.SetToDefault()
	args.Encodings.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	// Prevent upstream panic: azureblobexporter uses rand.IntN with this value,
	// which panics when n <= 0. Error early on invalid configuration.
	if args.BlobUploader.BlobNameFormat.SerialNumRange <= 0 {
		return fmt.Errorf("blob_uploader.blob_name_format.serial_num_range must be > 0 (got %d)", args.BlobUploader.BlobNameFormat.SerialNumRange)
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

	cfg.URL = args.BlobUploader.URL
	cfg.Auth = args.BlobUploader.Auth.Convert()
	cfg.Container = args.BlobUploader.Container.Convert()
	cfg.BlobNameFormat = args.BlobUploader.BlobNameFormat.Convert()
	cfg.FormatType = args.MarshalerName.Convert()
	cfg.AppendBlob = args.AppendBlob.Convert()
	enc, err := args.Encodings.Convert()
	if err != nil {
		return nil, err
	}
	cfg.Encodings = enc
	r := args.Retry.Convert()
	if r != nil {
		cfg.BackOffConfig = *r
	}

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

// BlobUploader collects Azure Storage-specific settings.
type BlobUploader struct {
	URL            string             `alloy:"url,attr,optional"`
	Auth           Authentication     `alloy:"auth,block"`
	Container      TelemetryContainer `alloy:"container,block,optional"`
	BlobNameFormat BlobNameFormat     `alloy:"blob_name_format,block,optional"`
}

func (b *BlobUploader) SetToDefault() {
	b.Auth.SetToDefault()
	b.Container.SetToDefault()
	b.BlobNameFormat.SetToDefault()
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
	// Safe default to match common usage.
	if a.Type == "" {
		a.Type = "connection_string"
	}
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
	if t.Logs == "" {
		t.Logs = "logs"
	}
	if t.Metrics == "" {
		t.Metrics = "metrics"
	}
	if t.Traces == "" {
		t.Traces = "traces"
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
	SerialNumRange           int64             `alloy:"serial_num_range,attr,optional"`
	SerialNumBeforeExtension bool              `alloy:"serial_num_before_extension,attr,optional"`
	Params                   map[string]string `alloy:"params,attr,optional"`
}

func (f *BlobNameFormat) SetToDefault() {
	if f.MetricsFormat == "" {
		f.MetricsFormat = "2006/01/02/metrics_15_04_05.json"
	}
	if f.LogsFormat == "" {
		f.LogsFormat = "2006/01/02/logs_15_04_05.json"
	}
	if f.TracesFormat == "" {
		f.TracesFormat = "2006/01/02/traces_15_04_05.json"
	}
	if f.SerialNumRange == 0 {
		f.SerialNumRange = 10000
	}
	// SerialNumBeforeExtension defaults to false.
}

func (f BlobNameFormat) Convert() azureblobexporter.BlobNameFormat {
	return azureblobexporter.BlobNameFormat{
		MetricsFormat:            f.MetricsFormat,
		LogsFormat:               f.LogsFormat,
		TracesFormat:             f.TracesFormat,
		SerialNumRange:           f.SerialNumRange,
		SerialNumBeforeExtension: f.SerialNumBeforeExtension,
		Params:                   f.Params,
	}
}

// AppendBlob mirrors azureblobexporter.AppendBlob.
type AppendBlob struct {
	Enabled   bool   `alloy:"enabled,attr,optional"`
	Separator string `alloy:"separator,attr,optional"`
}

func (a *AppendBlob) SetToDefault() {
	if a.Separator == "" {
		a.Separator = "\n"
	}
}

func (a AppendBlob) Convert() azureblobexporter.AppendBlob {
	return azureblobexporter.AppendBlob{
		Enabled:   a.Enabled,
		Separator: a.Separator,
	}
}

// Encodings mirrors azureblobexporter.Encodings.
type Encodings struct {
	Logs    string `alloy:"logs,attr,optional"`
	Metrics string `alloy:"metrics,attr,optional"`
	Traces  string `alloy:"traces,attr,optional"`
}

func (e *Encodings) SetToDefault() {}

func (e Encodings) Convert() (azureblobexporter.Encodings, error) {
	var out azureblobexporter.Encodings
	if e.Logs != "" {
		id, err := parseComponentID(e.Logs)
		if err != nil {
			return out, err
		}
		out.Logs = &id
	}
	if e.Metrics != "" {
		id, err := parseComponentID(e.Metrics)
		if err != nil {
			return out, err
		}
		out.Metrics = &id
	}
	if e.Traces != "" {
		id, err := parseComponentID(e.Traces)
		if err != nil {
			return out, err
		}
		out.Traces = &id
	}
	return out, nil
}

func parseComponentID(s string) (otelcomponent.ID, error) {
	// Accept "type" or "type/name".
	if s == "" {
		return otelcomponent.ID{}, fmt.Errorf("empty component id")
	}
	parts := strings.SplitN(s, "/", 2)
	if len(parts) == 1 {
		return otelcomponent.NewID(otelcomponent.MustNewType(parts[0])), nil
	}
	if parts[0] == "" || parts[1] == "" {
		return otelcomponent.ID{}, fmt.Errorf("invalid component id %q: want type or type/name", s)
	}
	return otelcomponent.NewIDWithName(otelcomponent.MustNewType(parts[0]), parts[1]), nil
}

// MarshalerType maps to exporter format.
type MarshalerType struct {
	Type string `alloy:"type,attr,optional"`
}

func (m *MarshalerType) SetToDefault() {
	if m.Type == "" {
		m.Type = "otlp_json"
	}
}

func (m MarshalerType) Convert() string {
	switch m.Type {
	case "", "json", "otlp_json", "sumo_ic":
		return "json"
	case "proto", "otlp_proto":
		return "proto"
	default:
		// Pass-through to allow future values; upstream will validate.
		return m.Type
	}
}
