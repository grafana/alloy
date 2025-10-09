package stages

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/loki/process/metric"
	"github.com/grafana/alloy/internal/featuregate"
)

var testMetricAlloy = `
stage.json {
		expressions = { "app" = "app", "payload" = "payload" }
}
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
} `

var testMetricLogLine1 = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"loki",
    "payload": 10,
	"component": ["parser","type"],
	"level" : "WARN"
}
`

var testMetricLogLine2 = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"app":"bloki",
    "payload": 20,
	"component": ["parser","type"],
	"level" : "WARN"
}
`

var testMetricLogLineWithMissingKey = `
{
	"time":"2012-11-01T22:08:41+00:00",
	"payload": 20,
	"component": ["parser","type"],
	"level" : "WARN"
}
`

const expectedMetrics = `# HELP my_agent_custom_loki_count uhhhhhhh
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
loki_process_custom_total_bytes_count{test="app"} 255
# HELP loki_process_custom_total_lines_count nothing to see here...
# TYPE loki_process_custom_total_lines_count counter
loki_process_custom_total_lines_count{test="app"} 2
`

func TestMetricsPipeline(t *testing.T) {
	registry := prometheus.NewRegistry()
	pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testMetricAlloy), nil, registry, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}

	out := <-pl.Run(withInboundEntries(newEntry(nil, model.LabelSet{"test": "app"}, testMetricLogLine1, time.Now())))
	out.Line = testMetricLogLine2
	<-pl.Run(withInboundEntries(out))

	if err := testutil.GatherAndCompare(registry,
		strings.NewReader(expectedMetrics)); err != nil {
		t.Fatalf("mismatch metrics: %v", err)
	}

	pl.Cleanup()

	if err := testutil.GatherAndCompare(registry,
		strings.NewReader("")); err != nil {
		t.Fatalf("mismatch metrics: %v", err)
	}
}

func TestNegativeGauge(t *testing.T) {
	registry := prometheus.NewRegistry()
	testConfig := `
stage.regex {
		expression = "vehicle=(?P<vehicle>\\d+) longitude=(?P<longitude>[-]?\\d+\\.\\d+) latitude=(?P<latitude>\\d+\\.\\d+)"
}
stage.labels {
		values = { "vehicle" = "" }
}
stage.metrics {
		metric.gauge {
				name = "longitude"
				description = "longitude GPS vehicle"
				action = "set"
		}
} `
	pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testConfig), nil, registry, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}

	<-pl.Run(withInboundEntries(newEntry(nil, model.LabelSet{"test": "app"}, `#<13>Jan 28 14:25:52 vehicle=1 longitude=-10.1234 latitude=15.1234`, time.Now())))
	if err := testutil.GatherAndCompare(registry,
		strings.NewReader(`
# HELP loki_process_custom_longitude longitude GPS vehicle
# TYPE loki_process_custom_longitude gauge
loki_process_custom_longitude{test="app",vehicle="1"} -10.1234
`)); err != nil {
		t.Fatalf("mismatch metrics: %v", err)
	}
}

func TestPipelineWithMissingKey_Metrics(t *testing.T) {
	var buf bytes.Buffer
	w := log.NewSyncWriter(&buf)
	logger := log.NewLogfmtLogger(w)
	pl, err := NewPipeline(logger, loadConfig(testMetricAlloy), nil, prometheus.DefaultRegisterer, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}
	Debug = true
	processEntries(pl, newEntry(nil, nil, testMetricLogLineWithMissingKey, time.Now()))
	expectedLog := "level=debug msg=\"failed to convert extracted value to string, can't perform value comparison\" metric=bloki_count err=\"can't convert <nil> to string\""
	if !(strings.Contains(buf.String(), expectedLog)) {
		t.Errorf("\nexpected: %s\n+actual: %s", expectedLog, buf.String())
	}
}

var testMetricWithDropAlloy = `
stage.json {
		expressions = { "app" = "app", "drop" = "drop" }
}
stage.match {
		selector = "{drop=\"true\"}"
		action = "drop"
}
stage.metrics {
		metric.counter {
				name = "loki_count"
				source = "app"
				description = "should only inc on non dropped labels"
				action = "inc"
		}
} `

const expectedDropMetrics = `# HELP loki_process_dropped_lines_total A count of all log lines dropped as a result of a pipeline stage
# TYPE loki_process_dropped_lines_total counter
loki_process_dropped_lines_total{reason="match_stage"} 1
# HELP loki_process_custom_loki_count should only inc on non dropped labels
# TYPE loki_process_custom_loki_count counter
loki_process_custom_loki_count 1
`

