package otelcoldefaultengine

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const release = "alloy-test-otelcol-default-engine"

// TestOtelcolDefaultEngine runs otelcol components in Alloy's default engine.
// otel-gen sends metrics and traces to them, and the metrics must reach Mimir
// both over OTLP and converted to Prometheus. Alloy's own otelcol metrics must
// be reported too.
func TestOtelcolDefaultEngine(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-otelcol-default-engine",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    release,
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	otelGen := deps.NewOtelGen(deps.OtelGenOptions{
		Namespace: ns.Name(),
		Endpoint:  release + ":4318",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, mimir, alloy, otelGen},
	})

	t.Run("OTLP", func(t *testing.T) {
		deps.AssertOtelGenMetrics(t, mimir, "otlp")
	})

	t.Run("OTLPToPrometheus", func(t *testing.T) {
		// The conversion adds _total to counters. The other names, and the
		// kinds of sample, are the same as over OTLP.
		positiveMetrics := []string{
			"example_counter_total",
			"example_float_counter_total",
			"example_float_updowncounter",
			"example_histogram_bucket",
			"example_float_histogram_bucket",
		}
		histogramMetrics := []string{
			"example_exponential_histogram",
			"example_exponential_float_histogram",
		}
		mimir.QueryMetrics(t, "otlp-to-prom", append(append([]string{"example_updowncounter"}, positiveMetrics...), histogramMetrics...))
		mimir.QueryPositive(t, "otlp-to-prom", positiveMetrics)
		mimir.QueryHistograms(t, "otlp-to-prom", histogramMetrics, func(c *assert.CollectT, sample deps.HistogramSample) {
			assert.Greaterf(c, sample.Count, 10.0, "%s: count", sample.Name())
			assert.Greaterf(c, sample.Sum, 10.0, "%s: sum", sample.Name())
			assert.NotEmptyf(c, sample.Buckets, "%s: buckets", sample.Name())
		})
	})

	t.Run("AlloySelfMetrics", func(t *testing.T) {
		// The k8s-monitoring chart's allow list relies on these:
		// https://github.com/grafana/k8s-monitoring-helm/blob/main/charts/k8s-monitoring-v1/default_allow_lists/alloy_integration.yaml
		mimir.QueryMetrics(t, "alloy-self", []string{
			"otelcol_exporter_send_failed_spans_total",
			"otelcol_exporter_sent_spans_total",
			"otelcol_processor_batch_batch_send_size_bucket",
			"otelcol_processor_batch_metadata_cardinality",
			"otelcol_processor_batch_timeout_trigger_send_total",
			"otelcol_receiver_accepted_spans_total",
			"otelcol_receiver_refused_spans_total",
		})
		// Spans go to one exporter that succeeds and one that fails.
		mimir.QueryPositive(t, "alloy-self", []string{
			"otelcol_exporter_sent_spans_total",
			"otelcol_exporter_send_failed_spans_total",
		})
	})
}
