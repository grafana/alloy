// Package connectors holds the otelgen specs for otelcol.connector.* components.
package connectors

import (
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/otelgen"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"
)

var All = []otelgen.Component{
	otelgen.Connector(
		"spanmetrics",
		spanmetricsconnector.NewFactory,
		otelgen.OverrideComponentName("spanmetrics"),
		otelgen.OverrideStability(
			"already generally available in Alloy, lowering it would be a breaking change",
			featuregate.StabilityGenerallyAvailable,
		),
		otelgen.ExtraArguments(),
		otelgen.ExtraValidation(),
		otelgen.OverrideArgumentName("dimensions", "dimension"),
		otelgen.OverrideArgumentName("calls_dimensions", "calls_dimension"),
		otelgen.OverrideArgumentName("histogram.dimensions", "dimension"),
		otelgen.OverrideArgumentName("events.dimensions", "dimension"),
		otelgen.OverrideArgumentRequired("histogram"),
		otelgen.OverrideArgumentType[string]("aggregation_temporality"),
		otelgen.OverrideArgumentType[int]("metric_timestamp_cache_size"),
		otelgen.OverrideDefaultArguments(
			"keeps the previous Alloy default",
			func(c *spanmetricsconnector.Config) {
				c.TimestampCacheSize = new(1000)
			},
		),
		otelgen.OverrideDefaultArguments(
			"keeps the previous Alloy default buckets for an explicit block",
			func(c *spanmetricsconnector.ExplicitHistogramConfig) {
				c.Buckets = []time.Duration{
					2 * time.Millisecond,
					4 * time.Millisecond,
					6 * time.Millisecond,
					8 * time.Millisecond,
					10 * time.Millisecond,
					50 * time.Millisecond,
					100 * time.Millisecond,
					200 * time.Millisecond,
					400 * time.Millisecond,
					800 * time.Millisecond,
					1 * time.Second,
					1400 * time.Millisecond,
					2 * time.Second,
					5 * time.Second,
					10 * time.Second,
					15 * time.Second,
				}
			},
		),
		otelgen.OverrideDefaultArguments(
			"keeps the previous Alloy default max_size for an exponential block",
			func(c *spanmetricsconnector.ExponentialHistogramConfig) {
				c.MaxSize = 160
			},
		),
	),
}
