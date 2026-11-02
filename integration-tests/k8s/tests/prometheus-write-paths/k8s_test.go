package prometheuswritepaths

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

// writePath is one way Alloy delivers scraped metrics to a Prometheus-compatible
// backend. Each is scraped under its own metric prefix so Mimir's metadata API,
// which has no label matcher, can be queried per path.
type writePath struct {
	testName string
	prefix   string
	// metadata is false for remote write v1, which carries metadata out of band
	// rather than alongside samples.
	metadata bool
}

func TestPrometheusWritePaths(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-prometheus-write-paths",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	promGen := deps.NewPromGen(deps.PromGenOptions{Namespace: ns.Name()})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-prometheus-write-paths",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, promGen, mimir, alloy},
	})

	for _, path := range []writePath{
		{testName: "prw-v1", prefix: "prw_v1_", metadata: false},
		{testName: "prw-v2", prefix: "prw_v2_", metadata: true},
		{testName: "otlp", prefix: "otlp_", metadata: true},
	} {
		t.Run(path.testName, func(t *testing.T) {
			floatMetrics := prefixed(path.prefix,
				"golang_counter",
				"golang_gauge",
				"golang_histogram_bucket",
				"golang_histogram_count",
				"golang_histogram_sum",
				"golang_mixed_histogram_bucket",
				"golang_mixed_histogram_count",
				"golang_mixed_histogram_sum",
				"golang_summary",
			)
			histogramMetrics := prefixed(path.prefix,
				"golang_native_histogram",
				"golang_mixed_histogram",
			)

			mimir.QueryMetrics(t, path.testName, append(append([]string{}, floatMetrics...), histogramMetrics...))
			mimir.QueryPositive(t, path.testName, floatMetrics)
			mimir.QueryHistograms(t, path.testName, histogramMetrics, func(c *assert.CollectT, sample deps.HistogramSample) {
				assert.Greaterf(c, sample.Count, 10.0, "%s: count", sample.Name())
				assert.Greaterf(c, sample.Sum, 10.0, "%s: sum", sample.Name())
				assert.NotEmptyf(c, sample.Buckets, "%s: buckets", sample.Name())
			})

			if !path.metadata {
				return
			}

			mimir.QueryMetadata(t, map[string]deps.ExpectedMetadata{
				path.prefix + "golang_counter":          {Type: "counter", Help: "The counter description string"},
				path.prefix + "golang_gauge":            {Type: "gauge", Help: "The gauge description string"},
				path.prefix + "golang_histogram":        {Type: "histogram", Help: "The histogram description string"},
				path.prefix + "golang_mixed_histogram":  {Type: "histogram", Help: "The mixed_histogram description string"},
				path.prefix + "golang_summary":          {Type: "summary", Help: "The summary description string"},
				path.prefix + "golang_native_histogram": {Type: "histogram", Help: "The native_histogram description string"},
			})
		})
	}
}

func prefixed(prefix string, names ...string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		out = append(out, prefix+name)
	}
	return out
}
