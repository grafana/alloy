package spanmetrics_test

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol/connector/spanmetrics"
	"github.com/grafana/alloy/internal/component/otelcol/processor/processortest"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/open-telemetry/opentelemetry-collector-contrib/connector/spanmetricsconnector"
	"github.com/stretchr/testify/require"
)

func getStringPtr(str string) *string {
	newStr := str
	return &newStr
}

func TestArguments_UnmarshalAlloy(t *testing.T) {
	defaultTimestampCacheSize := 1000
	timestampCacheSize := 12389

	tests := []struct {
		testName string
		cfg      string
		expected spanmetricsconnector.Config
		errorMsg string
	}{
		{
			testName: "defaultConfigExplicitHistogram",
			cfg: `
			histogram {
				explicit {}
			}

			output {}
			`,
			expected: spanmetricsconnector.Config{
				Dimensions:               []spanmetricsconnector.Dimension{},
				ExcludeDimensions:        nil,
				DimensionsCacheSize:      1000,
				TimestampCacheSize:       &defaultTimestampCacheSize,
				AggregationTemporality:   "AGGREGATION_TEMPORALITY_CUMULATIVE",
				ResourceMetricsCacheSize: 1000,
				Histogram: spanmetricsconnector.HistogramConfig{
					Disable:     false,
					Unit:        0,
					Exponential: nil,
					Explicit: &spanmetricsconnector.ExplicitHistogramConfig{
						Buckets: []time.Duration{
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
						},
					},
				},
				MetricsFlushInterval: 60 * time.Second,
				Namespace:            "",
				Exemplars: spanmetricsconnector.ExemplarsConfig{
					Enabled: false,
				},
				Events: spanmetricsconnector.EventsConfig{
					Enabled:    false,
					Dimensions: []spanmetricsconnector.Dimension{},
				},
			},
		},
		{
			testName: "defaultConfigExponentialHistogram",
			cfg: `
			histogram {
				exponential {}
			}

			output {}
			`,
			expected: spanmetricsconnector.Config{
				Dimensions:               []spanmetricsconnector.Dimension{},
				DimensionsCacheSize:      1000,
				ExcludeDimensions:        nil,
				AggregationTemporality:   "AGGREGATION_TEMPORALITY_CUMULATIVE",
				ResourceMetricsCacheSize: 1000,
				TimestampCacheSize:       &defaultTimestampCacheSize,
				Histogram: spanmetricsconnector.HistogramConfig{
					Disable:     false,
					Unit:        0,
					Exponential: &spanmetricsconnector.ExponentialHistogramConfig{MaxSize: 160},
					Explicit:    nil,
				},
				MetricsFlushInterval: 60 * time.Second,
				Namespace:            "",
				Events: spanmetricsconnector.EventsConfig{
					Enabled:    false,
					Dimensions: []spanmetricsconnector.Dimension{},
				},
			},
		},
		{
			testName: "explicitConfig",
			cfg: `
			dimension {
				name = "http.status_code"
			}
			dimension {
				name = "http.method"
				default = "GET"
			}
			exclude_dimensions = ["test_exclude_dim1", "test_exclude_dim2"]
			dimensions_cache_size = 333
			aggregation_temporality = "DELTA"
			resource_metrics_cache_size = 12345
			metric_timestamp_cache_size = 12389
			histogram {
				disable = true
				unit = "s"
				explicit {
					buckets = ["333ms", "777s", "999h"]
				}
			}
			metrics_flush_interval = "33s"
			metrics_expiration = "44s"
			namespace = "test.namespace"
			exemplars {
				enabled = true
			}
			events {
				enabled = true
				dimension {
					name = "exception1"
				}
			}
			output {}
			`,
			expected: spanmetricsconnector.Config{
				Dimensions: []spanmetricsconnector.Dimension{
					{Name: "http.status_code", Default: nil},
					{Name: "http.method", Default: getStringPtr("GET")},
				},
				ExcludeDimensions:        []string{"test_exclude_dim1", "test_exclude_dim2"},
				DimensionsCacheSize:      333,
				AggregationTemporality:   "AGGREGATION_TEMPORALITY_DELTA",
				ResourceMetricsCacheSize: 12345,
				TimestampCacheSize:       &timestampCacheSize,
				Histogram: spanmetricsconnector.HistogramConfig{
					Disable:     true,
					Unit:        1,
					Exponential: nil,
					Explicit: &spanmetricsconnector.ExplicitHistogramConfig{
						Buckets: []time.Duration{
							333 * time.Millisecond,
							777 * time.Second,
							999 * time.Hour,
						},
					},
				},
				MetricsFlushInterval: 33 * time.Second,
				MetricsExpiration:    44 * time.Second,
				Namespace:            "test.namespace",
				Exemplars: spanmetricsconnector.ExemplarsConfig{
					Enabled: true,
				},
				Events: spanmetricsconnector.EventsConfig{
					Enabled: true,
					Dimensions: []spanmetricsconnector.Dimension{
						{Name: "exception1", Default: nil},
					},
				},
			},
		},
		{
			testName: "exponentialHistogramMs",
			cfg: `
			histogram {
				unit = "ms"
				exponential {
					max_size = 123
				}
			}

			output {}
			`,
			expected: spanmetricsconnector.Config{
				Dimensions:               []spanmetricsconnector.Dimension{},
				DimensionsCacheSize:      1000,
				AggregationTemporality:   "AGGREGATION_TEMPORALITY_CUMULATIVE",
				ResourceMetricsCacheSize: 1000,
				TimestampCacheSize:       &defaultTimestampCacheSize,
				Histogram: spanmetricsconnector.HistogramConfig{
					Unit:        0,
					Exponential: &spanmetricsconnector.ExponentialHistogramConfig{MaxSize: 123},
					Explicit:    nil,
				},
				MetricsFlushInterval: 60 * time.Second,
				Namespace:            "",
				Events: spanmetricsconnector.EventsConfig{
					Enabled:    false,
					Dimensions: []spanmetricsconnector.Dimension{},
				},
			},
		},
		{
			testName: "invalidAggregationTemporality",
			cfg: `
			aggregation_temporality = "badVal"

			histogram {
				explicit {}
			}

			output {}
			`,
			errorMsg: `invalid aggregation_temporality: badVal`,
		},
		{
			testName: "invalidDimensionCache1",
			cfg: `
			dimensions_cache_size = -1

			histogram {
				explicit {}
			}

			output {}
			`,
			errorMsg: `invalid cache size: -1, the maximum number of the items in the cache should be positive`,
		},
		{
			testName: "invalidDimensionCache2",
			cfg: `
			dimensions_cache_size = 0

			histogram {
				explicit {}
			}

			output {}
			`,
			errorMsg: `invalid cache size: 0, the maximum number of the items in the cache should be positive`,
		},
		{
			testName: "invalidMetricsFlushInterval1",
			cfg: `
			metrics_flush_interval = "0s"

			histogram {
				explicit {}
			}

			output {}
			`,
			errorMsg: `metrics_flush_interval must be greater than 0`,
		},
		{
			testName: "invalidMetricsFlushInterval2",
			cfg: `
			metrics_flush_interval = "-1s"

			histogram {
				explicit {}
			}

			output {}
			`,
			errorMsg: `metrics_flush_interval must be greater than 0`,
		},
		{
			testName: "invalidDuplicateHistogramConfig",
			cfg: `
			histogram {
				explicit {
					buckets = ["333ms", "777s", "999h"]
				}
				exponential {
					max_size = 123
				}
			}

			output {}
			`,
			errorMsg: `only one of exponential or explicit histogram configuration can be specified`,
		},
		{
			testName: "invalidHistogramExplicitUnit",
			cfg: `
			histogram {
				explicit {
					buckets = ["333fakeunit", "777s", "999h"]
				}
			}

			output {}
			`,
			errorMsg: `4:17: "333fakeunit" time: unknown unit "fakeunit" in duration "333fakeunit"`,
		},
		{
			testName: "invalidHistogramExponentialSize",
			cfg: `
			histogram {
				exponential {
					max_size = -1
				}
			}

			output {}
			`,
			errorMsg: `max_size must be greater than 0`,
		},
		{
			testName: "invalidHistogramUnit",
			cfg: `
			histogram {
				unit = "badUnit"
				explicit {}
			}

			output {}
			`,
			errorMsg: `unknown unit "badUnit", allowed units are "ms" and "s"`,
		},
		{
			testName: "invalidHistogramNoConfig",
			cfg: `
			histogram {}

			output {}
			`,
			errorMsg: `either exponential or explicit histogram configuration must be specified`,
		},
		{
			testName: "invalidNoHistogram",
			cfg: `
			output {}
			`,
			errorMsg: `missing required block "histogram"`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.testName, func(t *testing.T) {
			var args spanmetrics.Arguments
			err := syntax.Unmarshal([]byte(tc.cfg), &args)
			if tc.errorMsg != "" {
				require.ErrorContains(t, err, tc.errorMsg)
				return
			}

			require.NoError(t, err)

			actualPtr, err := args.Convert()
			require.NoError(t, err)

			actual := actualPtr.(*spanmetricsconnector.Config)

			require.NoError(t, actual.Validate())

			require.Equal(t, tc.expected, *actual)
		})
	}
}

