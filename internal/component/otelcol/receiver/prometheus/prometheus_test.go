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
// mixed histograms (both classic and native), gauges, and sum/counter metrics,
// verifying each gets converted to the appropriate OTLP metric type.
func TestComprehensive(t *testing.T) {
	t.Run("with metadata", func(t *testing.T) {
		testComprehensive(t, testMetadataStore{
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
			"testMixedHistogram": scrape.MetricMetadata{
				MetricFamily: "testMixedHistogram",
				Type:         model.MetricTypeHistogram,
				Help:         "A test mixed histogram metric with both classic and native buckets",
			},
		})
	})
	t.Run("without metadata", func(t *testing.T) {
		testComprehensive(t, testMetadataStore{})
	})
}

func testComprehensive(t *testing.T, metadataStore testMetadataStore) {
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
		ctx = scrape.ContextWithMetricMetadataStore(ctx, metadataStore)
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

		// 5. Send a mixed histogram (both classic buckets and native histogram data)
		mixedHistogramName := "testMixedHistogram"

		// First, send classic histogram buckets
		// Histogram buckets use canonical numbers
		// https://prometheus.io/docs/specs/om/open_metrics_spec/#considerations-canonical-numbers
		mixedBuckets := []string{"0.25", "2.5", "25.0", "+Inf"}
		mixedCounts := []float64{5, 15, 25, 30} // cumulative counts

		for i, bucket := range mixedBuckets {
			bucketLabels := labels.FromStrings(
				model.MetricNameLabel, mixedHistogramName+"_bucket",
				model.JobLabel, "testJob",
				model.InstanceLabel, "otelcol.receiver.prometheus",
				model.BucketLabel, bucket,
				"region", "us-west",
				"otel_scope_name", "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp",
				"otel_scope_version", "v0.24.0",
			)
			_, err = app.Append(0, bucketLabels, ts, mixedCounts[i])
			require.NoError(t, err)
		}

		// Mixed histogram count
		mixedCountLabels := labels.FromStrings(
			model.MetricNameLabel, mixedHistogramName+"_count",
			model.JobLabel, "testJob",
			model.InstanceLabel, "otelcol.receiver.prometheus",
			"region", "us-west",
			"otel_scope_name", "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp",
			"otel_scope_version", "v0.24.0",
		)
		_, err = app.Append(0, mixedCountLabels, ts, 30.0)
		require.NoError(t, err)

		// Mixed histogram sum
		mixedSumLabels := labels.FromStrings(
			model.MetricNameLabel, mixedHistogramName+"_sum",
			model.JobLabel, "testJob",
			model.InstanceLabel, "otelcol.receiver.prometheus",
			"region", "us-west",
			"otel_scope_name", "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp",
			"otel_scope_version", "v0.24.0",
		)
		_, err = app.Append(0, mixedSumLabels, ts, 125.5)
		require.NoError(t, err)

		// Then, send native exponential histogram data for the same metric
		mixedNativeHistLabels := labels.FromStrings(
			model.MetricNameLabel, mixedHistogramName,
			model.JobLabel, "testJob",
			model.InstanceLabel, "otelcol.receiver.prometheus",
			"region", "us-west",
			"otel_scope_name", "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp",
			"otel_scope_version", "v0.24.0",
		)

		// Create a native histogram with the same count and sum as the classic histogram
		mixedNativeHist := tsdbutil.GenerateTestHistogram(123)
		mixedNativeHist.Count = 30
		mixedNativeHist.Sum = 125.5
		mixedNativeHist.ZeroCount = 1
		mixedNativeHist.Schema = 2

		_, err = app.AppendHistogram(0, mixedNativeHistLabels, ts, mixedNativeHist, nil)
		require.NoError(t, err)

		require.NoError(t, app.Commit())
	}()

	// Wait for our client to get the metrics.
	select {
	case <-time.After(5 * time.Second):
		require.FailNow(t, "failed waiting for metrics")
	case m := <-metricCh:
		// Should have 6 metrics: gauge, counter, classic histogram, native histogram, and mixed histogram (2 representations)
		// require.Equal(t, 6, m.MetricCount())

		require.Equal(t, "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp", m.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Name())
		require.Equal(t, "v0.24.0", m.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Version())

		type resultKey struct {
			name string
			typ  pmetric.MetricType
		}

		metrics := make(map[resultKey]pmetric.Metric)
		for i := 0; i < m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().Len(); i++ {
			metric := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(i)
			metrics[resultKey{name: metric.Name(), typ: metric.Type()}] = metric
		}

		// 1. Verify gauge metric
		gaugeMetric, exists := metrics[resultKey{name: "testGauge", typ: pmetric.MetricTypeGauge}]
		require.True(t, exists, "testGauge metric should exist")
		require.Equal(t, 1, gaugeMetric.Gauge().DataPoints().Len())
		require.Equal(t, 100.0, gaugeMetric.Gauge().DataPoints().At(0).DoubleValue())
		require.Equal(t, 1, gaugeMetric.Gauge().DataPoints().At(0).Exemplars().Len())
		if _, ok := metadataStore.GetMetadata("testGauge"); ok {
			require.Equal(t, "A test gauge metric", gaugeMetric.Description())
		}
		require.Equal(t, 1, gaugeMetric.Gauge().DataPoints().At(0).Exemplars().Len())
		require.Equal(t, "123456789abcdef0123456789abcdef0", gaugeMetric.Gauge().DataPoints().At(0).Exemplars().At(0).TraceID().String())
		require.Equal(t, "123456789abcdef0", gaugeMetric.Gauge().DataPoints().At(0).Exemplars().At(0).SpanID().String())
		require.Equal(t, 2.0, gaugeMetric.Gauge().DataPoints().At(0).Exemplars().At(0).DoubleValue())

		// 2. Verify counter/sum metric
		if _, ok := metadataStore.GetMetadata("testCounter"); ok {
			counterMetric, exists := metrics[resultKey{name: "testCounter_total", typ: pmetric.MetricTypeSum}]
			require.True(t, exists, "testCounter_total metric should exist")
			require.Equal(t, "Sum", counterMetric.Type().String())
			require.Equal(t, 1, counterMetric.Sum().DataPoints().Len())
			require.Equal(t, 42.0, counterMetric.Sum().DataPoints().At(0).DoubleValue())
			require.Equal(t, "A test counter metric", counterMetric.Description())
		} else {
			counterMetric, exists := metrics[resultKey{name: "testCounter_total", typ: pmetric.MetricTypeGauge}]
			require.True(t, exists, "testCounter_total metric should exist")
			require.Equal(t, 1, counterMetric.Gauge().DataPoints().Len())
			require.Equal(t, 42.0, counterMetric.Gauge().DataPoints().At(0).DoubleValue())
		}

		// 3. Verify classic histogram
		if _, ok := metadataStore.GetMetadata("testClassicHistogram"); ok {
			classicHistMetric, exists := metrics[resultKey{name: "testClassicHistogram", typ: pmetric.MetricTypeHistogram}]
			require.True(t, exists, "testClassicHistogram metric should exist")
			require.Equal(t, 1, classicHistMetric.Histogram().DataPoints().Len())
			require.Equal(t, "A test classic histogram metric", classicHistMetric.Description())
		} else {
			// Without metadata we cannot combine series into a histogram
			require.Contains(t, metrics, resultKey{name: "testClassicHistogram_bucket", typ: pmetric.MetricTypeGauge})
			require.Contains(t, metrics, resultKey{name: "testClassicHistogram_count", typ: pmetric.MetricTypeGauge})
			require.Contains(t, metrics, resultKey{name: "testClassicHistogram_sum", typ: pmetric.MetricTypeGauge})
		}

		// 4. Verify native exponential histogram
		// Should work regardless of metadata.
		nativeHistMetric, exists := metrics[resultKey{name: "testNativeHistogram", typ: pmetric.MetricTypeExponentialHistogram}]
		require.True(t, exists, "testNativeHistogram metric should exist")
		require.Equal(t, 1, nativeHistMetric.ExponentialHistogram().DataPoints().Len())
		if _, ok := metadataStore.GetMetadata("testNativeHistogram"); ok {
			require.Equal(t, "A test native histogram metric", nativeHistMetric.Description())
		}
		expHistDP := nativeHistMetric.ExponentialHistogram().DataPoints().At(0)
		require.Greater(t, expHistDP.Count(), uint64(0))
		require.True(t, expHistDP.HasSum())
		require.NotEqual(t, int32(0), expHistDP.Scale()) // Should have a valid scale

		// 5. Verify mixed histogram - should have both classic and exponential representations
		// Group metrics by type to verify we have both representations of the mixed histogram

		// Verify mixed classic histogram properties
		if _, ok := metadataStore.GetMetadata("testMixedHistogram"); ok {
			mixedClassicHist, exists := metrics[resultKey{name: "testMixedHistogram", typ: pmetric.MetricTypeHistogram}]
			require.True(t, exists, "testMixedHistogram as explicit histogram should exist")
			require.Equal(t, 1, mixedClassicHist.Histogram().DataPoints().Len())
			require.Equal(t, "A test mixed histogram metric with both classic and native buckets", mixedClassicHist.Description())
		} else {
			// Without metadata we cannot combine series into a histogram
			require.Contains(t, metrics, resultKey{name: "testMixedHistogram_bucket", typ: pmetric.MetricTypeGauge})
			require.Contains(t, metrics, resultKey{name: "testMixedHistogram_count", typ: pmetric.MetricTypeGauge})
			require.Contains(t, metrics, resultKey{name: "testMixedHistogram_sum", typ: pmetric.MetricTypeGauge})
		}

		// Verify mixed exponential histogram properties
		mixedNativeHist, exists := metrics[resultKey{name: "testMixedHistogram", typ: pmetric.MetricTypeExponentialHistogram}]
		require.True(t, exists, "testMixedHistogram as exponential histogram should exist")
		require.Equal(t, 1, mixedNativeHist.ExponentialHistogram().DataPoints().Len())
		mixedNativeDP := mixedNativeHist.ExponentialHistogram().DataPoints().At(0)
		require.Equal(t, uint64(30), mixedNativeDP.Count())
		require.Equal(t, 125.5, mixedNativeDP.Sum())
		require.NotEqual(t, int32(0), mixedNativeDP.Scale(), "should have a valid scale")
		if _, ok := metadataStore.GetMetadata("testMixedHistogram"); ok {
			require.Equal(t, "A test mixed histogram metric with both classic and native buckets", mixedNativeHist.Description())
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

func TestNHCBDeltaBuckets(t *testing.T) {
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

	metricCh := make(chan pmetric.Metrics)
	args.Output = makeMetricsOutput(metricCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))
	require.NoError(t, ctrl.WaitExports(time.Second))

	exports := ctrl.Exports().(prometheus.Exports)

	go func() {
		ts := time.Now().Unix()

		ctx := t.Context()
		ctx = scrape.ContextWithMetricMetadataStore(ctx, testMetadataStore{
			"testNHCBDeltaHistogram": scrape.MetricMetadata{
				MetricFamily: "testNHCBDeltaHistogram",
				Type:         model.MetricTypeHistogram,
				Help:         "A test NHCB histogram with delta buckets",
			},
		})
		ctx = scrape.ContextWithTarget(ctx, scrape.NewTarget(
			labels.EmptyLabels(),
			&config.DefaultScrapeConfig,
			model.LabelSet{},
			model.LabelSet{},
		))
		app := exports.Receiver.Appender(ctx)

		nhcbHistLabels := labels.FromStrings(
			model.MetricNameLabel, "testNHCBDeltaHistogram",
			model.JobLabel, "testJob",
			model.InstanceLabel, "otelcol.receiver.prometheus",
			"endpoint", "/api/v1",
		)

		nhcbHist := &histogram.Histogram{
			Schema:          histogram.CustomBucketsSchema, // -53
			Count:           180,
			Sum:             100.5,
			CustomValues:    []float64{1.0, 2.0, 5.0, 10.0},
			PositiveSpans:   []histogram.Span{{Offset: 0, Length: 5}},
			PositiveBuckets: []int64{10, 15, 20, 5, 0}, // Delta encoded
		}
		nhcbHist.CounterResetHint = histogram.NotCounterReset

		_, err := app.AppendHistogram(0, nhcbHistLabels, ts, nhcbHist, nil)
		require.NoError(t, err)
		require.NoError(t, app.Commit())
	}()

	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for NHCB delta histogram metrics")
	case m := <-metricCh:
		require.Equal(t, 1, m.MetricCount())
		metric := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

		require.Equal(t, "testNHCBDeltaHistogram", metric.Name())
		require.Equal(t, "A test NHCB histogram with delta buckets", metric.Description())

		// NHCB should be converted to OTel Histogram (not ExponentialHistogram)
		require.Equal(t, pmetric.MetricTypeHistogram, metric.Type(),
			"NHCB should be converted to Histogram type, not ExponentialHistogram")

		hist := metric.Histogram()
		require.Equal(t, 1, hist.DataPoints().Len())

		dp := hist.DataPoints().At(0)

		// Verify count and sum
		require.Equal(t, uint64(180), dp.Count())
		require.True(t, dp.HasSum())
		require.Equal(t, 100.5, dp.Sum())

		// Verify explicit bounds from CustomValues
		bounds := dp.ExplicitBounds().AsRaw()
		require.Equal(t, []float64{1.0, 2.0, 5.0, 10.0}, bounds,
			"Explicit bounds should match CustomValues")

		// Verify bucket counts (converted from delta to absolute)
		// Delta: [10, 15, 20, 5, 0] -> Absolute: [10, 25, 45, 50, 50]
		bucketCounts := dp.BucketCounts().AsRaw()
		require.Equal(t, []uint64{10, 25, 45, 50, 50}, bucketCounts,
			"Bucket counts should be converted from delta to absolute values")
	}
}

