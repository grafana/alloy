// Package memorylimiter provides an otelcol.processor.memory_limiter component.
package memorylimiter

import (
	"errors"
	"fmt"
	"strings"
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

	// Floor and ceiling for the forced-GC interval on each limit path.
	MinGCIntervalWhenSoftLimited time.Duration `alloy:"min_gc_interval_when_soft_limited,attr,optional"`
	MinGCIntervalWhenHardLimited time.Duration `alloy:"min_gc_interval_when_hard_limited,attr,optional"`
	MaxGCIntervalWhenSoftLimited time.Duration `alloy:"max_gc_interval_when_soft_limited,attr,optional"`
	MaxGCIntervalWhenHardLimited time.Duration `alloy:"max_gc_interval_when_hard_limited,attr,optional"`

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

// SetToDefault implements syntax.Defaulter. The GC intervals come from the
// upstream factory rather than literals here, so a contrib bump carries through.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.DebugMetrics.SetToDefault()

	upstream := memorylimiterprocessor.NewFactory().CreateDefaultConfig().(*memorylimiterprocessor.Config)
	args.MinGCIntervalWhenSoftLimited = upstream.MinGCIntervalWhenSoftLimited
	args.MinGCIntervalWhenHardLimited = upstream.MinGCIntervalWhenHardLimited
	args.MaxGCIntervalWhenSoftLimited = upstream.MaxGCIntervalWhenSoftLimited
	args.MaxGCIntervalWhenHardLimited = upstream.MaxGCIntervalWhenHardLimited
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	// Round down before validating anything so the rules below, and upstream's,
	// describe the values the processor will actually run with.
	args.MemoryLimit = roundDownToMiB(args.MemoryLimit)
	args.MemorySpikeLimit = roundDownToMiB(args.MemorySpikeLimit)

	// Upstream accepts both and silently prefers limit, so this rule is ours alone.
	if args.MemoryLimit > 0 && args.MemoryLimitPercentage > 0 {
		return fmt.Errorf("either limit or limit_percentage must be set, but not both")
	}

	// Ours too: upstream doesn't require spike_limit_percentage to be set at all.
	pctOutOfRange := args.MemoryLimitPercentage > 100 || args.MemorySpikePercentage <= 0 || args.MemorySpikePercentage > 100
	if args.MemoryLimitPercentage > 0 && pctOutOfRange {
		return fmt.Errorf("limit_percentage and spike_limit_percentage must be greater than 0 and and less or equal than 100")
	}

	if args.MemoryLimit > 0 && args.MemorySpikeLimit == 0 {
		args.MemorySpikeLimit = roundDownToMiB(args.MemoryLimit / 5)
	}

	// Every remaining rule is upstream's, reported in Alloy's attribute names.
	return args.validateUpstream()
}

// roundDownToMiB drops any remainder below a whole MiB, which is all upstream's
// limit fields can hold.
func roundDownToMiB(b units.Base2Bytes) units.Base2Bytes {
	return b / units.Mebibyte * units.Mebibyte
}

// validateUpstream runs the collector's own rules rather than restating them here.
func (args Arguments) validateUpstream() error {
	otelCfg, err := args.Convert()
	if err != nil {
		return err
	}

	return alloyFieldNames(otelCfg.(*memorylimiterprocessor.Config).Validate())
}

// upstreamFieldNames rewrites upstream's MiB-suffixed field names to the Alloy
// attributes users actually write. That is what lets us defer those rules to
// upstream instead of restating them here to get the wording right.
var upstreamFieldNames = strings.NewReplacer(
	"'spike_limit_mib'", "'spike_limit'",
	"'limit_mib'", "'limit'",
)

func alloyFieldNames(err error) error {
	if err == nil {
		return nil
	}

	msg := upstreamFieldNames.Replace(err.Error())
	if msg == err.Error() {
		return err
	}

	return errors.New(msg)
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	result := memorylimiterprocessor.NewFactory().CreateDefaultConfig().(*memorylimiterprocessor.Config)

	result.CheckInterval = args.CheckInterval
	result.MemoryLimitMiB = uint32(roundDownToMiB(args.MemoryLimit) / units.Mebibyte)
	result.MemorySpikeLimitMiB = uint32(roundDownToMiB(args.MemorySpikeLimit) / units.Mebibyte)
	result.MemoryLimitPercentage = args.MemoryLimitPercentage
	result.MemorySpikePercentage = args.MemorySpikePercentage
	result.MinGCIntervalWhenSoftLimited = args.MinGCIntervalWhenSoftLimited
	result.MinGCIntervalWhenHardLimited = args.MinGCIntervalWhenHardLimited
	result.MaxGCIntervalWhenSoftLimited = args.MaxGCIntervalWhenSoftLimited
	result.MaxGCIntervalWhenHardLimited = args.MaxGCIntervalWhenHardLimited

	return result, nil
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
