// Package attributes provides an otelcol.processor.attributes component.
package attributes

import (
	"fmt"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/processor"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/attributesprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.attributes",
		Stability: featuregate.StabilityStable,
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
	Match otelcol.MatchConfig `river:",squash"`

	// Actions performed on the input data in the order specified in the config.
	// Example actions are "insert", "update", "upsert", "delete", "hash".
	Actions otelcol.AttrActionKeyValueSlice `river:"action,block,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `river:"output,block"`
}

var (
	_ processor.Arguments = Arguments{}
)

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	input := make(map[string]interface{})

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
