// Package probabilistic_sampler provides an otelcol.processor.probabilistic_sampler component.
package probabilistic_sampler

import (
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/probabilisticsamplerprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.probabilistic_sampler",
		Stability: featuregate.StabilityGenerallyAvailable,
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
	SamplingPercentage float32 `alloy:"sampling_percentage,attr,optional"`
	HashSeed           uint32  `alloy:"hash_seed,attr,optional"`
	Mode               string  `alloy:"mode,attr,optional"`
	FailClosed         bool    `alloy:"fail_closed,attr,optional"`
	SamplingPrecision  int     `alloy:"sampling_precision,attr,optional"`
	AttributeSource    string  `alloy:"attribute_source,attr,optional"`
	FromAttribute      string  `alloy:"from_attribute,attr,optional"`
	SamplingPriority   string  `alloy:"sampling_priority,attr,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var (
	_ processor.Arguments = Arguments{}
	_ syntax.Validator    = (*Arguments)(nil)
	_ syntax.Defaulter    = (*Arguments)(nil)
)

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	FailClosed:        true,
	AttributeSource:   "traceID",
	SamplingPrecision: 4,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
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
		Mode:               probabilisticsamplerprocessor.SamplerMode(args.Mode),
		FailClosed:         args.FailClosed,
		SamplingPrecision:  args.SamplingPrecision,
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

// DebugMetricsConfig implements processor.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