func TestMetricsWithDropInPipeline(t *testing.T) {
	registry := prometheus.NewRegistry()
	pl, err := NewPipeline(log.NewNopLogger(), loadConfig(testMetricWithDropAlloy), nil, registry, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatal(err)
	}
	lbls := model.LabelSet{}
	droppingLabels := model.LabelSet{
		"drop": "true",
	}
	in := make(chan Entry)
	out := pl.Run(in)

	in <- newEntry(nil, lbls, testMetricLogLine1, time.Now())
	e := <-out
	e.Labels = droppingLabels
	e.Line = testMetricLogLine2
	in <- e
	close(in)
	<-out

	if err := testutil.GatherAndCompare(registry,
		strings.NewReader(expectedDropMetrics)); err != nil {
		t.Fatalf("mismatch metrics: %v", err)
	}
}

var testMetricWithNonPromLabel = `
stage.static_labels {
		values = { "good_label" = "1" }
}
stage.metrics {
		metric.counter {
				name = "loki_count"
				source = "app"
				description = "should count all entries"
				match_all = true
				action = "inc"
		}
} `

func TestNonPrometheusLabelsShouldBeDropped(t *testing.T) {
	const counterConfig = `
stage.static_labels {
		values = { "good_label" = "1" }
}
stage.tenant {
		value = "2"
}
stage.metrics {
		metric.counter {
				name = "loki_count"
				source = "app"
				description = "should count all entries"
				match_all = true
				action = "inc"
		}
} `

	const expectedCounterMetrics = `# HELP loki_process_custom_loki_count should count all entries
# TYPE loki_process_custom_loki_count counter
loki_process_custom_loki_count{good_label="1"} 1
`

	const gaugeConfig = `
stage.regex {
		expression = "vehicle=(?P<vehicle>\\d+) longitude=(?P<longitude>[-]?\\d+\\.\\d+) latitude=(?P<latitude>\\d+\\.\\d+)"
}
stage.labels {
		values = { "vehicle" = "" }
}
stage.metrics {
		metric.gauge {
				name = "longitude"
				description = "longitude GPS vehicle"
				action = "set"
		}
}`

	const expectedGaugeMetrics = `# HELP loki_process_custom_longitude longitude GPS vehicle
# TYPE loki_process_custom_longitude gauge
loki_process_custom_longitude{vehicle="1"} -10.1234
`

	const histogramConfig = `
stage.json {
		expressions = { "payload" = "payload" }
}
stage.metrics {
		metric.histogram {
				name = "payload_size_bytes"
				description = "payload size in bytes"
				source = "payload"
				buckets = [10, 20]
		}
}`

	const expectedHistogramMetrics = `# HELP loki_process_custom_payload_size_bytes payload size in bytes
# TYPE loki_process_custom_payload_size_bytes histogram
loki_process_custom_payload_size_bytes_bucket{test="app",le="10"} 1
loki_process_custom_payload_size_bytes_bucket{test="app",le="20"} 1
loki_process_custom_payload_size_bytes_bucket{test="app",le="+Inf"} 1
loki_process_custom_payload_size_bytes_sum{test="app"} 10
loki_process_custom_payload_size_bytes_count{test="app"} 1
`
	for name, tc := range map[string]struct {
		promtailConfig  string
		labels          model.LabelSet
		line            string
		expectedCollect string
	}{
		"counter metric with non-prometheus incoming label": {
			promtailConfig: testMetricWithNonPromLabel,
			labels: model.LabelSet{
				"__bad_label__": "2",
			},
			line:            testMetricLogLine1,
			expectedCollect: expectedCounterMetrics,
		},
		"counter metric with tenant step injected label": {
			promtailConfig:  counterConfig,
			line:            testMetricLogLine1,
			expectedCollect: expectedCounterMetrics,
		},
		"gauge metric with non-prometheus incoming label": {
			promtailConfig: gaugeConfig,
			labels: model.LabelSet{
				"__bad_label__": "2",
			},
			line:            `#<13>Jan 28 14:25:52 vehicle=1 longitude=-10.1234 latitude=15.1234`,
			expectedCollect: expectedGaugeMetrics,
		},
		"histogram metric with non-prometheus incoming label": {
			promtailConfig: histogramConfig,
			labels: model.LabelSet{
				"test":          "app",
				"__bad_label__": "2",
			},
			line:            testMetricLogLine1,
			expectedCollect: expectedHistogramMetrics,
		},
	} {
		t.Run(name, func(t *testing.T) {
			registry := prometheus.NewRegistry()
			pl, err := NewPipeline(log.NewNopLogger(), loadConfig(tc.promtailConfig), nil, registry, featuregate.StabilityGenerallyAvailable)
			require.NoError(t, err)
			in := make(chan Entry)
			out := pl.Run(in)

			in <- newEntry(nil, tc.labels, tc.line, time.Now())
			close(in)
			<-out

			err = testutil.GatherAndCompare(registry, strings.NewReader(tc.expectedCollect))
			require.NoError(t, err, "gathered metrics are different than expected")
		})
	}
}

var (
	labelFoo = model.LabelSet(map[model.LabelName]model.LabelValue{"foo": "bar", "bar": "foo"})
	labelFu  = model.LabelSet(map[model.LabelName]model.LabelValue{"fu": "baz", "baz": "fu"})
)

