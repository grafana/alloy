package otelcolconvert

import (
	"fmt"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/connector/spanmetrics"
	"github.com/grafana/alloy/internal/converter/diag"
	"github.com/grafana/alloy/internal/converter/internal/common"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"
	"go.opentelemetry.io/collector/component"
)

func init() {
	converters = append(converters, spanmetricsConnectorConverter{})
}

type spanmetricsConnectorConverter struct{}

func (spanmetricsConnectorConverter) Factory() component.Factory {
	return spanmetricsconnector.NewFactory()
}

func (spanmetricsConnectorConverter) InputComponentName() string {
	return "otelcol.connector.spanmetrics"
}

func (spanmetricsConnectorConverter) ConvertAndAppend(state *State, id component.InstanceID, cfg component.Config) diag.Diagnostics {
	var diags diag.Diagnostics

	label := state.AlloyComponentLabel()

	args := toSpanmetricsConnector(state, id, cfg.(*spanmetricsconnector.Config))
	block := common.NewBlockWithOverride([]string{"otelcol", "connector", "spanmetrics"}, label, args)

	diags.Add(
		diag.SeverityLevelInfo,
		fmt.Sprintf("Converted %s into %s", StringifyInstanceID(id), StringifyBlock(block)),
	)

	state.Body().AppendBlock(block)
	return diags
}

func toSpanmetricsConnector(state *State, id component.InstanceID, cfg *spanmetricsconnector.Config) *spanmetrics.Arguments {
	if cfg == nil {
		return nil
	}
	var (
		nextMetrics = state.Next(id, component.DataTypeMetrics)
	)

	var exponential *spanmetrics.ExponentialHistogramConfig
	if cfg.Histogram.Exponential != nil {
		exponential = &spanmetrics.ExponentialHistogramConfig{
			MaxSize: cfg.Histogram.Exponential.MaxSize,
		}
	}

	var explicit *spanmetrics.ExplicitHistogramConfig
	if cfg.Histogram.Explicit != nil {
		explicit = &spanmetrics.ExplicitHistogramConfig{
			Buckets: cfg.Histogram.Explicit.Buckets,
		}
	}

	// If none have been explicitly set, assign the upstream default.
	if exponential == nil && explicit == nil {
		explicit = &spanmetrics.ExplicitHistogramConfig{Buckets: []time.Duration{}}
		explicit.SetToDefault()
	}

	var dimensions []spanmetrics.Dimension
	for _, d := range cfg.Dimensions {
		dimensions = append(dimensions, spanmetrics.Dimension{
			Name:    d.Name,
			Default: d.Default,
		})
	}

	var eventDimensions []spanmetrics.Dimension
	for _, d := range cfg.Dimensions {
		eventDimensions = append(eventDimensions, spanmetrics.Dimension{
			Name:    d.Name,
			Default: d.Default,
		})
	}

	timestampCacheSize := spanmetrics.DefaultArguments.TimestampCacheSize
	if cfg.TimestampCacheSize != nil {
		timestampCacheSize = *cfg.TimestampCacheSize
	}

	return &spanmetrics.Arguments{
		Dimensions:             dimensions,
		ExcludeDimensions:      cfg.ExcludeDimensions,
		DimensionsCacheSize:    cfg.DimensionsCacheSize,
		AggregationTemporality: spanmetrics.FromOTelAggregationTemporality(cfg.AggregationTemporality),
		Histogram: spanmetrics.HistogramConfig{
			Disable:     cfg.Histogram.Disable,
			Unit:        cfg.Histogram.Unit.String(),
			Exponential: exponential,
			Explicit:    explicit,
		},
		MetricsFlushInterval:         cfg.MetricsFlushInterval,
		MetricsExpiration:            cfg.MetricsExpiration,
		TimestampCacheSize:           timestampCacheSize,
		Namespace:                    cfg.Namespace,
		ResourceMetricsCacheSize:     cfg.ResourceMetricsCacheSize,
		ResourceMetricsKeyAttributes: cfg.ResourceMetricsKeyAttributes,
		Exemplars: spanmetrics.ExemplarsConfig{
			Enabled:         cfg.Exemplars.Enabled,
			MaxPerDataPoint: cfg.Exemplars.MaxPerDataPoint,
		},
		Events: spanmetrics.EventsConfig{
			Enabled:    cfg.Events.Enabled,
			Dimensions: eventDimensions,
		},

		Output: &otelcol.ConsumerArguments{
			Metrics: ToTokenizedConsumers(nextMetrics),
		},

		DebugMetrics: common.DefaultValue[spanmetrics.Arguments]().DebugMetrics,
	}
}
