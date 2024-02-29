// Package probabilistic_sampler provides an otelcol.processor.probabilistic_sampler component.
package probabilistic_sampler

import (
	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/processor"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.probabilistic_sampler",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := probabilisticsamplerprocessor.NewFactory()
			return processor.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.processor.probabilistic_sampler component.
type Arguments struct {
	SamplingPercentage float32 `river:"sampling_percentage,attr,optional"`
	HashSeed           uint32  `river:"hash_seed,attr,optional"`
	AttributeSource    string  `river:"attribute_source,attr,optional"`
	FromAttribute      string  `river:"from_attribute,attr,optional"`
	SamplingPriority   string  `river:"sampling_priority,attr,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `river:"output,block"`
}

var (
	_ processor.Arguments = Arguments{}
	_ river.Validator     = (*Arguments)(nil)
	_ river.Defaulter     = (*Arguments)(nil)
)

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	AttributeSource: "traceID",
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements river.Validator.
func (args *Arguments) Validate() error {
	cfg, err := args.Convert()
	if err != nil {
		return err
	}

	return cfg.(*probabilisticsamplerprocessor.Config).Validate()
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &probabilisticsamplerprocessor.Config{
		SamplingPercentage: args.SamplingPercentage,
		HashSeed:           args.HashSeed,
		AttributeSource:    probabilisticsamplerprocessor.AttributeSource(args.AttributeSource),
		FromAttribute:      args.FromAttribute,
		SamplingPriority:   args.SamplingPriority,
	}, nil
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
