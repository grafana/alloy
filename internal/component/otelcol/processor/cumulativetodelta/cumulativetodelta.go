// Package cumulativetodelta provides an otelcol.processor.cumulativetodelta
// component.
package cumulativetodelta

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.cumulativetodelta",
		Stability: featuregate.StabilityPublicPreview,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := cumulativetodeltaprocessor.NewFactory()
			return processor.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.processor.cumulativetodelta component.
type Arguments struct {
	MaxStaleness time.Duration `alloy:"max_staleness,attr,optional"`
	InitialValue string        `alloy:"initial_value,attr,optional"`
	Include      MatchArgs     `alloy:"include,block,optional"`
	Exclude      MatchArgs     `alloy:"exclude,block,optional"`

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
	Include:      MatchArgs{},
	Exclude:      MatchArgs{},
	MaxStaleness: 0,
	InitialValue: "auto",
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.MaxStaleness < 0 {
		return fmt.Errorf("max_staleness must be a non-negative duration (got %s)", args.MaxStaleness)
	}
	if args.InitialValue != InitialValueAuto && args.InitialValue != InitialValueKeep && args.InitialValue != InitialValueDrop {
		return fmt.Errorf("initial_value must be one of %q, %q, %q", InitialValueAuto, InitialValueKeep, InitialValueDrop)
	}

	if (len(args.Include.Metrics) > 0 && len(args.Include.MatchType) == 0) ||
		(len(args.Exclude.Metrics) > 0 && len(args.Exclude.MatchType) == 0) {

		return fmt.Errorf("match_type must be set if metrics are supplied")
	}
	if (len(args.Include.MatchType) > 0 && len(args.Include.Metrics) == 0) ||
		(len(args.Exclude.MatchType) > 0 && len(args.Exclude.Metrics) == 0) {

		return fmt.Errorf("metrics must be supplied if match_type is set")
	}

	if (len(args.Include.MatchType) > 0 && args.Include.MatchType != "strict" && args.Include.MatchType != "regexp") ||
		(len(args.Include.MatchType) > 0 && args.Include.MatchType != "strict" && args.Include.MatchType != "regexp") {

		return fmt.Errorf("match_type must be one of %q and %q", "strict", "regexp")
	}

	return nil
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	var result cumulativetodeltaprocessor.Config

	result.MaxStaleness = args.MaxStaleness

	initialValue, err := ConvertInitialValue(args.InitialValue)
	if err != nil {
		return nil, err
	}

	err = mapstructure.Decode(initialValue, &result)
	if err != nil {
		return nil, err
	}

	include, err := args.Include.Convert()
	if err != nil {
		return nil, err
	}
	result.Include = *include

	exclude, err := args.Exclude.Convert()
	if err != nil {
		return nil, err
	}
	result.Exclude = *exclude

	return &result, nil
}

// Extensions implements processor.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
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

type MatchArgs struct {
	Metrics   []string `alloy:"metrics,attr"`
	MatchType string   `alloy:"match_type,attr"`
}

func (matchArgs MatchArgs) Convert() (*cumulativetodeltaprocessor.MatchMetrics, error) {
	var result cumulativetodeltaprocessor.MatchMetrics

	matchMetrics, err := ConvertMatchMetrics(matchArgs.Metrics, matchArgs.MatchType)

	if err != nil {
		return nil, err
	}

	err = mapstructure.Decode(matchMetrics, &result)
	if err != nil {
		return nil, err
	}

	return &result, nil
}

const (
	InitialValueAuto string = "auto"
	InitialValueKeep string = "keep"
	InitialValueDrop string = "drop"
)

func ConvertInitialValue(initialValue string) (map[string]interface{}, error) {
	switch initialValue {
	case InitialValueAuto:
		return map[string]interface{}{
			"initial_value": 0,
		}, nil
	case InitialValueKeep:
		return map[string]interface{}{
			"initial_value": 1,
		}, nil
	case InitialValueDrop:
		return map[string]interface{}{
			"initial_value": 2,
		}, nil
	default:
		return nil, fmt.Errorf(
			"unknown initial_value %q, allowed values are %q, %q, and %q",
			initialValue, InitialValueAuto, InitialValueKeep, InitialValueDrop)
	}
}

func ConvertMatchMetrics(metrics []string, matchType string) (map[string]interface{}, error) {
	var result = make(map[string]interface{})

	if len(metrics) > 0 {
		result["metrics"] = metrics
	}
	if matchType != "" {
		result["match_type"] = matchType
	}
	return result, nil
}
