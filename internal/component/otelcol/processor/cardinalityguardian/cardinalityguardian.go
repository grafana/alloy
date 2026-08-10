// Package cardinalityguardian provides an otelcol.processor.cardinality_guardian
// component.
package cardinalityguardian

import (
	"fmt"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/processor"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cardinalityguardianprocessor"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/pipeline"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.processor.cardinality_guardian",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return processor.New(opts, cardinalityguardianprocessor.NewFactory(), args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.processor.cardinality_guardian component.
type Arguments struct {
	// MaxCardinalityDeltaPerEpoch is the maximum number of new unique label
	// values allowed for a single metric+label-key combination within one
	// epoch.
	MaxCardinalityDeltaPerEpoch int `alloy:"max_cardinality_delta_per_epoch,attr,optional"`

	// EpochDurationSeconds controls how often the sliding cardinality window
	// advances, in seconds. Must be at least 10.
	EpochDurationSeconds int `alloy:"epoch_duration_seconds,attr,optional"`

	// NeverDropLabels is the list of label keys that the processor will never
	// strip or tag, regardless of how high their cardinality grows.
	NeverDropLabels []string `alloy:"never_drop_labels,attr,optional"`

	// EnforcementMode controls how the processor handles high-cardinality
	// attributes once the delta threshold is exceeded. One of "tag_only",
	// "overflow_attribute", or "strip_and_reaggregate".
	EnforcementMode string `alloy:"enforcement_mode,attr,optional"`

	// EstimatedCostPerMetricMonth configures the theoretical cost per active
	// time series, used solely to populate the
	// "otelcol_processor_cardinality_estimated_savings_dollars_total" counter.
	EstimatedCostPerMetricMonth float64 `alloy:"estimated_cost_per_metric_month,attr,optional"`

	// TopOffendersCount is the number of highest-delta (metric, label) pairs
	// to report via the "otelcol_processor_cardinality_top_offenders" gauge.
	// Set to 0 to disable.
	TopOffendersCount int `alloy:"top_offenders_count,attr,optional"`

	// MaxTrackerCount is the absolute maximum number of concurrent
	// (metric, label) tracking sketches across all shards. Set to 0 to
	// disable the limit.
	MaxTrackerCount int `alloy:"max_tracker_count,attr,optional"`

	// MetricOverrides allows per-metric cardinality limits that override the
	// global MaxCardinalityDeltaPerEpoch.
	MetricOverrides map[string]int `alloy:"metric_overrides,attr,optional"`

	// DropLogMaxPerEpoch caps the number of "Dropping high-cardinality
	// attribute" warning logs emitted per epoch. Set to 0 to disable the cap.
	DropLogMaxPerEpoch int `alloy:"drop_log_max_per_epoch,attr,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var (
	_ processor.Arguments = Arguments{}
	_ syntax.Defaulter    = &Arguments{}
	_ syntax.Validator    = &Arguments{}
)

// DefaultArguments holds default settings for Arguments, mirroring the
// upstream processor defaults.
var DefaultArguments = Arguments{
	MaxCardinalityDeltaPerEpoch: 100,
	EpochDurationSeconds:        300,
	NeverDropLabels:             []string{"http.status_code", "region"},
	EnforcementMode:             string(cardinalityguardianprocessor.EnforcementTagOnly),
	EstimatedCostPerMetricMonth: 0.05,
	TopOffendersCount:           10,
	MaxTrackerCount:             0,
	DropLogMaxPerEpoch:          10,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.NeverDropLabels = append([]string(nil), DefaultArguments.NeverDropLabels...)
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.MaxCardinalityDeltaPerEpoch <= 0 {
		return fmt.Errorf("max_cardinality_delta_per_epoch must be greater than 0")
	}
	if args.EpochDurationSeconds < 10 {
		return fmt.Errorf("epoch_duration_seconds must be at least 10")
	}
	if args.EstimatedCostPerMetricMonth < 0 {
		return fmt.Errorf("estimated_cost_per_metric_month cannot be negative")
	}
	if args.TopOffendersCount < 0 || args.TopOffendersCount > 500 {
		return fmt.Errorf("top_offenders_count must be between 0 and 500")
	}
	if args.MaxTrackerCount < 0 || args.MaxTrackerCount > 10000000 {
		return fmt.Errorf("max_tracker_count must be between 0 and 10,000,000")
	}
	for name, limit := range args.MetricOverrides {
		if name == "" {
			return fmt.Errorf("metric_overrides contains an empty metric name")
		}
		if limit <= 0 {
			return fmt.Errorf("metric_overrides[%q] must be greater than 0", name)
		}
	}
	if args.DropLogMaxPerEpoch < 0 {
		return fmt.Errorf("drop_log_max_per_epoch must be >= 0")
	}
	switch cardinalityguardianprocessor.EnforcementMode(args.EnforcementMode) {
	case cardinalityguardianprocessor.EnforcementTagOnly,
		cardinalityguardianprocessor.EnforcementOverflowAttribute,
		cardinalityguardianprocessor.EnforcementStripAndReaggregate:
		// valid
	default:
		return fmt.Errorf("enforcement_mode must be one of: tag_only, overflow_attribute, strip_and_reaggregate; got %q", args.EnforcementMode)
	}
	return nil
}

// Convert implements processor.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	return &cardinalityguardianprocessor.Config{
		MaxCardinalityDeltaPerEpoch: args.MaxCardinalityDeltaPerEpoch,
		EpochDurationSeconds:        args.EpochDurationSeconds,
		NeverDropLabels:             args.NeverDropLabels,
		EnforcementMode:             cardinalityguardianprocessor.EnforcementMode(args.EnforcementMode),
		EstimatedCostPerMetricMonth: args.EstimatedCostPerMetricMonth,
		TopOffendersCount:           args.TopOffendersCount,
		MaxTrackerCount:             args.MaxTrackerCount,
		MetricOverrides:             args.MetricOverrides,
		DropLogMaxPerEpoch:          args.DropLogMaxPerEpoch,
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
