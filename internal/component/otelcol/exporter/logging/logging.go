// Package logging provides an otelcol.exporter.logging component.
package logging

import (
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/exporter"
	"github.com/grafana/agent/internal/featuregate"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	loggingexporter "go.opentelemetry.io/collector/exporter/loggingexporter"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.logging",
		Stability: featuregate.StabilityStable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := loggingexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeAll)
		},
	})
}

// Arguments configures the otelcol.exporter.logging component.
type Arguments struct {
	Verbosity          configtelemetry.Level `river:"verbosity,attr,optional"`
	SamplingInitial    int                   `river:"sampling_initial,attr,optional"`
	SamplingThereafter int                   `river:"sampling_thereafter,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcol.DebugMetricsArguments `river:"debug_metrics,block,optional"`
}

var _ exporter.Arguments = Arguments{}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Verbosity:          configtelemetry.LevelNormal,
		SamplingInitial:    2,
		SamplingThereafter: 500,
	}
	args.DebugMetrics.SetToDefault()
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &loggingexporter.Config{
		Verbosity:          args.Verbosity,
		SamplingInitial:    args.SamplingInitial,
		SamplingThereafter: args.SamplingInitial,
	}, nil
}

// Extensions implements exporter.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements exporter.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcol.DebugMetricsArguments {
	return args.DebugMetrics
}
