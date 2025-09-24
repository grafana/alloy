package prometheus_test

import (
	"context"
	"strconv"
	"testing"
	"time"

	alloyprometheus "github.com/grafana/alloy/internal/component/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/scrape"
	"github.com/prometheus/prometheus/tsdb/tsdbutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/fakeconsumer"
	"github.com/grafana/alloy/internal/component/otelcol/receiver/prometheus"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

// testMetadataStore implements scrape.MetricMetadataStore for testing
type testMetadataStore map[string]scrape.MetricMetadata

func (tmc testMetadataStore) GetMetadata(familyName string) (scrape.MetricMetadata, bool) {
	lookup, ok := tmc[familyName]
	return lookup, ok
}

func (tmc testMetadataStore) ListMetadata() []scrape.MetricMetadata { return nil }

func (tmc testMetadataStore) SizeMetadata() int { return 0 }

func (tmc testMetadataStore) LengthMetadata() int {
	return len(tmc)
}

// TestComprehensive performs a comprehensive integration test which runs the
// otelcol.receiver.prometheus component and ensures that it can receive and
// forward different types of metrics: native histograms, classic histograms,
// gauges, and sum/counter metrics, verifying each gets converted to the
// appropriate OTLP metric type.
func TestComprehensive(t *testing.T) {
	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.prometheus")
	require.NoError(t, err)

	cfg := `
		output {
			// no-op: will be overridden by test code.
		}
	`
	var args prometheus.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	// Override our settings so metrics get forwarded to metricCh.
	metricCh := make(chan pmetric.Metrics)
	args.Output = makeMetricsOutput(metricCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))
	require.NoError(t, ctrl.WaitExports(time.Second))

	exports := ctrl.Exports().(prometheus.Exports)

	// Use the exported Appendable to send different types of metrics to the receiver.
	go func() {
		ts := time.Now().Unix()

		ctx := t.Context()
		ctx = scrape.ContextWithMetricMetadataStore(ctx, testMetadataStore{
			"testGauge": scrape.MetricMetadata{
				MetricFamily: "testGauge",
				Type:         model.MetricTypeGauge,
				Help:         "A test gauge metric",
			},
			"testCounter": scrape.MetricMetadata{
				MetricFamily: "testCounter",
				Type:         model.MetricTypeCounter,
				Help:         "A test counter metric",
			},
			"testClassicHistogram": scrape.MetricMetadata{
				MetricFamily: "testClassicHistogram",
				Type:         model.MetricTypeHistogram,
				Help:         "A test classic histogram metric",
			},
			"testNativeHistogram": scrape.MetricMetadata{
				MetricFamily: "testNativeHistogram",
				Type:         model.MetricTypeHistogram,
				Help:         "A test native histogram metric",
			},
		})
		ctx = scrape.ContextWithTarget(ctx, scrape.NewTarget(
			labels.EmptyLabels(),
			&config.DefaultScrapeConfig,
			model.LabelSet{},
			model.LabelSet{},
		))
		app := exports.Receiver.Appender(ctx)

		// 1. Send a gauge metric
		gaugeLabels := labels.New(
			labels.Label{Name: model.MetricNameLabel, Value: "testGauge"},
			labels.Label{Name: model.JobLabel, Value: "testJob"},
			labels.Label{Name: model.InstanceLabel, Value: "otelcol.receiver.prometheus"},
			labels.Label{Name: "foo", Value: "bar"},
			labels.Label{Name: "otel_scope_name", Value: "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp"},
			labels.Label{Name: "otel_scope_version", Value: "v0.24.0"},
		)
		_, err := app.Append(0, gaugeLabels, ts, 100.0)
		require.NoError(t, err)

		exemplarLabels := labels.New(
			labels.Label{Name: model.MetricNameLabel, Value: "testGauge"},
			labels.Label{Name: "trace_id", Value: "123456789abcdef0123456789abcdef0"},
			labels.Label{Name: "span_id", Value: "123456789abcdef0"},
		)
		exemplar := exemplar.Exemplar{
			Value:  2,
			Ts:     ts,
			HasTs:  true,
			Labels: exemplarLabels,
		}
		_, err = app.AppendExemplar(0, gaugeLabels, exemplar)
		require.NoError(t, err)

		// 2. Send a counter/sum metric (using _total suffix to indicate it's a counter)
		counterLabels := labels.New(
			labels.Label{Name: model.MetricNameLabel, Value: "testCounter_total"},
			labels.Label{Name: model.JobLabel, Value: "testJob"},
			labels.Label{Name: model.InstanceLabel, Value: "otelcol.receiver.prometheus"},
			labels.Label{Name: "service", Value: "api"},
			labels.Label{Name: "otel_scope_name", Value: "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp"},
			labels.Label{Name: "otel_scope_version", Value: "v0.24.0"},
		)
		_, err = app.Append(0, counterLabels, ts, 42.0)
		require.NoError(t, err)

		// 3. Send a classic/traditional histogram (bucket, count, sum)
		histogramName := "testClassicHistogram"

		// Histogram buckets
		buckets := []float64{0.1, 0.5, 1.0, 5.0, 10.0}
		counts := []float64{1, 3, 5, 8, 10} // cumulative counts

		for i, bucket := range buckets {
			bucketLabels := labels.New(
				labels.Label{Name: model.MetricNameLabel, Value: histogramName + "_bucket"},
				labels.Label{Name: model.JobLabel, Value: "testJob"},
				labels.Label{Name: model.InstanceLabel, Value: "otelcol.receiver.prometheus"},
				labels.Label{Name: "le", Value: strconv.FormatFloat(bucket, 'f', -1, 64)},
				labels.Label{Name: "method", Value: "GET"},
				labels.Label{Name: "otel_scope_name", Value: "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp"},
				labels.Label{Name: "otel_scope_version", Value: "v0.24.0"},
			)
			_, err = app.Append(0, bucketLabels, ts, counts[i])
			require.NoError(t, err)
		}

		// Histogram +Inf bucket
		infBucketLabels := labels.New(
			labels.Label{Name: model.MetricNameLabel, Value: histogramName + "_bucket"},
			labels.Label{Name: model.JobLabel, Value: "testJob"},
			labels.Label{Name: model.InstanceLabel, Value: "otelcol.receiver.prometheus"},
			labels.Label{Name: "le", Value: "+Inf"},
			labels.Label{Name: "method", Value: "GET"},
			labels.Label{Name: "otel_scope_name", Value: "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp"},
			labels.Label{Name: "otel_scope_version", Value: "v0.24.0"},
		)
		_, err = app.Append(0, infBucketLabels, ts, 10.0)
		require.NoError(t, err)

		// Histogram count
		countLabels := labels.New(
			labels.Label{Name: model.MetricNameLabel, Value: histogramName + "_count"},
			labels.Label{Name: model.JobLabel, Value: "testJob"},
			labels.Label{Name: model.InstanceLabel, Value: "otelcol.receiver.prometheus"},
			labels.Label{Name: "method", Value: "GET"},
			labels.Label{Name: "otel_scope_name", Value: "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp"},
			labels.Label{Name: "otel_scope_version", Value: "v0.24.0"},
		)
		_, err = app.Append(0, countLabels, ts, 10.0)
		require.NoError(t, err)

		// Histogram sum
		sumLabels := labels.New(
			labels.Label{Name: model.MetricNameLabel, Value: histogramName + "_sum"},
			labels.Label{Name: model.JobLabel, Value: "testJob"},
			labels.Label{Name: model.InstanceLabel, Value: "otelcol.receiver.prometheus"},
			labels.Label{Name: "method", Value: "GET"},
			labels.Label{Name: "otel_scope_name", Value: "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp"},
			labels.Label{Name: "otel_scope_version", Value: "v0.24.0"},
		)
		_, err = app.Append(0, sumLabels, ts, 23.5)
		require.NoError(t, err)

		// 4. Send a native exponential histogram
		nativeHistLabels := labels.New(
			labels.Label{Name: model.MetricNameLabel, Value: "testNativeHistogram"},
			labels.Label{Name: model.JobLabel, Value: "testJob"},
			labels.Label{Name: model.InstanceLabel, Value: "otelcol.receiver.prometheus"},
			labels.Label{Name: "endpoint", Value: "/api/v1"},
			labels.Label{Name: "otel_scope_name", Value: "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp"},
			labels.Label{Name: "otel_scope_version", Value: "v0.24.0"},
		)
		h := tsdbutil.GenerateTestHistogram(42)
		_, err = app.AppendHistogram(0, nativeHistLabels, ts, h, nil)
		require.NoError(t, err)

		require.NoError(t, app.Commit())
	}()

	// Wait for our client to get the metrics.
	select {
	case <-time.After(5 * time.Second):
		require.FailNow(t, "failed waiting for metrics")
	case m := <-metricCh:
		// Should have 4 metrics: gauge, counter, classic histogram, native histogram
		require.Equal(t, 4, m.MetricCount())

		require.Equal(t, "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp", m.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Name())
		require.Equal(t, "v0.24.0", m.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Version())

		metrics := make(map[string]pmetric.Metric)
		for i := 0; i < m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len(); i++ {
			metric := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(i)
			metrics[metric.Name()] = metric
		}

		// 1. Verify gauge metric
		gaugeMetric, exists := metrics["testGauge"]
		require.True(t, exists, "testGauge metric should exist")
		require.Equal(t, pmetric.MetricTypeGauge, gaugeMetric.Type())
		require.Equal(t, "Gauge", gaugeMetric.Type().String())
		require.Equal(t, 1, gaugeMetric.Gauge().DataPoints().Len())
		require.Equal(t, 100.0, gaugeMetric.Gauge().DataPoints().At(0).DoubleValue())
		require.Equal(t, 1, gaugeMetric.Gauge().DataPoints().At(0).Exemplars().Len())
		require.Equal(t, "A test gauge metric", gaugeMetric.Description())
		require.Equal(t, 1, gaugeMetric.Gauge().DataPoints().At(0).Exemplars().Len())
		require.Equal(t, "123456789abcdef0123456789abcdef0", gaugeMetric.Gauge().DataPoints().At(0).Exemplars().At(0).TraceID().String())
		require.Equal(t, "123456789abcdef0", gaugeMetric.Gauge().DataPoints().At(0).Exemplars().At(0).SpanID().String())
		require.Equal(t, 2.0, gaugeMetric.Gauge().DataPoints().At(0).Exemplars().At(0).DoubleValue())

		// 2. Verify counter/sum metric
		counterMetric, exists := metrics["testCounter_total"]
		require.True(t, exists, "testCounter_total metric should exist")
		require.Equal(t, pmetric.MetricTypeSum, counterMetric.Type()) // NoopMetadataStore makes it gauge
		require.Equal(t, "Sum", counterMetric.Type().String())
		require.Equal(t, 1, counterMetric.Sum().DataPoints().Len())
		require.Equal(t, 42.0, counterMetric.Sum().DataPoints().At(0).DoubleValue())
		require.Equal(t, "A test counter metric", counterMetric.Description())

		// 3. Verify classic histogram
		classicHistMetric, exists := metrics["testClassicHistogram"]
		require.True(t, exists, "testClassicHistogram metric should exist")
		require.Equal(t, pmetric.MetricTypeHistogram, classicHistMetric.Type()) // NoopMetadataStore makes it gauge
		require.Equal(t, "Histogram", classicHistMetric.Type().String())
		require.Equal(t, 1, classicHistMetric.Histogram().DataPoints().Len())
		require.Equal(t, "A test classic histogram metric", classicHistMetric.Description())

		// 4. Verify native exponential histogram
		nativeHistMetric, exists := metrics["testNativeHistogram"]
		require.True(t, exists, "testNativeHistogram metric should exist")
		require.Equal(t, pmetric.MetricTypeExponentialHistogram, nativeHistMetric.Type())
		require.Equal(t, "ExponentialHistogram", nativeHistMetric.Type().String())
		require.Equal(t, 1, nativeHistMetric.ExponentialHistogram().DataPoints().Len())
		require.Equal(t, "A test native histogram metric", nativeHistMetric.Description())

		expHistDP := nativeHistMetric.ExponentialHistogram().DataPoints().At(0)
		require.Greater(t, expHistDP.Count(), uint64(0))
		require.True(t, expHistDP.HasSum())
		require.NotEqual(t, int32(0), expHistDP.Scale()) // Should have a valid scale
	}
}