func TestNHCBAbsoluteBuckets(t *testing.T) {
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

	metricCh := make(chan pmetric.Metrics)
	args.Output = makeMetricsOutput(metricCh)

	go func() {
		err := ctrl.Run(ctx, args)
		require.NoError(t, err)
	}()

	require.NoError(t, ctrl.WaitRunning(time.Second))
	require.NoError(t, ctrl.WaitExports(time.Second))

	exports := ctrl.Exports().(prometheus.Exports)

	go func() {
		ts := time.Now().Unix()

		ctx := t.Context()
		ctx = scrape.ContextWithMetricMetadataStore(ctx, testMetadataStore{
			"testNHCBAbsoluteHistogram": scrape.MetricMetadata{
				MetricFamily: "testNHCBAbsoluteHistogram",
				Type:         model.MetricTypeHistogram,
				Help:         "A test NHCB histogram with absolute buckets",
			},
		})
		ctx = scrape.ContextWithTarget(ctx, scrape.NewTarget(
			labels.EmptyLabels(),
			&config.DefaultScrapeConfig,
			model.LabelSet{},
			model.LabelSet{},
		))
		app := exports.Receiver.Appender(ctx)

		nhcbHistLabels := labels.FromStrings(
			model.MetricNameLabel, "testNHCBAbsoluteHistogram",
			model.JobLabel, "testJob",
			model.InstanceLabel, "otelcol.receiver.prometheus",
			"endpoint", "/api/v2",
		)

		nhcbFloatHist := &histogram.FloatHistogram{
			Schema:          histogram.CustomBucketsSchema, // -53
			Count:           50.0,
			Sum:             125.25,
			CustomValues:    []float64{0.5, 2.0},
			PositiveSpans:   []histogram.Span{{Offset: 0, Length: 3}},
			PositiveBuckets: []float64{15.0, 20.0, 15.0}, // Absolute values
		}
		nhcbFloatHist.CounterResetHint = histogram.NotCounterReset

		_, err := app.AppendHistogram(0, nhcbHistLabels, ts, nil, nhcbFloatHist)
		require.NoError(t, err)
		require.NoError(t, app.Commit())
	}()

	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for NHCB absolute histogram metrics")
	case m := <-metricCh:
		require.Equal(t, 1, m.MetricCount())
		metric := m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0)

		require.Equal(t, "testNHCBAbsoluteHistogram", metric.Name())
		require.Equal(t, "A test NHCB histogram with absolute buckets", metric.Description())

		// NHCB should be converted to OTel Histogram (not ExponentialHistogram)
		require.Equal(t, pmetric.MetricTypeHistogram, metric.Type(),
			"NHCB should be converted to Histogram type, not ExponentialHistogram")

		hist := metric.Histogram()
		require.Equal(t, 1, hist.DataPoints().Len())

		dp := hist.DataPoints().At(0)

		// Verify count and sum
		require.Equal(t, uint64(50), dp.Count())
		require.True(t, dp.HasSum())
		require.Equal(t, 125.25, dp.Sum())

		// Verify explicit bounds from CustomValues
		bounds := dp.ExplicitBounds().AsRaw()
		require.Equal(t, []float64{0.5, 2.0}, bounds,
			"Explicit bounds should match CustomValues")

		// Verify bucket counts (already absolute values, truncated to uint64)
		// Float buckets: [15.0, 20.0, 15.0] -> uint64: [15, 20, 15]
		bucketCounts := dp.BucketCounts().AsRaw()
		require.Equal(t, []uint64{15, 20, 15}, bucketCounts,
			"Bucket counts should match absolute float values (truncated to uint64)")
	}
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
