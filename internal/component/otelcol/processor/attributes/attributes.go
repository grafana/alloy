// Package attributes provides an otelcol.processor.attributes component.
package attributes

import (
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.attributes",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := attributesprocessor.NewFactory()
			return processor.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.processor.attributes component.
type Arguments struct {
	// Pre-processing filtering to include/exclude data from the processor.
	Match otelcol.MatchConfig `alloy:",squash"`

	// Actions performed on the input data in the order specified in the config.
	// Example actions are "insert", "update", "upsert", "delete", "hash".
	Actions otelcol.AttrActionKeyValueSlice `alloy:"action,block,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var (
	_ processor.Arguments = Arguments{}
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	args.DebugMetrics.SetToDefault()
}

func (args *Arguments) Validate() error {
	return args.Actions.Validate()
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	input := make(map[string]any)

	if actions := args.Actions.Convert(); len(actions) > 0 {
		input["actions"] = actions
	}

	if args.Match.Include != nil {
		matchConfig, err := args.Match.Include.Convert()
		if err != nil {
			return nil, fmt.Errorf("error getting 'include' match properties: %w", err)
		}
		if len(matchConfig) > 0 {
			input["include"] = matchConfig
		}
	}

	if args.Match.Exclude != nil {
		matchConfig, err := args.Match.Exclude.Convert()
		if err != nil {
			return nil, fmt.Errorf("error getting 'exclude' match properties: %w", err)
		}
		if len(matchConfig) > 0 {
			input["exclude"] = matchConfig
		}
	}

	var result attributesprocessor.Config
	err := mapstructure.Decode(input, &result)

	if err != nil {
		return nil, err
	}

	return &result, nil
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
