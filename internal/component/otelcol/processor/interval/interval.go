// Package interval provides an otelcol.processor.interval component.
package interval

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/intervalprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.interval",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return processor.New(opts, intervalprocessor.NewFactory(), args.(Arguments))
		},
	})
}

type Arguments struct {
	// The interval in which the processor should export the aggregated metrics. Default: 60s.
	Interval time.Duration `alloy:"interval,attr,optional"`

	PassThrough PassThrough `alloy:"passthrough,block,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

type PassThrough struct {
	// Determines whether gauge metrics should be passed through as they are or aggregated.
	Gauge bool `alloy:"gauge,attr,optional"`
	// Determines whether summary metrics should be passed through as they are or aggregated.
	Summary bool `alloy:"summary,attr,optional"`
}

func (args PassThrough) Convert() intervalprocessor.PassThrough {
	return intervalprocessor.PassThrough{
		Gauge:   args.Gauge,
		Summary: args.Summary,
	}
}

var _ processor.Arguments = Arguments{}

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	Interval: 60 * time.Second,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.Interval <= 0 {
		return fmt.Errorf("interval must be greater than 0")
	}
	return nil
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &intervalprocessor.Config{
		Interval:    args.Interval,
		PassThrough: args.PassThrough.Convert(),
	}, nil
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
