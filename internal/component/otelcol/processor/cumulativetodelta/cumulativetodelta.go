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
	"github.com/mitchellh/mapstructure"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.cumulativetodelta",
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},
		Community: true,

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := cumulativetodeltaprocessor.NewFactory()
			return processor.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.processor.cumulativetodelta component.
type Arguments struct {
	MaxStaleness time.Duration `alloy:"max_staleness,attr,optional"`
	InitialValue InitialValue  `alloy:"initial_value,attr,optional"`

	Include *MatchMetrics `alloy:"include,block,optional"`
	Exclude *MatchMetrics `alloy:"exclude,block,optional"`

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
	MaxStaleness: 5 * time.Minute,  // same as cumulativetodelta's MaxStale default
	InitialValue: InitialValueAuto, // I Guess?
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.MaxStaleness < 0 {
		return fmt.Errorf("max_stale must be a positive duration (got %s)", args.MaxStaleness)
	}

	return nil
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	input := make(map[string]interface{})

	input["max_staleness"] = args.MaxStaleness
	input["initial_value"] = args.InitialValue
	input["include"] = args.Include.convertToMap()
	input["exclude"] = args.Exclude.convertToMap()

	var result cumulativetodeltaprocessor.Config
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

// Metric matcher used to include of exclude a set or metrics based of flexible criteria
type MatchMetrics struct {
	// regexp / strict
	MatchType string `alloy:"match_type,attr,optional"`

	// TODO: Do I need to configure this? How should I do it?
	// cf. github.com/open-telemetry/opentelemetry-collector-contrib/internal/filter/filterset/regexp
	// RegexpConfig string `alloy:"regexp,attr,optional"`

	// List of metric names (if match_type = strict) or regexs (if match_type = regexp)
	Metrics []string `alloy:"metrics,attr,optional"`
	// List of metric types to consider
	// MetricTypes []string `alloy:"metric_types,attr,optional"` // TODO uncomment in v0.118
}

func (m *MatchMetrics) convertToMap() map[string]interface{} {
	input := make(map[string]interface{})

	input["match_type"] = m.MatchType
	input["metrics"] = m.Metrics
	// input["metric_types"] = m.MetricTypes // TODO uncomment in v0.118

	return input
}

// cf. https://github.com/open-telemetry/opentelemetry-collector-contrib/blob/main/processor/cumulativetodeltaprocessor/internal/tracking/tracker.go
type InitialValue int

const (
	InitialValueAuto InitialValue = iota
	InitialValueKeep
	InitialValueDrop
)

func (i *InitialValue) UnmarshalText(text []byte) error {
	switch string(text) {
	case "auto":
		*i = InitialValueAuto
	case "keep":
		*i = InitialValueKeep
	case "drop":
		*i = InitialValueDrop
	default:
		return fmt.Errorf("unknown initial_value: %s", text)
	}
	return nil
}
