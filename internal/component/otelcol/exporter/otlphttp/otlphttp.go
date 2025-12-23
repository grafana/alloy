// Package otlphttp provides an otelcol.exporter.otlphttp component.
package otlphttp

import (
	"errors"
	"fmt"
	"maps"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/exporter/otlphttpexporter"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.otlphttp",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := otlphttpexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeSignalConstFunc(exporter.TypeAll))
		},
	})
}

// Arguments configures the otelcol.exporter.otlphttp component.
type Arguments struct {
	Client HTTPClientArguments    `alloy:"client,block"`
	Queue  otelcol.QueueArguments `alloy:"sending_queue,block,optional"`
	Retry  otelcol.RetryArguments `alloy:"retry_on_failure,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	// The URLs to send metrics/logs/traces to. If omitted the exporter will
	// use Client.Endpoint by appending "/v1/metrics", "/v1/logs" or
	// "/v1/traces", respectively. If set, these settings override
	// Client.Endpoint for the corresponding signal.
	TracesEndpoint  string `alloy:"traces_endpoint,attr,optional"`
	MetricsEndpoint string `alloy:"metrics_endpoint,attr,optional"`
	LogsEndpoint    string `alloy:"logs_endpoint,attr,optional"`

	Encoding string `alloy:"encoding,attr,optional"`
}

var _ exporter.Arguments = Arguments{}

const (
	EncodingProto string = "proto"
	EncodingJSON  string = "json"
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Encoding: EncodingProto,
	}
	args.Queue.SetToDefault()
	args.Retry.SetToDefault()
	args.Client.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	httpClientArgs := *(*otelcol.HTTPClientArguments)(&args.Client)
	convertedClientArgs, err := httpClientArgs.Convert()
	if err != nil {
		return nil, err
	}
	q, err := args.Queue.Convert()
	if err != nil {
		return nil, err
	}
	return &otlphttpexporter.Config{
		ClientConfig:    *convertedClientArgs,
		QueueConfig:     q,
		RetryConfig:     *args.Retry.Convert(),
		TracesEndpoint:  args.TracesEndpoint,
		MetricsEndpoint: args.MetricsEndpoint,
		LogsEndpoint:    args.LogsEndpoint,
		Encoding:        otlphttpexporter.EncodingType(args.Encoding),
	}, nil
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	ext := (*otelcol.HTTPClientArguments)(&args.Client).Extensions()
	maps.Copy(ext, args.Queue.Extensions())
	return ext
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements exporter.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Client.Endpoint == "" && args.TracesEndpoint == "" && args.MetricsEndpoint == "" && args.LogsEndpoint == "" {
		return errors.New("at least one endpoint must be specified")
	}
	if args.Encoding != EncodingProto && args.Encoding != EncodingJSON {
		return fmt.Errorf("invalid encoding type %s", args.Encoding)
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

// SetToDefault implements syntax.Defaulter.
func (args *HTTPClientArguments) SetToDefault() {
	maxIdleConns := DefaultMaxIdleConns
	idleConnTimeout := DefaultIdleConnTimeout
	*args = HTTPClientArguments{
		MaxIdleConns:    maxIdleConns,
		IdleConnTimeout: idleConnTimeout,

		Timeout:           30 * time.Second,
		Headers:           map[string]string{},
		Compression:       otelcol.CompressionTypeGzip,
		ReadBufferSize:    0,
		WriteBufferSize:   512 * 1024,
		HTTP2PingTimeout:  15 * time.Second,
		ForceAttemptHTTP2: true,
	}
}
