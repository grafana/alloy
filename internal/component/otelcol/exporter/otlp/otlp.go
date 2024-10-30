// Package otlp provides an otelcol.exporter.otlp component.
package otlp

import (
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelpexporterhelper "go.opentelemetry.io/collector/exporter/exporterhelper"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	otelextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.otlp",
		Stability: featuregate.StabilityGenerallyAvailable,
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
	Timeout time.Duration `alloy:"timeout,attr,optional"`

	Queue otelcol.QueueArguments `alloy:"sending_queue,block,optional"`
	Retry otelcol.RetryArguments `alloy:"retry_on_failure,block,optional"`

	// Add BatcherConfig once https://github.com/open-telemetry/opentelemetry-collector/issues/8122 is resolved
	// BatcherConfig exporterbatcher.Config `mapstructure:"batcher"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`

	Client GRPCClientArguments `alloy:"client,block"`
}

var _ exporter.Arguments = Arguments{}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Timeout: otelcol.DefaultTimeout,
	}

	args.Queue.SetToDefault()
	args.Retry.SetToDefault()
	args.Client.SetToDefault()
	args.DebugMetrics.SetToDefault()
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &otlpexporter.Config{
		TimeoutSettings: otelpexporterhelper.TimeoutSettings{
			Timeout: args.Timeout,
		},
		QueueConfig:  *args.Queue.Convert(),
		RetryConfig:  *args.Retry.Convert(),
		ClientConfig: *(*otelcol.GRPCClientArguments)(&args.Client).Convert(),
	}, nil
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return (*otelcol.GRPCClientArguments)(&args.Client).Extensions()
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements exporter.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}

// GRPCClientArguments is used to configure otelcol.exporter.otlp with
// component-specific defaults.
type GRPCClientArguments otelcol.GRPCClientArguments

// SetToDefault implements syntax.Defaulter.
func (args *GRPCClientArguments) SetToDefault() {
	*args = GRPCClientArguments{
		Headers:         map[string]string{},
		Compression:     otelcol.CompressionTypeGzip,
		WriteBufferSize: 512 * 1024,
		BalancerName:    otelcol.DefaultBalancerName,
	}
}
