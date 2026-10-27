package otelcolentrypoint

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const testName = "otelcol-entrypoint"

// TestOtelcolEntrypoint runs Alloy's OTel engine through the image's
// /bin/otelcol wrapper, as tools that expect an otelcol binary do, with a plain
// collector pipeline. otel-gen sends every metric type to it, and each must
// reach Mimir as the right kind of sample.
func TestOtelcolEntrypoint(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-otelcol-entrypoint",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	alloyOtel := deps.NewAlloyOtel(deps.AlloyOtelOptions{
		Namespace:           ns.Name(),
		CollectorConfigPath: "./config/collector.yaml",
		OtelcolEntrypoint:   true,
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
