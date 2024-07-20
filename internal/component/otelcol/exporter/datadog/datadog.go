// Package otlphttp provides an otelcol.exporter.otlphttp component.

package datadog

import (
	"errors"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/datadogexporter"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.datadog",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := datadogexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeAll)
		},
	})
}

// Arguments configures the otelcol.exporter.datadog component.
type Arguments struct {
	Client  HTTPClientArguments    `alloy:"client,block,optional"`
	Timeout time.Duration          `alloy:"timeout,attr,optional"`
	Queue   otelcol.QueueArguments `alloy:"sending_queue,block,optional"`
	Retry   otelcol.RetryArguments `alloy:"retry_on_failure,block,optional"`

	// Datadog specific configuration settings
	APISettings  otelcol.DatadogAPIArguments          `alloy:"api,block"`
	Traces       otelcol.DatadogTracesArguments       `alloy:"traces,block,optional"`
	Metrics      otelcol.DatadogMetricsArguments      `alloy:"metrics,block,optional"`
	HostMetadata otelcol.DatadogHostMetadataArguments `alloy:"host_metadata,block,optional"`
	OnlyMetadata bool                                 `alloy:"only_metadata,attr,optional"`
	Hostname     string                               `alloy:"hostname,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// DatadogAPISettings holds the configuration settings for the Datadog API.

var _ exporter.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.Queue.SetToDefault()
	args.Retry.SetToDefault()
	args.Client.SetToDefault()

	args.APISettings.SetToDefault()
	args.Traces.SetToDefault()
	args.Metrics.SetToDefault()
	args.HostMetadata.SetToDefault()

	args.DebugMetrics.SetToDefault()
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &datadogexporter.Config{
		ClientConfig:  *(*otelcol.HTTPClientArguments)(&args.Client).Convert(),
		QueueSettings: *args.Queue.Convert(),
		BackOffConfig: *args.Retry.Convert(),
		TagsConfig: datadogexporter.TagsConfig{
			Hostname: args.Hostname},
		API:          *args.APISettings.Convert(),
		Traces:       *args.Traces.Convert(),
		Metrics:      *args.Metrics.Convert(),
		HostMetadata: *args.HostMetadata.Convert(),
		OnlyMetadata: args.OnlyMetadata,
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

func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// Validate implements syntax.Validator.
// Checks taken from upstream
func (args *Arguments) Validate() error {
	if args.APISettings.Key == "" {
		return errors.New("missing API key")
	}

	return nil
}

// HTTPClientArguments is used to configure otelcol.exporter.otlphttp with
// component-specific defaults.
type HTTPClientArguments otelcol.HTTPClientArguments

// SetToDefault implements syntax.Defaulter.
func (args *HTTPClientArguments) SetToDefault() {
	*args = HTTPClientArguments{
		Timeout:         5 * time.Second,
		Headers:         map[string]string{},
		Compression:     otelcol.CompressionTypeGzip,
		ReadBufferSize:  0,
		WriteBufferSize: 512 * 1024,
	}
}