func TestMetricStage_Process(t *testing.T) {
	jsonStageConfig := StageConfig{JSONConfig: &JSONConfig{
		Expressions: map[string]string{
			"total_keys":      "length(keys(@))",
			"keys_per_line":   "length(keys(@))",
			"numeric_float":   "numeric.float",
			"numeric_integer": "numeric.integer",
			"numeric_string":  "numeric.string",
			"contains_warn":   "contains(values(@),'WARN')",
			"contains_false":  "contains(keys(@),'nope')",
		},
	}}
	regexHTTPFixture := `11.11.11.11 - frank [25/Jan/2000:14:00:01 -0500] "GET /1986.js HTTP/1.1" 200 932ms"`
	regexStageConfig := StageConfig{RegexConfig: &RegexConfig{
		Expression: "(?P<get>\"GET).*HTTP/1.1\" (?P<status>\\d*) (?P<time>\\d*ms)",
	}}
	timeSource := "time"
	trueVal := "true"
	metricsStageConfig := StageConfig{MetricsConfig: &MetricsConfig{
		Metrics: []MetricConfig{
			{
				Counter: &metric.CounterConfig{
					Name:        "total_keys",
					Description: "the total keys per doc",
					Source:      "total_keys",
					Action:      metric.CounterAdd,
				}},
			{
				Histogram: &metric.HistogramConfig{
					Name:        "keys_per_line",
					Description: "keys per doc",
					Source:      "keys_per_line",
					Buckets:     []float64{1, 3, 5, 10},
				}},
			{
				Gauge: &metric.GaugeConfig{
					Name:        "numeric_float",
					Description: "numeric_float",
					Source:      "numeric_float",
					Action:      metric.GaugeAdd,
				}},
			{
				Gauge: &metric.GaugeConfig{
					Name:        "numeric_integer",
					Description: "numeric.integer",
					Source:      "numeric_integer",
					Action:      metric.GaugeAdd,
				}},
			{
				Gauge: &metric.GaugeConfig{
					Name:        "numeric_string",
					Description: "numeric.string",
					Source:      "numeric_string",
					Action:      metric.GaugeAdd,
				}},
			{
				Counter: &metric.CounterConfig{
					Name:        "contains_warn",
					Description: "contains_warn",
					Source:      "contains_warn",
					Value:       trueVal,
					Action:      metric.CounterInc,
				}},
			{
				Counter: &metric.CounterConfig{
					Name:        "matches",
					Source:      timeSource,
					Description: "all matches",
					Action:      metric.CounterInc,
				}},
			{
				Histogram: &metric.HistogramConfig{
					Name:        "response_time_seconds",
					Source:      timeSource,
					Description: "response time in ms",
					Buckets:     []float64{0.5, 1, 2},
				}},
		}}}

	registry := prometheus.NewRegistry()
	jsonStage, err := New(log.NewNopLogger(), nil, jsonStageConfig, registry, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatalf("failed to create stage with metrics: %v", err)
	}
	regexStage, err := New(log.NewNopLogger(), nil, regexStageConfig, registry, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatalf("failed to create stage with metrics: %v", err)
	}
	metricStage, err := New(log.NewNopLogger(), nil, metricsStageConfig, registry, featuregate.StabilityGenerallyAvailable)
	if err != nil {
		t.Fatalf("failed to create stage with metrics: %v", err)
	}
	out := processEntries(jsonStage, newEntry(nil, labelFoo, logFixture, time.Now()))
	out[0].Line = regexHTTPFixture
	out = processEntries(regexStage, out...)
	out = processEntries(metricStage, out...)
	out[0].Labels = labelFu
	// Process the same extracted values again with different labels so we can verify proper metric/label assignments
	_ = processEntries(metricStage, out...)
	names := metricNames(metricsStageConfig)
	if err := testutil.GatherAndCompare(registry,
		strings.NewReader(goldenMetrics), names...); err != nil {
		t.Fatalf("mismatch metrics: %v", err)
	}
}

func metricNames(sc StageConfig) []string {
	cfg := sc.MetricsConfig
	result := make([]string, 0, len(cfg.Metrics))
	for _, config := range cfg.Metrics {
		switch {
		case config.Counter != nil:
			customPrefix := ""
			if config.Counter.Prefix != "" {
				customPrefix = config.Counter.Prefix
			} else {
				customPrefix = defaultMetricsPrefix
			}
			result = append(result, customPrefix+config.Counter.Name)
		case config.Gauge != nil:
			customPrefix := ""
			if config.Gauge.Prefix != "" {
				customPrefix = config.Gauge.Prefix
			} else {
				customPrefix = defaultMetricsPrefix
			}
			result = append(result, customPrefix+config.Gauge.Name)
		case config.Histogram != nil:
			customPrefix := ""
			if config.Histogram.Prefix != "" {
				customPrefix = config.Histogram.Prefix
			} else {
				customPrefix = defaultMetricsPrefix
			}
			result = append(result, customPrefix+config.Histogram.Name)
		}
	}
	return result
}

const goldenMetrics = `# HELP loki_process_custom_contains_warn contains_warn
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
`
