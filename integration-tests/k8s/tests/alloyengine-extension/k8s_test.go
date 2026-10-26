package alloyengineextension

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const testName = "alloyengine-extension"

// TestAlloyengineExtension runs Alloy with the OTel engine (`alloy otel`),
// whose alloyengine extension runs an OTLP pipeline from config.alloy. otel-gen
// sends every metric type to it, and each must reach Mimir as the right kind of
// sample.
func TestAlloyengineExtension(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-alloyengine-extension",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	alloyOtel := deps.NewAlloyOtel(deps.AlloyOtelOptions{
		Namespace:           ns.Name(),
		CollectorConfigPath: "./config/collector.yaml",
		AlloyConfigPath:     "./config/config.alloy",
	})
	otelGen := deps.NewOtelGen(deps.OtelGenOptions{
		Namespace: ns.Name(),
		Endpoint:  "alloy-otel:4318",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, mimir, alloyOtel, otelGen},
	})

	floatMetrics := []string{
		"example_counter",
		"example_float_counter",
		"example_float_updowncounter",
		"example_histogram_bucket",
		"example_float_histogram_bucket",
	}
	// otel-gen only ever subtracts from this one, so its samples are negative
	// and it can't go through QueryPositive.
	negativeMetrics := []string{"example_updowncounter"}
	histogramMetrics := []string{
		"example_exponential_histogram",
		"example_exponential_float_histogram",
	}

	mimir.QueryMetrics(t, testName, append(append(append([]string{}, floatMetrics...), negativeMetrics...), histogramMetrics...))
	mimir.QueryPositive(t, testName, floatMetrics)
	mimir.QueryHistograms(t, testName, histogramMetrics, func(c *assert.CollectT, sample deps.HistogramSample) {
		assert.Greaterf(c, sample.Count, 10.0, "%s: count", sample.Name())
		assert.Greaterf(c, sample.Sum, 10.0, "%s: sum", sample.Name())
		assert.NotEmptyf(c, sample.Buckets, "%s: buckets", sample.Name())
	})
}
