// Package otlp provides an otelcol.exporter.otlp component.
package otlp

import (
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/exporter"
	"github.com/grafana/agent/internal/featuregate"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelpexporterhelper "go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.otlp",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := otlpexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeAll)
		},
	})
}

// Arguments configures the otelcol.exporter.otlp component.
type Arguments struct {
	Timeout time.Duration `river:"timeout,attr,optional"`

	Queue otelcol.QueueArguments `river:"sending_queue,block,optional"`
	Retry otelcol.RetryArguments `river:"retry_on_failure,block,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcol.DebugMetricsArguments `river:"debug_metrics,block,optional"`

	Client GRPCClientArguments `river:"client,block"`
}

var _ exporter.Arguments = Arguments{}

// DefaultArguments holds default values for Arguments.
var DefaultArguments = Arguments{
	Timeout:      otelcol.DefaultTimeout,
	Queue:        otelcol.DefaultQueueArguments,
	Retry:        otelcol.DefaultRetryArguments,
	Client:       DefaultGRPCClientArguments,
	DebugMetrics: otelcol.DefaultDebugMetricsArguments,
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &otlpexporter.Config{
		TimeoutSettings: otelpexporterhelper.TimeoutSettings{
			Timeout: args.Timeout,
		},
		QueueSettings:      *args.Queue.Convert(),
		RetrySettings:      *args.Retry.Convert(),
		GRPCClientSettings: *(*otelcol.GRPCClientArguments)(&args.Client).Convert(),
	}, nil
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return (*otelcol.GRPCClientArguments)(&args.Client).Extensions()
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcol.DebugMetricsArguments {
	return args.DebugMetrics
}

// GRPCClientArguments is used to configure otelcol.exporter.otlp with
// component-specific defaults.
type GRPCClientArguments otelcol.GRPCClientArguments

// DefaultGRPCClientArguments holds component-specific default settings for
// GRPCClientArguments.
var DefaultGRPCClientArguments = GRPCClientArguments{
	Headers:         map[string]string{},
	Compression:     otelcol.CompressionTypeGzip,
	WriteBufferSize: 512 * 1024,
	BalancerName:    otelcol.DefaultBalancerName,
}

// SetToDefault implements river.Defaulter.
func (args *GRPCClientArguments) SetToDefault() {
	*args = DefaultGRPCClientArguments
}
