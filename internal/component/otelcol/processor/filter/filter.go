package filter

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.filter",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := filterprocessor.NewFactory()
			return processor.New(opts, fact, args.(Arguments))
		},
	})
}

type Arguments struct {
	// ErrorMode determines how the processor reacts to errors that occur while processing a statement.
	ErrorMode ottl.ErrorMode `alloy:"error_mode,attr,optional"`
	Traces    TraceConfig    `alloy:"traces,block,optional"`
	Metrics   MetricConfig   `alloy:"metrics,block,optional"`
	Logs      LogConfig      `alloy:"logs,block,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var (
	_ processor.Arguments = Arguments{}
)

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	ErrorMode: ottl.PropagateError,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	otelArgs, err := args.convertImpl()
	if err != nil {
		return err
	}
	return otelArgs.Validate()
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return args.convertImpl()
}

// convertImpl is a helper function which returns the real type of the config,
// instead of the otelcomponent.Config interface.
func (args Arguments) convertImpl() (*filterprocessor.Config, error) {
	input := make(map[string]any)

	input["error_mode"] = args.ErrorMode

	if len(args.Traces.Span) > 0 || len(args.Traces.SpanEvent) > 0 {
		input["traces"] = args.Traces.convert()
	}

	if len(args.Metrics.Metric) > 0 || len(args.Metrics.Datapoint) > 0 {
		input["metrics"] = args.Metrics.convert()
	}

	if len(args.Logs.LogRecord) > 0 {
		input["logs"] = args.Logs.convert()
	}

	result := filterprocessor.NewFactory().CreateDefaultConfig().(*filterprocessor.Config)
	err := mapstructure.Decode(input, result)

	if err != nil {
		return nil, err
	}

	return result, nil
}

// Extensions implements processor.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements processor.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements processor.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// DebugMetricsConfig implements processor.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
