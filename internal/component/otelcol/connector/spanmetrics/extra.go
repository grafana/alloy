package spanmetrics

import (
	"errors"
	"fmt"
	"slices"

	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"
)

const collectorInstanceIDDimension = "collector.instance.id"

// ExtraArguments are the arguments that only exist in Alloy.
type ExtraArguments struct {
	// IncludeCollectorInstanceID keeps the collector.instance.id dimension,
	// which upstream always adds.
	IncludeCollectorInstanceID bool `alloy:"include_collector_instance_id,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (e *ExtraArguments) SetToDefault() {}

// ApplyTo applies the extra arguments to cfg.
func (e ExtraArguments) ApplyTo(cfg *spanmetricsconnector.Config) error {
	if !e.IncludeCollectorInstanceID && !slices.Contains(cfg.ExcludeDimensions, collectorInstanceIDDimension) {
		// Copy so the user's exclude_dimensions slice is not modified.
		cfg.ExcludeDimensions = append(slices.Clone(cfg.ExcludeDimensions), collectorInstanceIDDimension)
	}
	return nil
}

// validate checks what upstream validation does not.
func (args *Arguments) validate() error {
	// Upstream accepts 0 but panics creating its flush ticker.
	if args.MetricsFlushInterval <= 0 {
		return errors.New("metrics_flush_interval must be greater than 0")
	}
	switch args.AggregationTemporality {
	case aggregationTemporalityCumulative, aggregationTemporalityDelta:
	default:
		return fmt.Errorf("invalid aggregation_temporality: %v", args.AggregationTemporality)
	}
	if args.Histogram.Exponential == nil && args.Histogram.Explicit == nil {
		return errors.New("either exponential or explicit histogram configuration must be specified")
	}
	if args.Histogram.Exponential != nil && args.Histogram.Exponential.MaxSize <= 0 {
		return errors.New("max_size must be greater than 0")
	}
	return nil
}
