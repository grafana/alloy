// Package otlphttp provides an otelcol.exporter.otlphttp component.
package otlphttp

import (
	"errors"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/exporter"
	"github.com/grafana/agent/internal/featuregate"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.otlphttp",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := otlphttpexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeAll)
		},
	})
}

// Arguments configures the otelcol.exporter.otlphttp component.
type Arguments struct {
	Client HTTPClientArguments    `river:"client,block"`
	Queue  otelcol.QueueArguments `river:"sending_queue,block,optional"`
	Retry  otelcol.RetryArguments `river:"retry_on_failure,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcol.DebugMetricsArguments `river:"debug_metrics,block,optional"`

	// The URLs to send metrics/logs/traces to. If omitted the exporter will
	// use Client.Endpoint by appending "/v1/metrics", "/v1/logs" or
	// "/v1/traces", respectively. If set, these settings override
	// Client.Endpoint for the corresponding signal.
	TracesEndpoint  string `river:"traces_endpoint,attr,optional"`
	MetricsEndpoint string `river:"metrics_endpoint,attr,optional"`
	LogsEndpoint    string `river:"logs_endpoint,attr,optional"`
}

var _ exporter.Arguments = Arguments{}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{}
	args.Queue.SetToDefault()
	args.Retry.SetToDefault()
	args.Client.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &otlphttpexporter.Config{
		HTTPClientSettings: *(*otelcol.HTTPClientArguments)(&args.Client).Convert(),
		QueueSettings:      *args.Queue.Convert(),
		RetrySettings:      *args.Retry.Convert(),
		TracesEndpoint:     args.TracesEndpoint,
		MetricsEndpoint:    args.MetricsEndpoint,
		LogsEndpoint:       args.LogsEndpoint,
	}, nil
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return (*otelcol.HTTPClientArguments)(&args.Client).Extensions()
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcol.DebugMetricsArguments {
	return args.DebugMetrics
}

// Validate implements river.Validator.
func (args *Arguments) Validate() error {
	if args.Client.Endpoint == "" && args.TracesEndpoint == "" && args.MetricsEndpoint == "" && args.LogsEndpoint == "" {
		return errors.New("at least one endpoint must be specified")
	}
	return nil
}

// HTTPClientArguments is used to configure otelcol.exporter.otlphttp with
// component-specific defaults.
type HTTPClientArguments otelcol.HTTPClientArguments

// Default server settings.
var (
	DefaultMaxIdleConns    = 100
	DefaultIdleConnTimeout = 90 * time.Second
)

// SetToDefault implements river.Defaulter.
func (args *HTTPClientArguments) SetToDefault() {
	maxIdleConns := DefaultMaxIdleConns
	idleConnTimeout := DefaultIdleConnTimeout
	*args = HTTPClientArguments{
		MaxIdleConns:    &maxIdleConns,
		IdleConnTimeout: &idleConnTimeout,

		Timeout:         30 * time.Second,
		Headers:         map[string]string{},
		Compression:     otelcol.CompressionTypeGzip,
		ReadBufferSize:  0,
		WriteBufferSize: 512 * 1024,
	}
}
