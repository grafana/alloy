package prometheus_test

import (
	"context"
	"testing"
	"time"

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
	alloyprometheus "github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
)

// Test performs a basic integration test which runs the
// otelcol.receiver.prometheus component and ensures that it can receive and
// forward metric data.
func Test(t *testing.T) {
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

	// Use the exported Appendable to send metrics to the receiver in the
	// background.
	go func() {
		l := labels.Labels{
			{Name: model.MetricNameLabel, Value: "testMetric"},
			{Name: model.JobLabel, Value: "testJob"},
			{Name: model.InstanceLabel, Value: "otelcol.receiver.prometheus"},
			{Name: "foo", Value: "bar"},
			{Name: model.MetricNameLabel, Value: "otel_scope_info"},
			{Name: "otel_scope_name", Value: "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp"},
			{Name: "otel_scope_version", Value: "v0.24.0"},
		}
		ts := time.Now().Unix()
		v := 100.

		exemplarLabels := labels.Labels{
			{Name: model.MetricNameLabel, Value: "testMetric"},
			{Name: "trace_id", Value: "123456789abcdef0123456789abcdef0"},
			{Name: "span_id", Value: "123456789abcdef0"},
		}
		exemplar := exemplar.Exemplar{
			Value:  2,
			Ts:     ts,
			HasTs:  true,
			Labels: exemplarLabels,
		}

		ctx := t.Context()
		ctx = scrape.ContextWithMetricMetadataStore(ctx, alloyprometheus.NoopMetadataStore{})
		ctx = scrape.ContextWithTarget(ctx, scrape.NewTarget(
			labels.EmptyLabels(),
			&config.DefaultScrapeConfig,
			model.LabelSet{},
			model.LabelSet{},
		))
		app := exports.Receiver.Appender(ctx)
		_, err := app.Append(0, l, ts, v)
		require.NoError(t, err)
		_, err = app.AppendExemplar(0, l, exemplar)
		require.NoError(t, err)
		require.NoError(t, app.Commit())
	}()

	// Wait for our client to get the metric.
	select {
	case <-time.After(time.Second):
		require.FailNow(t, "failed waiting for metrics")
	case m := <-metricCh:
		require.Equal(t, 1, m.MetricCount())
		require.Equal(t, "testMetric", m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Name())
		require.Equal(t, "go.opentelemetry.io.contrib.instrumentation.net.http.otelhttp", m.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Name())
		require.Equal(t, "v0.24.0", m.ResourceMetrics().At(0).ScopeMetrics().At(0).Scope().Version())
		require.Equal(t, "Gauge", m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Type().String())
		require.Equal(t, 1, m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().Len())
		require.Equal(t, 1, m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Exemplars().Len())
		require.Equal(t, "123456789abcdef0123456789abcdef0", m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Exemplars().At(0).TraceID().String())
		require.Equal(t, "123456789abcdef0", m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Exemplars().At(0).SpanID().String())
		require.Equal(t, 2.0, m.ResourceMetrics().At(0).ScopeMetrics().At(0).Metrics().At(0).Gauge().DataPoints().At(0).Exemplars().At(0).DoubleValue())
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
		l := labels.Labels{
			{Name: model.MetricNameLabel, Value: "testHistogram"},
			{Name: model.JobLabel, Value: "testJob"},
			{Name: model.InstanceLabel, Value: "otelcol.receiver.prometheus"},
			{Name: "foo", Value: "bar"},
		}
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
