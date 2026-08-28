package count_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/connector/count"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/countconnector"
	"github.com/stretchr/testify/require"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected countconnector.Config
	}{
		{
			testName: "Defaults",
			cfg: `
				output {}
			`,
			expected: countconnector.Config{
				Spans: map[string]countconnector.MetricInfo{
					"trace.span.count": {
						Description: "The number of spans observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				SpanEvents: map[string]countconnector.MetricInfo{
					"trace.span.event.count": {
						Description: "The number of span events observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				Metrics: map[string]countconnector.MetricInfo{
					"metric.count": {
						Description: "The number of metrics observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				DataPoints: map[string]countconnector.MetricInfo{
					"metric.datapoint.count": {
						Description: "The number of data points observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				Logs: map[string]countconnector.MetricInfo{
					"log.record.count": {
						Description: "The number of log records observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
			},
		},
		{
			testName: "CustomSpansCount",
			cfg: `
				spans {
					name = "my_span_count"
					description = "My custom span count"
				}

				output {}
			`,
			expected: countconnector.Config{
				Spans: map[string]countconnector.MetricInfo{
					"my_span_count": {
						Description: "My custom span count",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				SpanEvents: map[string]countconnector.MetricInfo{
					"trace.span.event.count": {
						Description: "The number of span events observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				Metrics: map[string]countconnector.MetricInfo{
					"metric.count": {
						Description: "The number of metrics observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				DataPoints: map[string]countconnector.MetricInfo{
					"metric.datapoint.count": {
						Description: "The number of data points observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				Logs: map[string]countconnector.MetricInfo{
					"log.record.count": {
						Description: "The number of log records observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
			},
		},
		{
			testName: "WithConditions",
			cfg: `
				logs {
					name = "error_log_count"
					description = "Count of error logs"
					conditions = [
						"severity_number >= SEVERITY_NUMBER_ERROR",
					]
				}

				output {}
			`,
			expected: countconnector.Config{
				Spans: map[string]countconnector.MetricInfo{
					"trace.span.count": {
						Description: "The number of spans observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				SpanEvents: map[string]countconnector.MetricInfo{
					"trace.span.event.count": {
						Description: "The number of span events observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				Metrics: map[string]countconnector.MetricInfo{
					"metric.count": {
						Description: "The number of metrics observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				DataPoints: map[string]countconnector.MetricInfo{
					"metric.datapoint.count": {
						Description: "The number of data points observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				Logs: map[string]countconnector.MetricInfo{
					"error_log_count": {
						Description: "Count of error logs",
						Conditions: []string{
							"severity_number >= SEVERITY_NUMBER_ERROR",
						},
						Attributes: []countconnector.AttributeConfig{},
					},
				},
			},
		},
		{
			testName: "WithAttributes",
			cfg: `
				datapoints {
					name = "datapoint_by_service"
					description = "Data points grouped by service"
					attributes {
						key = "service.name"
					}
				}

				output {}
			`,
			expected: countconnector.Config{
				Spans: map[string]countconnector.MetricInfo{
					"trace.span.count": {
						Description: "The number of spans observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				SpanEvents: map[string]countconnector.MetricInfo{
					"trace.span.event.count": {
						Description: "The number of span events observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				Metrics: map[string]countconnector.MetricInfo{
					"metric.count": {
						Description: "The number of metrics observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				DataPoints: map[string]countconnector.MetricInfo{
					"datapoint_by_service": {
						Description: "Data points grouped by service",
						Attributes: []countconnector.AttributeConfig{
							{
								Key: "service.name",
							},
						},
					},
				},
				Logs: map[string]countconnector.MetricInfo{
					"log.record.count": {
						Description: "The number of log records observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
			},
		},
		{
			testName: "WithAttributesAndDefaults",
			cfg: `
				spans {
					name = "span_by_env"
					description = "Spans grouped by environment"
					attributes {
						key           = "env"
						default_value = "production"
					}
				}

				output {}
			`,
			expected: countconnector.Config{
				Spans: map[string]countconnector.MetricInfo{
					"span_by_env": {
						Description: "Spans grouped by environment",
						Attributes: []countconnector.AttributeConfig{
							{
								Key:          "env",
								DefaultValue: "production",
							},
						},
					},
				},
				SpanEvents: map[string]countconnector.MetricInfo{
					"trace.span.event.count": {
						Description: "The number of span events observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				Metrics: map[string]countconnector.MetricInfo{
					"metric.count": {
						Description: "The number of metrics observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				DataPoints: map[string]countconnector.MetricInfo{
					"metric.datapoint.count": {
						Description: "The number of data points observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
				Logs: map[string]countconnector.MetricInfo{
					"log.record.count": {
						Description: "The number of log records observed.",
						Attributes:  []countconnector.AttributeConfig{},
					},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args count.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			err = args.Validate()
			require.NoError(t, err)

			actualConfig, err := args.Convert()
			require.NoError(t, err)

			actual := actualConfig.(*countconnector.Config)
			require.Equal(t, tc.expected, *actual)
		})
	}
}
