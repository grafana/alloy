package alloyengineextension

import (
	"testing"

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

	deps.AssertOtelGenMetrics(t, mimir, testName)
}
