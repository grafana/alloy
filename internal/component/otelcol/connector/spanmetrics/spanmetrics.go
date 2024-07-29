// Package spanmetrics provides an otelcol.connector.spanmetrics component.
package spanmetrics

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/otelcol"
	otelcolCfg "github.com/grafana/alloy/internal/component/otelcol/config"
	"github.com/grafana/alloy/internal/component/otelcol/connector"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.connector.spanmetrics",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   otelcol.ConsumerExports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			fact := spanmetricsconnector.NewFactory()
			return connector.New(opts, fact, args.(Arguments))
		},
	})
}

// Arguments configures the otelcol.connector.spanmetrics component.
type Arguments struct {
	// Dimensions defines the list of additional dimensions on top of the provided:
	// - service.name
	// - span.name
	// - span.kind
	// - status.code
	// The dimensions will be fetched from the span's attributes. Examples of some conventionally used attributes:
	// https://github.com/open-telemetry/opentelemetry-collector/blob/main/model/semconv/opentelemetry.go.
	Dimensions        []Dimension `alloy:"dimension,block,optional"`
	ExcludeDimensions []string    `alloy:"exclude_dimensions,attr,optional"`

	// DimensionsCacheSize defines the size of cache for storing Dimensions, which helps to avoid cache memory growing
	// indefinitely over the lifetime of the collector.
	DimensionsCacheSize int `alloy:"dimensions_cache_size,attr,optional"`

	// ResourceMetricsCacheSize defines the size of the cache holding metrics for a service. This is mostly relevant for
	// cumulative temporality to avoid memory leaks and correct metric timestamp resets.
	ResourceMetricsCacheSize int `alloy:"resource_metrics_cache_size,attr,optional"`

	// ResourceMetricsKeyAttributes filters the resource attributes used to create the resource metrics key hash.
	// This can be used to avoid situations where resource attributes may change across service restarts, causing
	// metric counters to break (and duplicate). A resource does not need to have all of the attributes. The list
	// must include enough attributes to properly identify unique resources or risk aggregating data from more
	// than one service and span.
	// e.g. ["service.name", "telemetry.sdk.language", "telemetry.sdk.name"]
	// See https://opentelemetry.io/docs/specs/semconv/resource/ for possible attributes.
	ResourceMetricsKeyAttributes []string `alloy:"resource_metrics_key_attributes,attr,optional"`

	AggregationTemporality string `alloy:"aggregation_temporality,attr,optional"`

	Histogram HistogramConfig `alloy:"histogram,block"`

	// MetricsEmitInterval is the time period between when metrics are flushed or emitted to the downstream components.
	MetricsFlushInterval time.Duration `alloy:"metrics_flush_interval,attr,optional"`

	// MetricsExpiration is the time period after which metrics are considered stale and are removed from the cache.
	// Default value (0) means that the metrics will never expire.
	MetricsExpiration time.Duration `alloy:"metrics_expiration,attr,optional"`

	// TimestampCacheSize controls the size of the cache used to keep track of delta metrics' TimestampUnixNano the last time it was flushed
	TimestampCacheSize int `alloy:"metric_timestamp_cache_size,attr,optional"`

	// Namespace is the namespace of the metrics emitted by the connector.
	Namespace string `alloy:"namespace,attr,optional"`

	// Exemplars defines the configuration for exemplars.
	Exemplars ExemplarsConfig `alloy:"exemplars,block,optional"`

	// Events defines the configuration for events section of spans.
	Events EventsConfig `alloy:"events,block,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `alloy:"output,block"`

	// DebugMetrics configures component internal metrics. Optional.
	DebugMetrics otelcolCfg.DebugMetricsArguments `alloy:"debug_metrics,block,optional"`
}

var (
	_ syntax.Validator    = (*Arguments)(nil)
	_ syntax.Defaulter    = (*Arguments)(nil)
	_ connector.Arguments = (*Arguments)(nil)
)

const (
	AggregationTemporalityCumulative = "CUMULATIVE"
	AggregationTemporalityDelta      = "DELTA"
)

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	DimensionsCacheSize:      1000,
	AggregationTemporality:   AggregationTemporalityCumulative,
	MetricsFlushInterval:     60 * time.Second,
	MetricsExpiration:        0,
	ResourceMetricsCacheSize: 1000,
	TimestampCacheSize:       1000,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
	args.DebugMetrics.SetToDefault()
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.DimensionsCacheSize <= 0 {
		return fmt.Errorf(
			"invalid cache size: %v, the maximum number of the items in the cache should be positive",
			args.DimensionsCacheSize)
	}

	if args.MetricsFlushInterval <= 0 {
		return fmt.Errorf("metrics_flush_interval must be greater than 0")
	}

	switch args.AggregationTemporality {
	case AggregationTemporalityCumulative, AggregationTemporalityDelta:
		// Valid
	default:
		return fmt.Errorf("invalid aggregation_temporality: %v", args.AggregationTemporality)
	}

	return nil
}

func convertAggregationTemporality(temporality string) (string, error) {
	switch temporality {
	case AggregationTemporalityCumulative:
		return "AGGREGATION_TEMPORALITY_CUMULATIVE", nil
	case AggregationTemporalityDelta:
		return "AGGREGATION_TEMPORALITY_DELTA", nil
	default:
		return "", fmt.Errorf("invalid aggregation_temporality: %v", temporality)
	}
}

func FromOTelAggregationTemporality(temporality string) string {
	switch temporality {
	case "AGGREGATION_TEMPORALITY_DELTA":
		return AggregationTemporalityDelta
	case "AGGREGATION_TEMPORALITY_CUMULATIVE":
		return AggregationTemporalityCumulative
	default:
		return ""
	}
}

// Convert implements connector.Arguments.
func (args Arguments) Convert() (otelcomponent.Config, error) {
	dimensions := make([]spanmetricsconnector.Dimension, 0, len(args.Dimensions))
	for _, d := range args.Dimensions {
		dimensions = append(dimensions, d.Convert())
	}

	histogram, err := args.Histogram.Convert()
	if err != nil {
		return nil, err
	}

	aggregationTemporality, err := convertAggregationTemporality(args.AggregationTemporality)
	if err != nil {
		return nil, err
	}

	excludeDimensions := append([]string(nil), args.ExcludeDimensions...)

	timestampCacheSize := args.TimestampCacheSize

	return &spanmetricsconnector.Config{
		Dimensions:                   dimensions,
		ExcludeDimensions:            excludeDimensions,
		DimensionsCacheSize:          args.DimensionsCacheSize,
		ResourceMetricsCacheSize:     args.ResourceMetricsCacheSize,
		TimestampCacheSize:           &timestampCacheSize,
		ResourceMetricsKeyAttributes: args.ResourceMetricsKeyAttributes,
		AggregationTemporality:       aggregationTemporality,
		Histogram:                    *histogram,
		MetricsFlushInterval:         args.MetricsFlushInterval,
		MetricsExpiration:            args.MetricsExpiration,
		Namespace:                    args.Namespace,
		Exemplars:                    *args.Exemplars.Convert(),
		Events:                       args.Events.Convert(),
	}, nil
}

// Extensions implements connector.Arguments.
func (args Arguments) Extensions() map[otelcomponent.ID]otelextension.Extension {
	return nil
}

// Exporters implements connector.Arguments.
func (args Arguments) Exporters() map[otelcomponent.DataType]map[otelcomponent.ID]otelcomponent.Component {
	return nil
}

// NextConsumers implements connector.Arguments.
func (args Arguments) NextConsumers() *otelcol.ConsumerArguments {
	return args.Output
}

// ConnectorType() int implements connector.Arguments.
func (Arguments) ConnectorType() int {
	return connector.ConnectorTracesToMetrics
}

// DebugMetricsConfig implements receiver.Arguments.
func (args Arguments) DebugMetricsConfig() otelcolCfg.DebugMetricsArguments {
	return args.DebugMetrics
}
