package stages

import (
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
)

func TestMetricsStage(t *testing.T) {
	type testCase struct {
		name            string
		config          string
		entries         []Entry
		expectedMetrics string
	}

	now := time.Now()

	tests := []testCase{
		{
			name: "counter, gauge and histogram",
			config: `
			stage.metrics {
				metric.counter {
					name = "loki_count"
					description = "uhhhhhhh"
					prefix = "my_agent_custom_"
					source = "app"
					value = "loki"
					action = "inc"
				}
				metric.gauge {
					name = "bloki_count"
					description = "blerrrgh"
					source = "app"
					value = "bloki"
					action = "dec"
				}
				metric.counter {
					name = "total_lines_count"
					description = "nothing to see here..."
					match_all = true
					action = "inc"
				}
				metric.counter {
					name = "total_bytes_count"
					description = "nothing to see here..."
					match_all = true
					count_entry_bytes = true
					action = "add"
				}
				metric.histogram {
					name = "payload_size_bytes"
					description = "grrrragh"
					source = "payload"
					buckets = [10, 20]
				}
			}
			`,
			entries: []Entry{
				newEntry(
					map[string]any{"app": "loki", "payload": float64(10)},
					model.LabelSet{"test": "app"},
					`{"time":"2012-11-01T22:08:41+00:00", "app":"loki", "payload": 10, "component": ["parser","type"], "level" : "WARN"}`,
					now,
				),
				newEntry(
					map[string]any{"app": "bloki", "payload": float64(20)},
					model.LabelSet{"test": "app"},
					`{"time":"2012-11-01T22:08:41+00:00", "app":"bloki", "payload": 20, "component": ["parser","type"], "level" : "WARN"}`,
					now,
				),
			},
			expectedMetrics: `
# HELP my_agent_custom_loki_count uhhhhhhh
# TYPE my_agent_custom_loki_count counter
my_agent_custom_loki_count{test="app"} 1
# HELP loki_process_custom_bloki_count blerrrgh
# TYPE loki_process_custom_bloki_count gauge
loki_process_custom_bloki_count{test="app"} -1
# HELP loki_process_custom_payload_size_bytes grrrragh
# TYPE loki_process_custom_payload_size_bytes histogram
loki_process_custom_payload_size_bytes_bucket{test="app",le="10"} 1
loki_process_custom_payload_size_bytes_bucket{test="app",le="20"} 2
loki_process_custom_payload_size_bytes_bucket{test="app",le="+Inf"} 2
loki_process_custom_payload_size_bytes_sum{test="app"} 30
loki_process_custom_payload_size_bytes_count{test="app"} 2
# HELP loki_process_custom_total_bytes_count nothing to see here...
# TYPE loki_process_custom_total_bytes_count counter
loki_process_custom_total_bytes_count{test="app"} 231
# HELP loki_process_custom_total_lines_count nothing to see here...
# TYPE loki_process_custom_total_lines_count counter
loki_process_custom_total_lines_count{test="app"} 2
			`,
		},
		{
			name: "negative gauge",
			config: `
			stage.metrics {
				metric.gauge {
					name = "longitude"
					description = "longitude GPS vehicle"
					action = "set"
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"longitude": "-10.1234"}, model.LabelSet{"test": "app", "vehicle": "1"}, "not important", now),
			},
			expectedMetrics: `
# HELP loki_process_custom_longitude longitude GPS vehicle
# TYPE loki_process_custom_longitude gauge
loki_process_custom_longitude{test="app",vehicle="1"} -10.1234
`,
		},
		{
			name: "missing source do not generate metrics",
			config: `
			stage.metrics {
				metric.counter {
					name = "loki_count"
					description = "uhhhhhhh"
					prefix = "my_agent_custom_"
					source = "app"
					value = "loki"
					action = "inc"
				}
				metric.gauge {
					name = "bloki_count"
					description = "blerrrgh"
					source = "app"
					value = "bloki"
					action = "dec"
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"app": nil}, model.LabelSet{}, "not important", now),
			},
			expectedMetrics: "",
		},
		{
			name: "non-prometheus incoming label is dropped from counter",
			config: `
			stage.metrics {
				metric.counter {
					name = "loki_count"
					source = "app"
					description = "should count all entries"
					match_all = true
					action = "inc"
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"good_label": "1", "__bad_label__": "2"}, "not important", now),
			},
			expectedMetrics: `
# HELP loki_process_custom_loki_count should count all entries
# TYPE loki_process_custom_loki_count counter
loki_process_custom_loki_count{good_label="1"} 1
`,
		},
		{
			name: "tenant step injected label is dropped from counter",
			config: `
			stage.metrics {
				metric.counter {
					name = "loki_count"
					source = "app"
					description = "should count all entries"
					match_all = true
					action = "inc"
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{}, model.LabelSet{"good_label": "1", ReservedLabelTenantID: "2"}, "not important", now),
			},
			expectedMetrics: `
# HELP loki_process_custom_loki_count should count all entries
# TYPE loki_process_custom_loki_count counter
loki_process_custom_loki_count{good_label="1"} 1
`,
		},
		{
			name: "non-prometheus incoming label is dropped from gauge",
			config: `
			stage.metrics {
				metric.gauge {
					name = "longitude"
					description = "longitude GPS vehicle"
					action = "set"
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"longitude": "-10.1234"}, model.LabelSet{"vehicle": "1", "__bad_label__": "2"}, "not important", now),
			},
			expectedMetrics: `
# HELP loki_process_custom_longitude longitude GPS vehicle
# TYPE loki_process_custom_longitude gauge
loki_process_custom_longitude{vehicle="1"} -10.1234
`,
		},
		{
			name: "non-prometheus incoming label is dropped from histogram",
			config: `
			stage.metrics {
				metric.histogram {
					name = "payload_size_bytes"
					description = "payload size in bytes"
					source = "payload"
					buckets = [10, 20]
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{"payload": float64(10)}, model.LabelSet{"test": "app", "__bad_label__": "2"}, "not important", now),
			},
			expectedMetrics: `
# HELP loki_process_custom_payload_size_bytes payload size in bytes
# TYPE loki_process_custom_payload_size_bytes histogram
loki_process_custom_payload_size_bytes_bucket{test="app",le="10"} 1
loki_process_custom_payload_size_bytes_bucket{test="app",le="20"} 1
loki_process_custom_payload_size_bytes_bucket{test="app",le="+Inf"} 1
loki_process_custom_payload_size_bytes_sum{test="app"} 10
loki_process_custom_payload_size_bytes_count{test="app"} 1
`,
		},
		{
			name: "same extracted values with different labels track independently",
			config: `
			stage.metrics {
				metric.counter {
					name = "total_keys"
					description = "the total keys per doc"
					source = "total_keys"
					action = "add"
				}
				metric.histogram {
					name = "keys_per_line"
					description = "keys per doc"
					source = "keys_per_line"
					buckets = [1, 3, 5, 10]
				}
				metric.gauge {
					name = "numeric_float"
					description = "numeric_float"
					source = "numeric_float"
					action = "add"
				}
				metric.gauge {
					name = "numeric_integer"
					description = "numeric.integer"
					source = "numeric_integer"
					action = "add"
				}
				metric.gauge {
					name = "numeric_string"
					description = "numeric.string"
					source = "numeric_string"
					action = "add"
				}
				metric.counter {
					name = "contains_warn"
					description = "contains_warn"
					source = "contains_warn"
					value = "true"
					action = "inc"
				}
				metric.counter {
					name = "matches"
					source = "time"
					description = "all matches"
					action = "inc"
				}
				metric.histogram {
					name = "response_time_seconds"
					source = "time"
					description = "response time in ms"
					buckets = [0.5, 1, 2]
				}
			}
			`,
			entries: []Entry{
				newEntry(map[string]any{
					"total_keys":      float64(8),
					"keys_per_line":   float64(8),
					"numeric_float":   12.34,
					"numeric_integer": float64(123),
					"numeric_string":  "123",
					"contains_warn":   true,
					"time":            "932ms",
				}, model.LabelSet{"foo": "bar", "bar": "foo"}, "not important", now),
				newEntry(map[string]any{
					"total_keys":      float64(8),
					"keys_per_line":   float64(8),
					"numeric_float":   12.34,
					"numeric_integer": float64(123),
					"numeric_string":  "123",
					"contains_warn":   true,
					"time":            "932ms",
				}, model.LabelSet{"fu": "baz", "baz": "fu"}, "not important", now),
			},
			expectedMetrics: `
# HELP loki_process_custom_contains_warn contains_warn
# TYPE loki_process_custom_contains_warn counter
loki_process_custom_contains_warn{bar="foo",foo="bar"} 1.0
loki_process_custom_contains_warn{baz="fu",fu="baz"} 1.0
# HELP loki_process_custom_keys_per_line keys per doc
# TYPE loki_process_custom_keys_per_line histogram
loki_process_custom_keys_per_line_bucket{bar="foo",foo="bar",le="1.0"} 0.0
loki_process_custom_keys_per_line_bucket{bar="foo",foo="bar",le="3.0"} 0.0
loki_process_custom_keys_per_line_bucket{bar="foo",foo="bar",le="5.0"} 0.0
loki_process_custom_keys_per_line_bucket{bar="foo",foo="bar",le="10.0"} 1.0
loki_process_custom_keys_per_line_bucket{bar="foo",foo="bar",le="+Inf"} 1.0
loki_process_custom_keys_per_line_sum{bar="foo",foo="bar"} 8.0
loki_process_custom_keys_per_line_count{bar="foo",foo="bar"} 1.0
loki_process_custom_keys_per_line_bucket{baz="fu",fu="baz",le="1.0"} 0.0
loki_process_custom_keys_per_line_bucket{baz="fu",fu="baz",le="3.0"} 0.0
loki_process_custom_keys_per_line_bucket{baz="fu",fu="baz",le="5.0"} 0.0
loki_process_custom_keys_per_line_bucket{baz="fu",fu="baz",le="10.0"} 1.0
loki_process_custom_keys_per_line_bucket{baz="fu",fu="baz",le="+Inf"} 1.0
loki_process_custom_keys_per_line_sum{baz="fu",fu="baz"} 8.0
loki_process_custom_keys_per_line_count{baz="fu",fu="baz"} 1.0
# HELP loki_process_custom_matches all matches
# TYPE loki_process_custom_matches counter
loki_process_custom_matches{bar="foo",foo="bar"} 1.0
loki_process_custom_matches{baz="fu",fu="baz"} 1.0
# HELP loki_process_custom_numeric_float numeric_float
# TYPE loki_process_custom_numeric_float gauge
loki_process_custom_numeric_float{bar="foo",foo="bar"} 12.34
loki_process_custom_numeric_float{baz="fu",fu="baz"} 12.34
# HELP loki_process_custom_numeric_integer numeric.integer
# TYPE loki_process_custom_numeric_integer gauge
loki_process_custom_numeric_integer{bar="foo",foo="bar"} 123.0
loki_process_custom_numeric_integer{baz="fu",fu="baz"} 123.0
# HELP loki_process_custom_numeric_string numeric.string
# TYPE loki_process_custom_numeric_string gauge
loki_process_custom_numeric_string{bar="foo",foo="bar"} 123.0
loki_process_custom_numeric_string{baz="fu",fu="baz"} 123.0
# HELP loki_process_custom_response_time_seconds response time in ms
# TYPE loki_process_custom_response_time_seconds histogram
loki_process_custom_response_time_seconds_bucket{bar="foo",foo="bar",le="0.5"} 0
loki_process_custom_response_time_seconds_bucket{bar="foo",foo="bar",le="1"} 1
loki_process_custom_response_time_seconds_bucket{bar="foo",foo="bar",le="2"} 1
loki_process_custom_response_time_seconds_bucket{bar="foo",foo="bar",le="+Inf"} 1
loki_process_custom_response_time_seconds_sum{bar="foo",foo="bar"} 0.932
loki_process_custom_response_time_seconds_count{bar="foo",foo="bar"} 1
loki_process_custom_response_time_seconds_bucket{baz="fu",fu="baz",le="0.5"} 0
loki_process_custom_response_time_seconds_bucket{baz="fu",fu="baz",le="1"} 1
loki_process_custom_response_time_seconds_bucket{baz="fu",fu="baz",le="2"} 1
loki_process_custom_response_time_seconds_bucket{baz="fu",fu="baz",le="+Inf"} 1
loki_process_custom_response_time_seconds_sum{baz="fu",fu="baz"} 0.932
loki_process_custom_response_time_seconds_count{baz="fu",fu="baz"} 1.0
# HELP loki_process_custom_total_keys the total keys per doc
# TYPE loki_process_custom_total_keys counter
loki_process_custom_total_keys{bar="foo",foo="bar"} 8.0
loki_process_custom_total_keys{baz="fu",fu="baz"} 8.0
			`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// This stage never mutates entries, so expected always equals entries.
			runPipelineTest(t, loadConfig(tt.config), tt.entries, tt.entries, entryCheckFNs{
				metrics: func(reg *prometheus.Registry) error {
					return testutil.GatherAndCompare(reg, strings.NewReader(tt.expectedMetrics))
				},
				metricsAfterCleanup: func(reg *prometheus.Registry) error {
					return testutil.GatherAndCompare(reg, strings.NewReader(""))
				},
			})
		})
	}
}