func TestHistogram(t *testing.T) {
	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.prometheus")
	require.NoError(t, err)

	cfg := `
		output {
			// no-op: will be overridden by test code.
		}
	`
	var args prometheus.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	// Override our settings so metrics get forwarded to metricCh.
	metricCh := make(chan pmetric.Metrics)
	args.Output = makeMetricsOutput(metricCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))
	require.NoError(t, ctrl.WaitExports(time.Second))

	exports := ctrl.Exports().(prometheus.Exports)

	// Use the exported Appendable to send histogram metrics to the receiver in the
	// background.
	go func() {
		l := labels.New(
			labels.Label{Name: model.MetricNameLabel, Value: "testHistogram"},
			labels.Label{Name: model.JobLabel, Value: "testJob"},
			labels.Label{Name: model.InstanceLabel, Value: "otelcol.receiver.prometheus"},
			labels.Label{Name: "foo", Value: "bar"},
		)
		ts := time.Now().Unix()

		// Create a native histogram using the test utility
		hist := tsdbutil.GenerateTestHistogram(1)
		hist.CounterResetHint = histogram.NotCounterReset
		fh := tsdbutil.GenerateTestFloatHistogram(1)
		fh.CounterResetHint = histogram.NotCounterReset

		ctx := t.Context()
		ctx = scrape.ContextWithMetricMetadataStore(ctx, alloyprometheus.NoopMetadataStore{})
		ctx = scrape.ContextWithTarget(ctx, scrape.NewTarget(
			labels.EmptyLabels(),
			&config.DefaultScrapeConfig,
			model.LabelSet{},
			model.LabelSet{},
		))
		app := exports.Receiver.Appender(ctx)
		_, err := app.AppendHistogram(0, l, ts, hist, fh)
		require.NoError(t, err)
		require.NoError(t, app.Commit())
	}()

	// Wait for our client to get the histogram metric.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for histogram metrics")
	case m := <-metricCh:
		require.Equal(t, 1, m.MetricCount())
		require.Equal(t, "testHistogram", m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Name())

		metricType := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Type().String()
		if assert.Equal(t, "ExponentialHistogram", metricType) {
			hist := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).ExponentialHistogram()
			require.Equal(t, 1, hist.DataPoints().Len())

			dp := hist.DataPoints().At(0)
			require.Equal(t, uint64(21), dp.Count())
			require.Equal(t, 36.8, dp.Sum())
			require.Equal(t, uint64(3), dp.ZeroCount())
		} else {
			// If it's not an exponential histogram, print some info for debugging.
			metric := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)
			t.Logf("Metric name: %s", metric.Name())
			t.Logf("Metric type: %s", metric.Type().String())
			t.Fail()
		}
	}
}

