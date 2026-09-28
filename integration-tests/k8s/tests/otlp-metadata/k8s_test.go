package otlpmetadata

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

func TestOtlpMetadata(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-otlp-metadata",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	promGen := deps.NewPromGen(deps.PromGenOptions{Namespace: ns.Name()})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-otlp-metadata",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, promGen, mimir, alloy},
	})

	floatMetrics := []string{
		"otlp_metadata_golang_counter",
		"otlp_metadata_golang_gauge",
		"otlp_metadata_golang_histogram_bucket",
		"otlp_metadata_golang_histogram_count",
		"otlp_metadata_golang_histogram_sum",
		"otlp_metadata_golang_mixed_histogram_bucket",
		"otlp_metadata_golang_mixed_histogram_count",
		"otlp_metadata_golang_mixed_histogram_sum",
		"otlp_metadata_golang_summary",
	}
	histogramMetrics := []string{
		"otlp_metadata_golang_native_histogram",
		"otlp_metadata_golang_mixed_histogram",
	}

	mimir.QueryMetrics(t, "otlp-metadata", append(append([]string{}, floatMetrics...), histogramMetrics...))
	mimir.QueryPositive(t, "otlp-metadata", floatMetrics)
	mimir.QueryHistograms(t, "otlp-metadata", histogramMetrics, func(c *assert.CollectT, sample deps.HistogramSample) {
		assert.Greaterf(c, sample.Count, 10.0, "%s: count", sample.Name())
		assert.Greaterf(c, sample.Sum, 10.0, "%s: sum", sample.Name())
		assert.NotEmptyf(c, sample.Buckets, "%s: buckets", sample.Name())
	})

	mimir.QueryMetadata(t, map[string]deps.ExpectedMetadata{
		"otlp_metadata_golang_counter":          {Type: "counter", Help: "The counter description string"},
		"otlp_metadata_golang_gauge":            {Type: "gauge", Help: "The gauge description string"},
		"otlp_metadata_golang_histogram":        {Type: "histogram", Help: "The histogram description string"},
		"otlp_metadata_golang_mixed_histogram":  {Type: "histogram", Help: "The mixed_histogram description string"},
		"otlp_metadata_golang_summary":          {Type: "summary", Help: "The summary description string"},
		"otlp_metadata_golang_native_histogram": {Type: "histogram", Help: "The native_histogram description string"},
	})
}
