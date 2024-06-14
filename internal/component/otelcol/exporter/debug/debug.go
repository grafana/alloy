package debug

import (
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/exporter"
	"github.com/grafana/alloy/internal/featuregate"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/exporter/debugexporter"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.exporter.debug",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := debugexporter.NewFactory()
			return exporter.New(opts, fact, args.(Arguments), exporter.TypeAll)
		},
	})
}

type Arguments struct {
	Verbosity          string `alloy:"verbosity,attr,optional"`
	SamplingInitial    int    `alloy:"sampling_initial,attr,optional"`
	SamplingThereafter int    `alloy:"sampling_thereafter,attr,optional"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

func (args Arguments) convertVerbosity() (configtelemetry.Level, error) {
	var verbosity configtelemetry.Level
	switch args.Verbosity {
	case "basic":
		verbosity = configtelemetry.LevelBasic
	case "normal":
		verbosity = configtelemetry.LevelNormal
	case "detailed":
		verbosity = configtelemetry.LevelDetailed
	default:
		// Invalid verbosity
		// debugexporter only supports basic, normal and detailed levels
		return verbosity, fmt.Errorf("invalid verbosity %q", args.Verbosity)
	}

	return verbosity, nil
}

var _ exporter.Arguments = Arguments{}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Verbosity:          "normal",
		SamplingInitial:    2,
		SamplingThereafter: 500,
	}
	args.DebugMetrics.SetToDefault()
}

// Convert implements exporter.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	verbosity, err := args.convertVerbosity()
	if err != nil {
		return nil, fmt.Errorf("error in conversion to config arguments, %v", err)
	}

	return &debugexporter.Config{
		Verbosity:          verbosity,
		SamplingInitial:    args.SamplingInitial,
		SamplingThereafter: args.SamplingThereafter,
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
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
