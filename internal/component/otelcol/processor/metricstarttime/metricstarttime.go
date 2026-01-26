// Package metricstarttime provides an otelcol.processor.metricstarttime component.
package metricstarttime

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/metricstarttimeprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.metric_start_time",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := metricstarttimeprocessor.NewFactory()
			return processor.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.processor.metricstarttime component.
type Arguments struct {
	// Strategy specifies which strategy to use for setting start time.
	// Valid values are "true_reset_point", "subtract_initial_point", and "start_time_metric".
	Strategy string `alloy:"strategy,attr,optional"`

	// GCInterval specifies how long to wait before removing a metric from the cache.
	GCInterval time.Duration `alloy:"gc_interval,attr,optional"`

	// StartTimeMetricRegex allows specifying a regex for a metric name containing
	// the start time for a resource. Only applies when strategy is "start_time_metric".
	StartTimeMetricRegex string `alloy:"start_time_metric_regex,attr,optional"`

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
	Strategy:   "true_reset_point",
	GCInterval: 10 * time.Minute,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	validStrategies := map[string]bool{
		"true_reset_point":       true,
		"subtract_initial_point": true,
		"start_time_metric":      true,
	}
	if !validStrategies[args.Strategy] {
		return fmt.Errorf("invalid strategy %q, must be one of: true_reset_point, subtract_initial_point, start_time_metric", args.Strategy)
	}
	if args.GCInterval <= 0 {
		return fmt.Errorf("gc_interval must be greater than 0")
	}
	return nil
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &metricstarttimeprocessor.Config{
		Strategy:             args.Strategy,
		GCInterval:           args.GCInterval,
		StartTimeMetricRegex: args.StartTimeMetricRegex,
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
