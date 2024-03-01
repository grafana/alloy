package filter

import (
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/processor"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/filterprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.filter",
		Stability: featuregate.StabilityExperimental,
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
	ErrorMode ottl.ErrorMode `river:"error_mode,attr,optional"`
	Traces    TraceConfig    `river:"traces,block,optional"`
	Metrics   MetricConfig   `river:"metrics,block,optional"`
	Logs      LogConfig      `river:"logs,block,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `river:"output,block"`
}

var (
	_ processor.Arguments = Arguments{}
)

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	ErrorMode: ottl.PropagateError,
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements river.Validator.
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
	input := make(map[string]interface{})

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

	var result filterprocessor.Config
	err := mapstructure.Decode(input, &result)

	if err != nil {
		return nil, err
	}

	return &result, nil
}

// Extensions implements processor.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements processor.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements processor.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}
