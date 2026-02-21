package convert_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol/exporter/prometheus/internal/convert"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/testappender"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func TestConverter(t *testing.T) {
	tt := []struct {
		name   string
		input  string
		expect string

		showTimestamps                bool
		includeTargetInfo             bool
		includeScopeInfo              bool
		includeScopeLabels            bool
		addMetricSuffixes             bool
		enableOpenMetrics             bool
		resourceToTelemetryConversion bool
	}{
		{
			name: "Gauge with metadata",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_seconds",
							"description": "A test gauge metric measuring duration",
							"gauge": {
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"as_double": 1234.56
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# HELP test_metric_seconds A test gauge metric measuring duration
				# TYPE test_metric_seconds gauge
				test_metric_seconds 1234.56
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Monotonic sum without metadata",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_seconds_total",
							"sum": {
								"aggregation_temporality": 2,
								"is_monotonic": true,
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"as_double": 15,
									"exemplars":[
										{
											"time_unix_nano": 1000000001,
											"as_double": 0.3,
											"span_id": "aaaaaaaaaaaaaaaa",
											"trace_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
										}
									]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_seconds counter
				test_metric_seconds_total 15.0 # {span_id="aaaaaaaaaaaaaaaa",trace_id="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} 0.3
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Non-monotonic sum",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_seconds",
							"sum": {
								"aggregation_temporality": 2,
								"is_monotonic": false,
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"as_double": 15
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_seconds gauge
				test_metric_seconds 15.0
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Histogram",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_seconds",
							"description": "A histogram of request durations",
							"histogram": {
								"aggregation_temporality": 2,
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"count": 333,
									"sum": 100,
									"bucket_counts": [0, 111, 0, 222],
									"explicit_bounds": [0.25, 0.5, 0.75, 1.0],
									"exemplars":[
										{
											"time_unix_nano": 1000000001,
											"as_double": 0.3,
											"span_id": "aaaaaaaaaaaaaaaa",
											"trace_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
										},
										{
											"time_unix_nano": 1000000003,
											"as_double": 1.5,
											"span_id": "cccccccccccccccc",
											"trace_id": "cccccccccccccccccccccccccccccccc"
										},
										{
											"time_unix_nano": 1000000002,
											"as_double": 0.5,
											"span_id": "bbbbbbbbbbbbbbbb",
											"trace_id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
										}
									]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# HELP test_metric_seconds A histogram of request durations
				# TYPE test_metric_seconds histogram
				test_metric_seconds_bucket{le="0.25"} 0
				test_metric_seconds_bucket{le="0.5"} 111 # {span_id="aaaaaaaaaaaaaaaa",trace_id="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} 0.3
				test_metric_seconds_bucket{le="0.75"} 111 # {span_id="bbbbbbbbbbbbbbbb",trace_id="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"} 0.5
				test_metric_seconds_bucket{le="1.0"} 333
				test_metric_seconds_bucket{le="+Inf"} 333 # {span_id="cccccccccccccccc",trace_id="cccccccccccccccccccccccccccccccc"} 1.5
				test_metric_seconds_sum 100.0
				test_metric_seconds_count 333
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Histogram out-of-order exemplars",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_seconds",
							"histogram": {
								"aggregation_temporality": 2,
								"data_points": [{
									"start_time_unix_nano": 1000000010,
									"time_unix_nano": 1000000010,
									"count": 333,
									"sum": 100,
									"bucket_counts": [0, 111, 0, 222],
									"explicit_bounds": [0.25, 0.5, 0.75, 1.0],
									"exemplars":[
										{
											"time_unix_nano": 1000000001,
											"as_double": 0.3,
											"span_id": "aaaaaaaaaaaaaaaa",
											"trace_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
										},
										{
											"time_unix_nano": 1000000000,
											"as_double": 0.3,
											"span_id": "dddddddddddddddd",
											"trace_id": "dddddddddddddddddddddddddddddddd"
										},
										{
											"time_unix_nano": 1000000003,
											"as_double": 1.5,
											"span_id": "cccccccccccccccc",
											"trace_id": "cccccccccccccccccccccccccccccccc"
										},
										{
											"time_unix_nano": 1000000002,
											"as_double": 0.5,
											"span_id": "bbbbbbbbbbbbbbbb",
											"trace_id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
										}
									]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_seconds histogram
				test_metric_seconds_bucket{le="0.25"} 0
				test_metric_seconds_bucket{le="0.5"} 111 # {span_id="aaaaaaaaaaaaaaaa",trace_id="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} 0.3
				test_metric_seconds_bucket{le="0.75"} 111 # {span_id="bbbbbbbbbbbbbbbb",trace_id="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"} 0.5
				test_metric_seconds_bucket{le="1.0"} 333
				test_metric_seconds_bucket{le="+Inf"} 333 # {span_id="cccccccccccccccc",trace_id="cccccccccccccccccccccccccccccccc"} 1.5
				test_metric_seconds_sum 100.0
				test_metric_seconds_count 333
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Histogram out-of-order bounds",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_seconds",
							"histogram": {
								"aggregation_temporality": 2,
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"count": 333,
									"sum": 100,
									"bucket_counts": [0, 111, 0, 222],
									"explicit_bounds": [0.5, 1.0, 0.25, 0.75]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_seconds histogram
				test_metric_seconds_bucket{le="0.25"} 0
				test_metric_seconds_bucket{le="0.5"} 0
				test_metric_seconds_bucket{le="0.75"} 222
				test_metric_seconds_bucket{le="1.0"} 333
				test_metric_seconds_bucket{le="+Inf"} 333
				test_metric_seconds_sum 100.0
				test_metric_seconds_count 333
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Summary with metadata",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_seconds",
							"description": "A test summary metric measuring duration",
							"summary": {
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"count": 333,
									"sum": 100,
									"quantile_values": [
										{ "quantile": 0, "value": 100 },
										{ "quantile": 0.25, "value": 200 },
										{ "quantile": 0.5, "value": 300 },
										{ "quantile": 0.75, "value": 400 },
										{ "quantile": 1, "value": 500 }
									]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# HELP test_metric_seconds A test summary metric measuring duration
				# TYPE test_metric_seconds summary
				test_metric_seconds{quantile="0.0"} 100.0
				test_metric_seconds{quantile="0.25"} 200.0
				test_metric_seconds{quantile="0.5"} 300.0
				test_metric_seconds{quantile="0.75"} 400.0
				test_metric_seconds{quantile="1.0"} 500.0
				test_metric_seconds_sum 100.0
				test_metric_seconds_count 333
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Timestamps",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_seconds",
							"gauge": {
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"as_double": 1234.56
								}]
							}
						}]
					}]
				}]
			}`,
			showTimestamps: true,
			expect: `
				# TYPE test_metric_seconds gauge
				test_metric_seconds 1234.56 1.0
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Labels from resource attributes",
			input: `{
				"resource_metrics": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "myservice" }
						}, {
							"key": "service.instance.id",
							"value": { "stringValue": "instance" }
						}, {
							"key": "do_not_display",
							"value": { "stringValue": "test" }
						}]
					},
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_seconds",
							"gauge": {
								"data_points": [{
									"as_double": 1234.56
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_seconds gauge
				test_metric_seconds{instance="instance",job="myservice"} 1234.56
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Labels from scope name and version",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"scope": {
							"name": "a-name",
							"version": "a-version",
							"attributes": [{
								"key": "something.extra",
								"value": { "stringValue": "zzz-extra-value" }
							}]
						},
						"metrics": [{
							"name": "test_metric_seconds",
							"gauge": {
								"data_points": [{
									"as_double": 1234.56
								}]
							}
						}]
					}]
				}]
			}`,
			includeScopeInfo: true,
			expect: `
				# TYPE otel_scope_info gauge
				otel_scope_info{otel_scope_name="a-name",otel_scope_version="a-version",something_extra="zzz-extra-value"} 1.0
				# TYPE test_metric_seconds gauge
				test_metric_seconds 1234.56
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Labels from data point",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"scope": {
							"name": "a-name",
							"version": "a-version",
							"attributes": [{
								"key": "something.extra",
								"value": { "stringValue": "zzz-extra-value" }
							}]
						},
						"metrics": [{
							"name": "test_metric_seconds",
							"gauge": {
								"data_points": [{
									"attributes": [{
										"key": "foo",
										"value": { "stringValue": "bar" }
									}],
									"as_double": 1234.56
								}]
							}
						}]
					}]
				}]
			}`,
			includeScopeLabels: true,
			expect: `
				# TYPE test_metric_seconds gauge
				test_metric_seconds{otel_scope_name="a-name",otel_scope_version="a-version",foo="bar"} 1234.56
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Target info metric",
			input: `{
				"resource_metrics": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "myservice" }
						}, {
							"key": "service.instance.id",
							"value": { "stringValue": "instance" }
						}, {
							"key": "custom_attr",
							"value": { "stringValue": "test" }
						}]
					},
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_seconds",
							"gauge": {
								"data_points": [{
									"as_double": 1234.56
								}]
							}
						}]
					}]
				}]
			}`,
			includeTargetInfo: true,
			expect: `
				# HELP target_info Target metadata
				# TYPE target_info gauge
				target_info{instance="instance",job="myservice",custom_attr="test"} 1.0
				# TYPE test_metric_seconds gauge
				test_metric_seconds{instance="instance",job="myservice"} 1234.56
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Gauge: add_metric_suffixes = false",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric",
							"unit": "seconds",
							"gauge": {
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"as_double": 1234.56
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric gauge
				test_metric 1234.56
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Gauge: add_metric_suffixes = true",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric",
							"unit": "seconds",
							"gauge": {
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"as_double": 1234.56
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_seconds gauge
				test_metric_seconds 1234.56
			`,
			addMetricSuffixes: true,
			enableOpenMetrics: true,
		},
		{
			name: "Monotonic sum: add_metric_suffixes = false",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_total",
							"unit": "seconds",
							"sum": {
								"aggregation_temporality": 2,
								"is_monotonic": true,
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"as_double": 15,
									"exemplars":[
										{
											"time_unix_nano": 1000000001,
											"as_double": 0.3,
											"span_id": "aaaaaaaaaaaaaaaa",
											"trace_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
										}
									]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric counter
				test_metric_total 15.0 # {span_id="aaaaaaaaaaaaaaaa",trace_id="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} 0.3
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Monotonic sum: add_metric_suffixes = true",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_total",
							"unit": "seconds",
							"sum": {
								"aggregation_temporality": 2,
								"is_monotonic": true,
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"as_double": 15,
									"exemplars":[
										{
											"time_unix_nano": 1000000001,
											"as_double": 0.3,
											"span_id": "aaaaaaaaaaaaaaaa",
											"trace_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
										}
									]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_seconds counter
				test_metric_seconds_total 15.0 # {span_id="aaaaaaaaaaaaaaaa",trace_id="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} 0.3
			`,
			addMetricSuffixes: true,
			enableOpenMetrics: true,
		},
		{
			name: "Monotonic sum: add_metric_suffixes = false, don't convert to open metrics",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric",
							"unit": "seconds",
							"sum": {
								"aggregation_temporality": 2,
								"is_monotonic": true,
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"as_double": 15,
									"exemplars":[
										{
											"time_unix_nano": 1000000001,
											"as_double": 0.3,
											"span_id": "aaaaaaaaaaaaaaaa",
											"trace_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
										}
									]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric counter
				test_metric 15
			`,
			enableOpenMetrics: false,
		},
		{
			name: "Monotonic sum: add_metric_suffixes = true, don't convert to open metrics",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric",
							"unit": "seconds",
							"sum": {
								"aggregation_temporality": 2,
								"is_monotonic": true,
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"as_double": 15,
									"exemplars":[
										{
											"time_unix_nano": 1000000001,
											"as_double": 0.3,
											"span_id": "aaaaaaaaaaaaaaaa",
											"trace_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
										}
									]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_seconds_total counter
				test_metric_seconds_total 15
			`,
			addMetricSuffixes: true,
			enableOpenMetrics: false,
		},
		{
			name: "Non-monotonic sum: add_metric_suffixes = false",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric",
							"unit": "seconds",
							"sum": {
								"aggregation_temporality": 2,
								"is_monotonic": false,
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"as_double": 15
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric gauge
				test_metric 15.0
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Non-monotonic sum: add_metric_suffixes = true",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric",
							"unit": "seconds",
							"sum": {
								"aggregation_temporality": 2,
								"is_monotonic": false,
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"as_double": 15
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_seconds gauge
				test_metric_seconds 15.0
			`,
			addMetricSuffixes: true,
			enableOpenMetrics: true,
		},
		{
			name: "Histogram: add_metric_suffixes = false",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric",
							"unit": "seconds",
							"histogram": {
								"aggregation_temporality": 2,
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"count": 333,
									"sum": 100,
									"bucket_counts": [0, 111, 0, 222],
									"explicit_bounds": [0.25, 0.5, 0.75, 1.0],
									"exemplars":[
										{
											"time_unix_nano": 1000000001,
											"as_double": 0.3,
											"span_id": "aaaaaaaaaaaaaaaa",
											"trace_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
										},
										{
											"time_unix_nano": 1000000003,
											"as_double": 1.5,
											"span_id": "cccccccccccccccc",
											"trace_id": "cccccccccccccccccccccccccccccccc"
										},
										{
											"time_unix_nano": 1000000002,
											"as_double": 0.5,
											"span_id": "bbbbbbbbbbbbbbbb",
											"trace_id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
										}
									]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric histogram
				test_metric_bucket{le="0.25"} 0
				test_metric_bucket{le="0.5"} 111 # {span_id="aaaaaaaaaaaaaaaa",trace_id="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} 0.3
				test_metric_bucket{le="0.75"} 111 # {span_id="bbbbbbbbbbbbbbbb",trace_id="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"} 0.5
				test_metric_bucket{le="1.0"} 333
				test_metric_bucket{le="+Inf"} 333 # {span_id="cccccccccccccccc",trace_id="cccccccccccccccccccccccccccccccc"} 1.5
				test_metric_sum 100.0
				test_metric_count 333
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Histogram: add_metric_suffixes = true",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric",
							"unit": "seconds",
							"histogram": {
								"aggregation_temporality": 2,
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"count": 333,
									"sum": 100,
									"bucket_counts": [0, 111, 0, 222],
									"explicit_bounds": [0.25, 0.5, 0.75, 1.0],
									"exemplars":[
										{
											"time_unix_nano": 1000000001,
											"as_double": 0.3,
											"span_id": "aaaaaaaaaaaaaaaa",
											"trace_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
										},
										{
											"time_unix_nano": 1000000003,
											"as_double": 1.5,
											"span_id": "cccccccccccccccc",
											"trace_id": "cccccccccccccccccccccccccccccccc"
										},
										{
											"time_unix_nano": 1000000002,
											"as_double": 0.5,
											"span_id": "bbbbbbbbbbbbbbbb",
											"trace_id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
										}
									]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_seconds histogram
				test_metric_seconds_bucket{le="0.25"} 0
				test_metric_seconds_bucket{le="0.5"} 111 # {span_id="aaaaaaaaaaaaaaaa",trace_id="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} 0.3
				test_metric_seconds_bucket{le="0.75"} 111 # {span_id="bbbbbbbbbbbbbbbb",trace_id="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"} 0.5
				test_metric_seconds_bucket{le="1.0"} 333
				test_metric_seconds_bucket{le="+Inf"} 333 # {span_id="cccccccccccccccc",trace_id="cccccccccccccccccccccccccccccccc"} 1.5
				test_metric_seconds_sum 100.0
				test_metric_seconds_count 333
			`,
			addMetricSuffixes: true,
			enableOpenMetrics: true,
		},
		{
			name: "Summary: add_metric_suffixes = false",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric",
							"unit": "seconds",
							"summary": {
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"count": 333,
									"sum": 100,
									"quantile_values": [
										{ "quantile": 0, "value": 100 },
										{ "quantile": 0.25, "value": 200 },
										{ "quantile": 0.5, "value": 300 },
										{ "quantile": 0.75, "value": 400 },
										{ "quantile": 1, "value": 500 }
									]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric summary
				test_metric{quantile="0.0"} 100.0
				test_metric{quantile="0.25"} 200.0
				test_metric{quantile="0.5"} 300.0
				test_metric{quantile="0.75"} 400.0
				test_metric{quantile="1.0"} 500.0
				test_metric_sum 100.0
				test_metric_count 333
			`,
			enableOpenMetrics: true,
		},
		{
			name: "Summary: add_metric_suffixes = true",
			input: `{
				"resource_metrics": [{
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric",
							"unit": "seconds",
							"summary": {
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"count": 333,
									"sum": 100,
									"quantile_values": [
										{ "quantile": 0, "value": 100 },
										{ "quantile": 0.25, "value": 200 },
										{ "quantile": 0.5, "value": 300 },
										{ "quantile": 0.75, "value": 400 },
										{ "quantile": 1, "value": 500 }
									]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_seconds summary
				test_metric_seconds{quantile="0.0"} 100.0
				test_metric_seconds{quantile="0.25"} 200.0
				test_metric_seconds{quantile="0.5"} 300.0
				test_metric_seconds{quantile="0.75"} 400.0
				test_metric_seconds{quantile="1.0"} 500.0
				test_metric_seconds_sum 100.0
				test_metric_seconds_count 333
			`,
			addMetricSuffixes: true,
			enableOpenMetrics: true,
		},
		{
			name: "Gauge: convert resource attributes to metric label",
			input: `{
				"resource_metrics": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "myservice" }
						}, {
							"key": "service.instance.id",
							"value": { "stringValue": "instance" }
						}, {
							"key": "raw",
							"value": { "stringValue": "test" }
						},{
							"key": "foo.one",
							"value": { "stringValue": "foo" }
						}, {
							"key": "bar.one",
							"value": { "stringValue": "bar" }
						}]
					},
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_gauge",
							"gauge": {
								"data_points": [{
									"as_double": 1234.56
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_gauge gauge
				test_metric_gauge{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test"} 1234.56
			`,
			enableOpenMetrics:             true,
			resourceToTelemetryConversion: true,
		},
		{
			name: "Gauge: NOT convert resource attributes to metric label",
			input: `{
				"resource_metrics": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "myservice" }
						}, {
							"key": "service.instance.id",
							"value": { "stringValue": "instance" }
						}, {
							"key": "raw",
							"value": { "stringValue": "test" }
						},{
							"key": "foo.one",
							"value": { "stringValue": "foo" }
						}, {
							"key": "bar.one",
							"value": { "stringValue": "bar" }
						}]
					},
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_gauge",
							"gauge": {
								"data_points": [{
									"as_double": 1234.56
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_gauge gauge
				test_metric_gauge{instance="instance",job="myservice"} 1234.56
			`,
			enableOpenMetrics:             true,
			resourceToTelemetryConversion: false,
		},
		{
			name: "Summary: convert resource attributes to metric label",
			input: `{
				"resource_metrics": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "myservice" }
						}, {
							"key": "service.instance.id",
							"value": { "stringValue": "instance" }
						}, {
							"key": "raw",
							"value": { "stringValue": "test" }
						},{
							"key": "foo.one",
							"value": { "stringValue": "foo" }
						}, {
							"key": "bar.one",
							"value": { "stringValue": "bar" }
						}]
					},
					"scope_metrics": [{
						"metrics": [{
							"name": "test_metric_summary",
							"unit": "seconds",
							"summary": {
								"data_points": [{
									"start_time_unix_nano": 1000000000,
									"time_unix_nano": 1000000000,
									"count": 333,
									"sum": 100,
									"quantile_values": [
										{ "quantile": 0, "value": 100 },
										{ "quantile": 0.5, "value": 400 },
										{ "quantile": 1, "value": 500 }
									]
								}]
							}
						}]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_summary summary
				test_metric_summary{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test",quantile="0.0"} 100.0
				test_metric_summary{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test",quantile="0.5"} 400.0
				test_metric_summary{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test",quantile="1.0"} 500.0
				test_metric_summary_sum{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test"} 100.0
				test_metric_summary_count{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test"} 333
			`,
			enableOpenMetrics:             true,
			resourceToTelemetryConversion: true,
		},
		{
			name: "Histogram: convert resource attributes to metric label",
			input: `{
				"resource_metrics": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "myservice" }
						}, {
							"key": "service.instance.id",
							"value": { "stringValue": "instance" }
						}, {
							"key": "raw",
							"value": { "stringValue": "test" }
						},{
							"key": "foo.one",
							"value": { "stringValue": "foo" }
						}, {
							"key": "bar.one",
							"value": { "stringValue": "bar" }
						}]
					},
					"scope_metrics": [{
						"metrics": [
							{
								"name": "test_metric_histogram",
								"unit": "seconds",
								"histogram": {
									"aggregation_temporality": 2,
									"data_points": [{
										"start_time_unix_nano": 1000000000,
										"time_unix_nano": 1000000000,
										"count": 333,
										"sum": 100,
										"bucket_counts": [0, 111, 0, 222],
										"explicit_bounds": [0.25, 0.5, 0.75, 1.0],
										"exemplars":[
											{
												"time_unix_nano": 1000000001,
												"as_double": 0.3,
												"span_id": "aaaaaaaaaaaaaaaa",
												"trace_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
											},
											{
												"time_unix_nano": 1000000003,
												"as_double": 1.5,
												"span_id": "cccccccccccccccc",
												"trace_id": "cccccccccccccccccccccccccccccccc"
											},
											{
												"time_unix_nano": 1000000002,
												"as_double": 0.5,
												"span_id": "bbbbbbbbbbbbbbbb",
												"trace_id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
											}
										]
									}]
								}
							}
						]
					}]
				}]
			}`,
			expect: `
				# TYPE test_metric_histogram histogram
				test_metric_histogram_bucket{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test",le="0.25"} 0
				test_metric_histogram_bucket{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test",le="0.5"} 111 # {span_id="aaaaaaaaaaaaaaaa",trace_id="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} 0.3
				test_metric_histogram_bucket{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test",le="0.75"} 111 # {span_id="bbbbbbbbbbbbbbbb",trace_id="bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"} 0.5
				test_metric_histogram_bucket{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test",le="1.0"} 333
				test_metric_histogram_bucket{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test",le="+Inf"} 333 # {span_id="cccccccccccccccc",trace_id="cccccccccccccccccccccccccccccccc"} 1.5
				test_metric_histogram_sum{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test"} 100.0
				test_metric_histogram_count{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test"} 333
			`,
			enableOpenMetrics:             true,
			resourceToTelemetryConversion: true,
		},
		{
			name: "Monotonic sum: convert resource attributes to metric label",
			input: `{
				"resource_metrics": [{
					"resource": {
						"attributes": [{
							"key": "service.name",
							"value": { "stringValue": "myservice" }
						}, {
							"key": "service.instance.id",
							"value": { "stringValue": "instance" }
						}, {
							"key": "raw",
							"value": { "stringValue": "test" }
						},{
							"key": "foo.one",
							"value": { "stringValue": "foo" }
						}, {
							"key": "bar.one",
							"value": { "stringValue": "bar" }
						}]
					},
					"scope_metrics": [{
						"metrics": [
							{
								"name": "test_metric_mono_sum_total",
								"description": "Total count of monotonic sum operations",
								"unit": "seconds",
								"sum": {
									"aggregation_temporality": 2,
									"is_monotonic": true,
									"data_points": [{
										"start_time_unix_nano": 1000000000,
										"time_unix_nano": 1000000000,
										"as_double": 15,
										"exemplars":[
											{
												"time_unix_nano": 1000000001,
												"as_double": 0.3,
												"span_id": "aaaaaaaaaaaaaaaa",
												"trace_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
											}
										]
									}]
								}
							}
						]
					}]
				}]
			}`,
			expect: `
				# HELP test_metric_mono_sum Total count of monotonic sum operations
				# TYPE test_metric_mono_sum counter
				test_metric_mono_sum_total{bar_one="bar",foo_one="foo",instance="instance",service_instance_id="instance",job="myservice",service_name="myservice",raw="test"} 15.0 # {span_id="aaaaaaaaaaaaaaaa",trace_id="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"} 0.3
			`,
			enableOpenMetrics:             true,
			resourceToTelemetryConversion: true,
		},
	}

	decoder := &pmetric.JSONUnmarshaler{}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := decoder.UnmarshalMetrics([]byte(tc.input))
			require.NoError(t, err)

			var app testappender.Appender
			app.HideTimestamps = !tc.showTimestamps

			l := util.TestLogger(t)
			conv := convert.New(l, appenderAppendable{Inner: &app}, convert.Options{
				IncludeTargetInfo:             tc.includeTargetInfo,
				IncludeScopeInfo:              tc.includeScopeInfo,
				IncludeScopeLabels:            tc.includeScopeLabels,
				AddMetricSuffixes:             tc.addMetricSuffixes,
				ResourceToTelemetryConversion: tc.resourceToTelemetryConversion,
				HonorMetadata:                 true,
			})
			require.NoError(t, conv.ConsumeMetrics(t.Context(), payload))

			families, err := app.MetricFamilies()
			require.NoError(t, err)

			c := testappender.Comparer{OpenMetrics: tc.enableOpenMetrics}
			require.NoError(t, c.Compare(families, tc.expect))
		})
	}
}

// Exponential histograms don't have a text format representation.
// In this test we are comparing the JSON format.
func TestConverterExponentialHistograms(t *testing.T) {
	input1 := `{
			"resource_metrics": [{
				"scope_metrics": [{
					"metrics": [{
						"name": "test_exponential_histogram",
						"description": "An exponential histogram for latency measurements",
						"exponential_histogram": {
							"aggregation_temporality": 2,
							"data_points": [{
								"start_time_unix_nano": 1000000000,
								"time_unix_nano": 1000000000,
								"scale": 0,
								"count": 11,
								"sum": 158.63,
								"positive": {
									"offset": -1,
									"bucket_counts": [2, 1, 3, 2, 0, 0, 3]
								},
								"exemplars":[
									{
										"time_unix_nano": 1000000001,
										"as_double": 3.0,
										"span_id": "aaaaaaaaaaaaaaaa",
										"trace_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
									},
									{
										"time_unix_nano": 1000000003,
										"as_double": 1.0,
										"span_id": "cccccccccccccccc",
										"trace_id": "cccccccccccccccccccccccccccccccc"
									},
									{
										"time_unix_nano": 1000000002,
										"as_double": 1.5,
										"span_id": "bbbbbbbbbbbbbbbb",
										"trace_id": "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
									}
								]
							}]
						}
					}]
				}]
			}]
		}`

	tt := []struct {
		name          string
		input         string
		expect        string
		honorMetadata bool
	}{
		{
			name:          "Exponential Histogram with metadata",
			honorMetadata: true,
			input:         input1,
			// The tests only allow one exemplar/series because it uses a map[series]exemplar as storage.
			// Therefore only the exemplar "cccccccccccccccc" is stored (bbbbbbbbbbbbbbbb is out-of-order).
			// The MetricFamily includes name, help, and type fields when HonorMetadata is enabled.
			// Type 4 = HISTOGRAM in the dto.MetricType enum.
			expect: `{
				"name": "test_exponential_histogram",
				"help": "An exponential histogram for latency measurements",
				"type": 4,
				"metric": [{
					"histogram": {
						"bucket": [{
							"exemplar": {
								"label": [
									{"name": "span_id", "value": "cccccccccccccccc"},
									{"name": "trace_id", "value": "cccccccccccccccccccccccccccccccc"}
								],
								"value": 1.0
							}
						}],
						"positive_delta": [2, -1, 2, -1, -2, 0, 3],
						"positive_span": [{"length": 7, "offset": 0}],
						"sample_count": 11,
						"sample_sum": 158.63,
						"schema": 0,
						"zero_count": 0,
						"zero_threshold": 1e-128
					}
				}]
			}`,
		},
		{
			name:          "Exponential Histogram without metadata",
			honorMetadata: false,
			input:         input1,
			// Without SendMetadata, the MetricFamily has type 3 (UNTYPED).
			// Note: exemplar bucket data is not present without proper metadata.
			expect: `{
				"name": "test_exponential_histogram",
				"type": 3,
				"metric": [{
					"histogram": {
						"positive_delta": [2, -1, 2, -1, -2, 0, 3],
						"positive_span": [{"length": 7, "offset": 0}],
						"sample_count": 11,
						"sample_sum": 158.63,
						"schema": 0,
						"zero_count": 0,
						"zero_threshold": 1e-128
					}
				}]
			}`,
		},
		{
			name:          "Exponential Histogram 2 with metadata",
			honorMetadata: true,
			input: `{
			"resource_metrics": [{
				"scope_metrics": [{
					"metrics": [{
						"name": "test_exponential_histogram_2",
						"description": "A second exponential histogram with negative buckets",
						"exponential_histogram": {
							"aggregation_temporality": 2,
							"data_points": [{
								"start_time_unix_nano": 1000000000,
								"time_unix_nano": 1000000000,
								"scale": 2,
								"count": 19,
								"sum": 200,
								"zero_count" : 5,
								"zero_threshold": 0.1,
								"positive": {
									"offset": 3,
									"bucket_counts": [0, 0, 0, 0, 2, 1, 1, 0, 3, 0, 0]
								},
								"negative": {
									"offset": 0,
									"bucket_counts": [0, 4, 0, 2, 3, 0, 0, 3]
								},
								"exemplars":[
									{
										"time_unix_nano": 1000000001,
										"as_double": 3.0,
										"span_id": "aaaaaaaaaaaaaaaa",
										"trace_id": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
									}
								]
							}]
						}
					}]
				}]
			}]
		}`,
			// zero_threshold is set to 1e-128 because dp.ZeroThreshold() is not yet available.
			// Type 4 = HISTOGRAM in the dto.MetricType enum.
			expect: `{
				"name": "test_exponential_histogram_2",
				"help": "A second exponential histogram with negative buckets",
				"type": 4,
				"metric": [{
					"histogram": {
						"bucket": [{
							"exemplar": {
								"label": [
									{"name": "span_id", "value": "aaaaaaaaaaaaaaaa"},
									{"name": "trace_id", "value": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}
								],
								"value": 3
							}
						}],
						"negative_delta": [0, 4, -4, 2, 1, -3, 0, 3],
						"negative_span": [{"length": 8, "offset": 1}],
						"positive_delta": [2, -1, 0, -1, 3, -3, 0],
						"positive_span": [
							{"length": 0, "offset": 4},
							{"length": 7, "offset": 4}
						],
						"sample_count": 19,
						"sample_sum": 200,
						"schema": 2,
						"zero_count": 5,
						"zero_threshold": 1e-128
					}
				}]
			}`,
		},
	}
	decoder := &pmetric.JSONUnmarshaler{}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := decoder.UnmarshalMetrics([]byte(tc.input))
			require.NoError(t, err)

			var app testappender.Appender
			l := util.TestLogger(t)
			conv := convert.New(l, appenderAppendable{Inner: &app}, convert.Options{
				HonorMetadata: tc.honorMetadata,
			})
			require.NoError(t, conv.ConsumeMetrics(t.Context(), payload))

			families, err := app.MetricFamilies()
			require.NoError(t, err)

			require.NotEmpty(t, families)
			require.NotNil(t, families[0])

			// Marshal the entire MetricFamily to get a complete representation
			// including metadata (name, type, help) when SendMetadata is enabled.
			familyJsonRep, err := json.Marshal(families[0])
			require.NoError(t, err)
			require.JSONEq(t, tc.expect, string(familyJsonRep))
		})
	}
}

// appenderAppendable always returns the same Appender.
type appenderAppendable struct {
	Inner storage.Appender
}

var _ storage.Appendable = appenderAppendable{}

func (aa appenderAppendable) Appender(context.Context) storage.Appender {
	return aa.Inner
}

// TestMetadataWrittenAfterSeries verifies that metadata is written after series
// data, so that appenders like the WAL (which require series to exist before
// metadata can be updated) work correctly.
func TestMetadataWrittenAfterSeries(t *testing.T) {
	input := `{
		"resource_metrics": [{
			"scope_metrics": [{
				"metrics": [{
					"name": "traces_service_graph_request_server_seconds",
					"description": "Histogram of request durations",
					"histogram": {
						"aggregation_temporality": 2,
						"data_points": [{
							"start_time_unix_nano": 1000000000,
							"time_unix_nano": 1000000000,
							"count": 10,
							"sum": 5.5,
							"bucket_counts": [2, 5, 3],
							"explicit_bounds": [0.1, 0.5, 1.0]
						}]
					}
				}]
			}]
		}]
	}`

	decoder := &pmetric.JSONUnmarshaler{}
	payload, err := decoder.UnmarshalMetrics([]byte(input))
	require.NoError(t, err)

	// strictMetadataAppender requires series to exist before metadata can be updated,
	// mimicking the behavior of the WAL appender.
	app := &strictMetadataAppender{
		metricFamilies: make(map[string]bool),
	}

	l := util.TestLogger(t)

	// With HonorMetadata enabled, there should be NO metadata errors because
	// the series is now created before metadata is written.
	conv := convert.New(l, appenderAppendable{Inner: app}, convert.Options{
		HonorMetadata: true,
	})
	err = conv.ConsumeMetrics(t.Context(), payload)
	require.NoError(t, err)

	// Verify that NO metadata errors occurred (the fix ensures series exists before metadata)
	require.Equal(t, 0, app.metadataErrorCount, "expected no metadata errors when series is created before metadata")

	// Verify that metadata was actually written
	require.True(t, app.metadataWriteCount > 0, "expected metadata to be written when HonorMetadata is true")

	// Reset the appender
	app = &strictMetadataAppender{
		metricFamilies: make(map[string]bool),
	}

	// With HonorMetadata disabled, no metadata should be written at all
	conv = convert.New(l, appenderAppendable{Inner: app}, convert.Options{
		HonorMetadata: false,
	})
	err = conv.ConsumeMetrics(t.Context(), payload)
	require.NoError(t, err)

	require.Equal(t, 0, app.metadataErrorCount, "expected no metadata errors when HonorMetadata is false")
	require.Equal(t, 0, app.metadataWriteCount, "expected no metadata writes when HonorMetadata is false")
}

// strictMetadataAppender is a test appender that mimics the WAL behavior:
// UpdateMetadata fails if the series doesn't exist.
// It tracks metric family names (base names without suffixes) to allow metadata
// to be matched against series with _sum, _count, _bucket suffixes.
type strictMetadataAppender struct {
	metricFamilies     map[string]bool // metric family names (base names)
	metadataErrorCount int
	metadataWriteCount int
}

func (a *strictMetadataAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	// Extract the metric family name (base name without suffixes)
	name := l.Get("__name__")
	a.metricFamilies[getMetricFamilyName(name)] = true
	return 0, nil
}

func (a *strictMetadataAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	return 0, nil
}

func (a *strictMetadataAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	// Mimic WAL behavior: fail if no series with this metric family exists
	name := l.Get("__name__")
	if !a.metricFamilies[name] {
		a.metadataErrorCount++
		return 0, fmt.Errorf("unknown series when trying to add metadata with SeriesRef: %d and labels: %s", ref, l)
	}
	a.metadataWriteCount++
	return 0, nil
}

func (a *strictMetadataAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	// Extract the metric family name (base name without suffixes)
	name := l.Get("__name__")
	a.metricFamilies[getMetricFamilyName(name)] = true
	return 0, nil
}

// getMetricFamilyName returns the base metric name without histogram/summary suffixes.
func getMetricFamilyName(name string) string {
	// Remove histogram suffixes
	for _, suffix := range []string{"_bucket", "_sum", "_count", "_total"} {
		if before, ok := strings.CutSuffix(name, suffix); ok {
			return before
		}
	}
	return name
}

func (a *strictMetadataAppender) Commit() error {
	return nil
}

func (a *strictMetadataAppender) Rollback() error {
	return nil
}

func (a *strictMetadataAppender) AppendSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64) (storage.SeriesRef, error) {
	return 0, nil
}

func (a *strictMetadataAppender) AppendHistogramSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return 0, nil
}

func (a *strictMetadataAppender) SetOptions(o *storage.AppendOptions) {}
