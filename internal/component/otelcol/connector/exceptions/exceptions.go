// Package exceptions provides an otelcol.connector.exceptions component.
package exceptions

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/exceptionsconnector"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/connector"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.connector.exceptions",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := exceptionsconnector.NewFactory()
			return connector.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.connector.exceptions component.
type Arguments struct {
	// Dimensions defines the list of additional dimensions on top of the provided:
	// - service.name
	// - span.name
	// - span.kind
	// - status.code
	// The dimensions are looked up in the span, span event, and resource attributes.
	Dimensions []Dimension `alloy:"dimension,block,optional"`

	// Exemplars configures how to attach exemplars to metrics.
	Exemplars ExemplarsConfig `alloy:"exemplars,block,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

// Dimension defines a dimension name and an optional default value used when
// the attribute is missing.
type Dimension struct {
	Name    string  `alloy:"name,attr"`
	Default *string `alloy:"default,attr,optional"`
}

func (d Dimension) Convert() exceptionsconnector.Dimension {
	res := exceptionsconnector.Dimension{Name: d.Name}
	if d.Default != nil {
		str := *d.Default
		res.Default = &str
	}
	return res
}

// ExemplarsConfig configures how exemplars are attached to metrics.
type ExemplarsConfig struct {
	Enabled bool `alloy:"enabled,attr,optional"`
}

var (
	_ syntax.Validator = (*Arguments)(nil)
	_ syntax.Defaulter = (*Arguments)(nil)
)

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = Arguments{
		Dimensions: []Dimension{
			{Name: string(semconv.ExceptionTypeKey)},
			{Name: string(semconv.ExceptionMessageKey)},
		},
	}
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	cfg, err := args.Convert()
	if err != nil {
		return err
	}
	return cfg.(*exceptionsconnector.Config).Validate()
}

// Convert implements connector.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	dimensions := make([]exceptionsconnector.Dimension, 0, len(args.Dimensions))
	for _, d := range args.Dimensions {
		dimensions = append(dimensions, d.Convert())
	}

	return &exceptionsconnector.Config{
		Dimensions: dimensions,
		Exemplars: exceptionsconnector.Exemplars{
			Enabled: args.Exemplars.Enabled,
		},
	}, nil
}

// Extensions implements connector.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// Exporters implements connector.Arguments.
func (args Arguments) Exporters() map[pipeline.Signal]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements connector.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// ConnectorType implements connector.Arguments.
func (Arguments) ConnectorType() int {
	return connector.ConnectorTracesToMetrics |
		connector.ConnectorTracesToLogs |
		connector.ConnectorLogsToMetrics |
		connector.ConnectorLogsToLogs
}

// DebugMetricsConfig implements connector.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
