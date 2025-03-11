// Package memorylimiter provides an otelcol.processor.memory_limiter component.
package memorylimiter

import (
	"fmt"
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
	"go.opentelemetry.io/collector/processor/memorylimiterprocessor"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.memory_limiter",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := memorylimiterprocessor.NewFactory()
			return processor.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.processor.memory_limiter component.
type Arguments struct {
	CheckInterval         time.Duration    `alloy:"check_interval,attr"`
	MemoryLimit           units.Base2Bytes `alloy:"limit,attr,optional"`
	MemorySpikeLimit      units.Base2Bytes `alloy:"spike_limit,attr,optional"`
	MemoryLimitPercentage uint32           `alloy:"limit_percentage,attr,optional"`
	MemorySpikePercentage uint32           `alloy:"spike_limit_percentage,attr,optional"`

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
	CheckInterval:         0,
	MemoryLimit:           0,
	MemorySpikeLimit:      0,
	MemoryLimitPercentage: 0,
	MemorySpikePercentage: 0,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.CheckInterval <= 0 {
		return fmt.Errorf("check_interval must be greater than zero")
	}

	if args.MemoryLimit > 0 && args.MemoryLimitPercentage > 0 {
		return fmt.Errorf("either limit or limit_percentage must be set, but not both")
	}

	if args.MemoryLimit > 0 {
		if args.MemorySpikeLimit >= args.MemoryLimit {
			return fmt.Errorf("spike_limit must be less than limit")
		}
		if args.MemorySpikeLimit == 0 {
			args.MemorySpikeLimit = args.MemoryLimit / 5
		}
		return nil
	}
	if args.MemoryLimitPercentage > 0 {
		if args.MemoryLimitPercentage <= 0 ||
			args.MemoryLimitPercentage > 100 ||
			args.MemorySpikePercentage <= 0 ||
			args.MemorySpikePercentage > 100 {

			return fmt.Errorf("limit_percentage and spike_limit_percentage must be greater than 0 and and less or equal than 100")
		}
		return nil
	}

	return fmt.Errorf("either limit or limit_percentage must be set to greater than zero")
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &memorylimiterprocessor.Config{
		CheckInterval:         args.CheckInterval,
		MemoryLimitMiB:        uint32(args.MemoryLimit / units.Mebibyte),
		MemorySpikeLimitMiB:   uint32(args.MemorySpikeLimit / units.Mebibyte),
		MemoryLimitPercentage: args.MemoryLimitPercentage,
		MemorySpikePercentage: args.MemorySpikePercentage,
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
