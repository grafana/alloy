// Package spanmetrics provides an otelcol.connector.spanmetrics component.
package spanmetrics

import (
	"fmt"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/grafana/agent/internal/component/otelcol/connector"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/river"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"
	otelcomponent "go.opentelemetry.io/collector/component"
	otelextension "go.opentelemetry.io/collector/extension"
)

func init() {
	component.Register(component.Registration{
		Name:      "otelcol.connector.spanmetrics",
		Stability: featuregate.StabilityExperimental,
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
	Dimensions        []Dimension `river:"dimension,block,optional"`
	ExcludeDimensions []string    `river:"exclude_dimensions,attr,optional"`

	// DimensionsCacheSize defines the size of cache for storing Dimensions, which helps to avoid cache memory growing
	// indefinitely over the lifetime of the collector.
	DimensionsCacheSize int `river:"dimensions_cache_size,attr,optional"`

	AggregationTemporality string `river:"aggregation_temporality,attr,optional"`

	Histogram HistogramConfig `river:"histogram,block"`

	// MetricsEmitInterval is the time period between when metrics are flushed or emitted to the downstream components.
	MetricsFlushInterval time.Duration `river:"metrics_flush_interval,attr,optional"`

	// Namespace is the namespace of the metrics emitted by the connector.
	Namespace string `river:"namespace,attr,optional"`

	// Exemplars defines the configuration for exemplars.
	Exemplars ExemplarsConfig `river:"exemplars,block,optional"`

	// Output configures where to send processed data. Required.
	Output *otelcol.ConsumerArguments `river:"output,block"`
}

var (
	_ river.Validator     = (*Arguments)(nil)
	_ river.Defaulter     = (*Arguments)(nil)
	_ connector.Arguments = (*Arguments)(nil)
)

const (
	AggregationTemporalityCumulative = "CUMULATIVE"
	AggregationTemporalityDelta      = "DELTA"
)

// DefaultArguments holds default settings for Arguments.
var DefaultArguments = Arguments{
	DimensionsCacheSize:    1000,
	AggregationTemporality: AggregationTemporalityCumulative,
	MetricsFlushInterval:   15 * time.Second,
}

// SetToDefault implements river.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// Validate implements river.Validator.
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

	return &spanmetricsconnector.Config{
		Dimensions:             dimensions,
		ExcludeDimensions:      excludeDimensions,
		DimensionsCacheSize:    args.DimensionsCacheSize,
		AggregationTemporality: aggregationTemporality,
		Histogram:              *histogram,
		MetricsFlushInterval:   args.MetricsFlushInterval,
		Namespace:              args.Namespace,
		Exemplars:              *args.Exemplars.Convert(),
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