func testRunProcessor(t *testing.T, processorConfig string, testSignal processortest.Signal) {
	ctx := componenttest.TestContext(t)
	testRunProcessorWithContext(ctx, t, processorConfig, testSignal)
}

func testRunProcessorWithContext(ctx context.Context, t *testing.T, processorConfig string, testSignal processortest.Signal) {
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.connector.spanmetrics")
	require.NoError(t, err)

	var args spanmetrics.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(processorConfig), &args))

	// Override the arguments so signals get forwarded to the test channel.
	args.Output = testSignal.MakeOutput()

	prc := processortest.ProcessorRunConfig{
		Ctx:        ctx,
		T:          t,
		Args:       args,
		TestSignal: testSignal,
		Ctrl:       ctrl,
		L:          l,
	}
	processortest.TestRunProcessor(prc)
}

func Test_ComponentIO(t *testing.T) {
	const defaultInputTrace = `{
		"resourceSpans": [{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "TestSvcName" }
				},
				{
					"key": "res_attribute1",
					"value": { "intValue": "11" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"trace_id": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
					"span_id": "086e83747d0e381e",
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "78" }
					}]
				}]
			}]
		},{
			"resource": {
				"attributes": [{
					"key": "service.name",
					"value": { "stringValue": "TestSvcName" }
				},
				{
					"key": "res_attribute1",
					"value": { "intValue": "11" }
				}]
			},
			"scopeSpans": [{
				"spans": [{
					"trace_id": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
					"span_id": "086e83747d0e381b",
					"name": "TestSpan",
					"attributes": [{
						"key": "attribute1",
						"value": { "intValue": "78" }
					}]
				}]
			}]
		}]
	}`

	tests := []struct {
		testName              string
		cfg                   string
		inputTraceJson        string
		expectedOutputLogJson string
	}{
		{
			testName: "Sum metric only",
			cfg: `
			metrics_flush_interval = "1s"
			histogram {
				disable = true
				explicit {}
			}

			output {
				// no-op: will be overridden by test code.
			}
		`,
			inputTraceJson: defaultInputTrace,
			expectedOutputLogJson: `{
				"resourceMetrics": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "TestSvcName" }
						},
						{
							"key": "res_attribute1",
							"value": { "intValue": "11" }
						}]
					},		
					"scopeMetrics": [{
						"scope": {
							"name": "spanmetricsconnector"
						},
						"metrics": [{
							"name": "calls",
							"sum": {
								"dataPoints": [{
									"attributes": [{
										"key": "service.name",
										"value": { "stringValue": "TestSvcName" }
									},
									{
										"key": "span.name",
										"value": { "stringValue": "TestSpan" }
									},
									{
										"key": "span.kind",
										"value": { "stringValue": "SPAN_KIND_UNSPECIFIED" }
									},
									{
										"key": "status.code",
										"value": { "stringValue": "STATUS_CODE_UNSET" }
									}],
									"startTimeUnixNano": "0",
									"timeUnixNano": "0",
									"asInt": "2"
								}],
								"aggregationTemporality": 2,
								"isMonotonic": true
							}
						}]
					}]
				}]
			}`,
		},
		{
			testName: "Sum metric only for two spans",
			cfg: `
			metrics_flush_interval = "1s"
			histogram {
				disable = true
				explicit {}
			}

			output {
				// no-op: will be overridden by test code.
			}
		`,
			inputTraceJson: `{
				"resourceSpans": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "TestSvcName" }
						},
						{
							"key": "k8s.pod.name",
							"value": { "stringValue": "first" }
						}]
					},
					"scopeSpans": [{
						"spans": [{
							"trace_id": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
							"span_id": "086e83747d0e381e",
							"name": "TestSpan",
							"attributes": [{
								"key": "attribute1",
								"value": { "intValue": "78" }
							}]
						}]
					}]
				},{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "TestSvcName" }
						},
						{
							"key": "k8s.pod.name",
							"value": { "stringValue": "second" }
						}]
					},
					"scopeSpans": [{
						"spans": [{
							"trace_id": "7bba9f33312b3dbb8b2c2c62bb7abe2d",
							"span_id": "086e83747d0e381b",
							"name": "TestSpan",
							"attributes": [{
								"key": "attribute1",
								"value": { "intValue": "78" }
							}]
						}]
					}]
				}]
			}`,
			expectedOutputLogJson: `{
				"resourceMetrics": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "TestSvcName" }
						},
						{
							"key": "k8s.pod.name",
							"value": { "stringValue": "first" }
						}]
					},		
					"scopeMetrics": [{
						"scope": {
							"name": "spanmetricsconnector"
						},
						"metrics": [{
							"name": "calls",
							"sum": {
								"dataPoints": [{
									"attributes": [{
										"key": "service.name",
										"value": { "stringValue": "TestSvcName" }
									},
									{
										"key": "span.name",
										"value": { "stringValue": "TestSpan" }
									},
									{
										"key": "span.kind",
										"value": { "stringValue": "SPAN_KIND_UNSPECIFIED" }
									},
									{
										"key": "status.code",
										"value": { "stringValue": "STATUS_CODE_UNSET" }
									}],
									"startTimeUnixNano": "0",
									"timeUnixNano": "0",
									"asInt": "1"
								}],
								"aggregationTemporality": 2,
								"isMonotonic": true
							}
						}]
					}]
				},
				{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "TestSvcName" }
						},
						{
							"key": "k8s.pod.name",
							"value": { "stringValue": "second" }
						}]
					},		
					"scopeMetrics": [{
						"scope": {
							"name": "spanmetricsconnector"
						},
						"metrics": [{
							"name": "calls",
							"sum": {
								"dataPoints": [{
									"attributes": [{
										"key": "service.name",
										"value": { "stringValue": "TestSvcName" }
									},
									{
										"key": "span.name",
										"value": { "stringValue": "TestSpan" }
									},
									{
										"key": "span.kind",
										"value": { "stringValue": "SPAN_KIND_UNSPECIFIED" }
									},
									{
										"key": "status.code",
										"value": { "stringValue": "STATUS_CODE_UNSET" }
									}],
									"startTimeUnixNano": "0",
									"timeUnixNano": "0",
									"asInt": "1"
								}],
								"aggregationTemporality": 2,
								"isMonotonic": true
							}
						}]
					}]
				}]
			}`,
		},
		{
			testName: "Sum and histogram",
			cfg: `
			metrics_flush_interval = "1s"
			histogram {
				explicit {
					buckets = ["5m", "10m", "30m"]
				}
			}

			output {
				// no-op: will be overridden by test code.
			}
		`,
			inputTraceJson: defaultInputTrace,
			expectedOutputLogJson: `{
				"resourceMetrics": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "TestSvcName" }
						},
						{
							"key": "res_attribute1",
							"value": { "intValue": "11" }
						}]
					},		
					"scopeMetrics": [{
						"scope": {
							"name": "spanmetricsconnector"
						},
						"metrics": [{
							"name": "calls",
							"sum": {
								"dataPoints": [{
									"attributes": [{
										"key": "service.name",
										"value": { "stringValue": "TestSvcName" }
									},
									{
										"key": "span.name",
										"value": { "stringValue": "TestSpan" }
									},
									{
										"key": "span.kind",
										"value": { "stringValue": "SPAN_KIND_UNSPECIFIED" }
									},
									{
										"key": "status.code",
										"value": { "stringValue": "STATUS_CODE_UNSET" }
									}],
									"startTimeUnixNano": "0",
									"timeUnixNano": "0",
									"asInt": "2"
								}],
								"aggregationTemporality": 2,
								"isMonotonic": true
							}
						},
                        {
                            "name": "duration",
                            "unit": "ms",
                            "histogram": {
                                "dataPoints": [
                                    {
                                        "attributes": [
                                            {
                                                "key": "service.name",
                                                "value": {
                                                    "stringValue": "TestSvcName"
                                                }
                                            },
                                            {
                                                "key": "span.name",
                                                "value": {
                                                    "stringValue": "TestSpan"
                                                }
                                            },
                                            {
                                                "key": "span.kind",
                                                "value": {
                                                    "stringValue": "SPAN_KIND_UNSPECIFIED"
                                                }
                                            },
                                            {
                                                "key": "status.code",
                                                "value": {
                                                    "stringValue": "STATUS_CODE_UNSET"
                                                }
                                            }
                                        ],
                                        "count": "2",
                                        "sum": 0,
                                        "bucketCounts": [ "2", "0", "0", "0" ],
                                        "explicitBounds": [ 300000, 600000, 1800000 ]
                                    }
                                ],
                                "aggregationTemporality": 2
                            }
                        }]
					}]
				}]
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.testName, func(t *testing.T) {
			var args spanmetrics.Arguments
			require.NoError(t, syntax.Unmarshal([]byte(tt.cfg), &args))

			testRunProcessor(t, tt.cfg, processortest.NewTraceToMetricSignal(tt.inputTraceJson, tt.expectedOutputLogJson))
		})
	}
}