// TestDuplicateLabelNamesError verifies that metrics with duplicate label names
// are properly rejected with the expected error message. This essentially verifies that
// labels.New() will ensure that labels are sorted
func TestDuplicateLabelNamesError(t *testing.T) {
	ctx := componenttest.TestContext(t)
	l := util.TestLogger(t)

	ctrl, err := componenttest.NewControllerFromID(l, "otelcol.receiver.prometheus")
	require.NoError(t, err)

	cfg := `
		output {
			// no-op: will be overridden by test code.
		}
	`
	var args prometheus.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))

	// Override our settings so metrics get forwarded to metricCh.
	metricCh := make(chan pmetric.Metrics)
	args.Output = makeMetricsOutput(metricCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))
	require.NoError(t, ctrl.WaitExports(time.Second))

	exports := ctrl.Exports().(prometheus.Exports)

	lbls := labels.New(
		labels.Label{Name: model.MetricNameLabel, Value: "test_metric"},
		labels.Label{Name: model.JobLabel, Value: "testJob"},
		labels.Label{Name: model.MetricNameLabel, Value: "duplicate_name"}, // Duplicate __name__
		labels.Label{Name: "instance", Value: "localhost:8080"})

	ctx = context.Background()
	ctx = scrape.ContextWithMetricMetadataStore(ctx, alloyprometheus.NoopMetadataStore{})
	ctx = scrape.ContextWithTarget(ctx, scrape.NewTarget(
		labels.EmptyLabels(),
		&config.DefaultScrapeConfig,
		model.LabelSet{},
		model.LabelSet{},
	))
	app := exports.Receiver.Appender(ctx)

	ts := time.Now().Unix()

	_, err = app.Append(0, lbls, ts, 42.0)

	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid sample: non-unique label names:")
}

// makeMetricsOutput returns a ConsumerArguments which will forward metrics to
// the provided channel.
func makeMetricsOutput(ch chan pmetric.Metrics) *otelcol.ConsumerArguments {
	metricsConsumer := fakeconsumer.Consumer{
		ConsumeMetricsFunc: func(ctx context.Context, m pmetric.Metrics) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- m:
				return nil
			}
		},
	}

	return &otelcol.ConsumerArguments{
		Metrics: []otelcol.Consumer{&metricsConsumer},
	}
}
