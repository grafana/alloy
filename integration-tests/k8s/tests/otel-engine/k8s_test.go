package otelengine

import (
	"fmt"
	"runtime"
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

// TestOtelEngine runs Alloy with the OTel engine (`alloy otel`), which runs the
// inner Alloy config via the alloyengine extension, and checks the product name
// in its outbound User-Agent.
func TestOtelEngine(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-otel-engine",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	nginx := deps.NewNginxProxy(deps.NginxProxyOptions{Namespace: ns.Name()})
	alloyOtel := deps.NewAlloyOtel(deps.AlloyOtelOptions{
		Namespace:           ns.Name(),
		CollectorConfigPath: "./config/collector.yaml",
		AlloyConfigPath:     "./config/config.alloy",
	})

	// nginx after mimir so the upstream Service exists when nginx starts; alloy
	// last so it pushes once everything downstream is ready.
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, mimir, nginx, alloyOtel},
	})

	t.Run("ExtensionUserAgent", func(t *testing.T) {
		deps.AssertUserAgentEquals(t, nginx, "Alloy OTel Extension/v123.456.789 (linux; docker)")
	})

	t.Run("NativeOtelExporterUserAgent", func(t *testing.T) {
		// A native collector pipeline (hostmetrics -> otlphttp, defined in collector.yaml,
		// not via the alloyengine extension) reports the collector's build info.
		want := fmt.Sprintf("Alloy OTel Collector distribution./v123.456.789 (linux/%s)", runtime.GOARCH)
		deps.AssertUserAgentEquals(t, nginx, want)
	})
}
