package signaltometrics_test

import (
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/connector/signaltometrics"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/signaltometricsconnector/config"
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/ottl"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/config/configoptional"
)

func TestArguments_UnmarshalAlloy(t *testing.T) {
	tests := []struct {
		testName string
		cfg      string
		expected config.Config
	}{
		{
			testName: "FullConfig",
			cfg: `
				spans {
					name        = "span.duration"
					description = "Span duration histogram"
					unit        = "ms"
					histogram {
						value = "Milliseconds(end_time - start_time)"
					}
				}

				datapoints {
					name = "datapoint.sum"
					attributes {
						key = "datapoint.kind"
					}
					sum {
						value = "value_double"
					}
				}

				logs {
					name        = "log.count.by.severity"
					description = "Number of logs by severity"
					conditions  = ["severity_number >= SEVERITY_NUMBER_WARN"]
					include_resource_attributes {
						key = "service.name"
					}
					attributes {
						key           = "severity_text"
						default_value = "INFO"
					}
					gauge {
						value = "1"
					}
				}

				error_mode = "ignore"

				output {}
			`,
			expected: config.Config{
				Spans: []config.MetricInfo{
					{
						Name:        "span.duration",
						Description: "Span duration histogram",
						Unit:        "ms",
						Histogram: configoptional.Some(config.Histogram{
							Buckets: []float64{2, 4, 6, 8, 10, 50, 100, 200, 400, 800, 1000, 1400, 2000, 5000, 10_000, 15_000},
							Value:   "Milliseconds(end_time - start_time)",
						}),
					},
				},
				Datapoints: []config.MetricInfo{
					{
						Name: "datapoint.sum",
						Attributes: []config.Attribute{
							{Key: "datapoint.kind"},
						},
						Sum: configoptional.Some(config.Sum{
							Value: "value_double",
						}),
					},
				},
				Logs: []config.MetricInfo{
					{
						Name:        "log.count.by.severity",
						Description: "Number of logs by severity",
						Conditions:  []string{"severity_number >= SEVERITY_NUMBER_WARN"},
						IncludeResourceAttributes: []config.Attribute{
							{Key: "service.name"},
						},
						Attributes: []config.Attribute{
							{Key: "severity_text", DefaultValue: "INFO"},
						},
						Gauge: configoptional.Some(config.Gauge{
							Value: "1",
						}),
					},
				},
				ErrorMode: ottl.IgnoreError,
			},
		},
		{
			testName: "ExponentialHistogramDefaults",
			cfg: `
				logs {
					name = "log.body.length"
					exponential_histogram {
						value = "Len(body)"
					}
				}

				output {}
			`,
			expected: config.Config{
				Logs: []config.MetricInfo{
					{
						Name: "log.body.length",
						ExponentialHistogram: configoptional.Some(config.ExponentialHistogram{
							MaxSize: 160,
							Value:   "Len(body)",
						}),
					},
				},
				ErrorMode: ottl.PropagateError,
			},
		},
		{
			testName: "HistogramAndExponentialHistogramOverrides",
			cfg: `
				spans {
					name = "span.duration"
					histogram {
						buckets = [1, 10, 100]
						value   = "Milliseconds(end_time - start_time)"
					}
				}

				logs {
					name = "log.body.length"
					exponential_histogram {
						max_size = 320
						value    = "Len(body)"
					}
				}

				output {}
			`,
			expected: config.Config{
				Spans: []config.MetricInfo{
					{
						Name: "span.duration",
						Histogram: configoptional.Some(config.Histogram{
							Buckets: []float64{1, 10, 100},
							Value:   "Milliseconds(end_time - start_time)",
						}),
					},
				},
				Logs: []config.MetricInfo{
					{
						Name: "log.body.length",
						ExponentialHistogram: configoptional.Some(config.ExponentialHistogram{
							MaxSize: 320,
							Value:   "Len(body)",
						}),
					},
				},
				ErrorMode: ottl.PropagateError,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args signaltometrics.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			require.NoError(t, err)

			err = args.Validate()
			require.NoError(t, err)

			actualConfig, err := args.Convert()
			require.NoError(t, err)

			actual := actualConfig.(*config.Config)
			require.Equal(t, tc.expected, *actual)
		})
	}
}

func TestArguments_NoSignals(t *testing.T) {
	// At least one signal must be configured. Validation runs during
	// unmarshalling, so the error surfaces there.
	var args signaltometrics.Arguments
	err := syntax.Unmarshal([]byte(`output {}`), &args)
	require.ErrorContains(t, err, "at least one should be specified")
}

func TestArguments_EmptyHistogramBuckets(t *testing.T) {
	// The default buckets are only applied when the buckets attribute is
	// omitted. An explicit empty list is passed through and rejected during
	// validation, rather than silently falling back to the defaults.
	cfg := `
		spans {
			name = "span.duration"
			histogram {
				buckets = []
				value   = "Milliseconds(end_time - start_time)"
			}
		}

		output {}
	`
	var args signaltometrics.Arguments
	err := syntax.Unmarshal([]byte(cfg), &args)
	require.ErrorContains(t, err, "histogram buckets missing")
}
