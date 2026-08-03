package metamonitorotel

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

// TestMetamonitorOtel runs Alloy with the OTel engine (`alloy otel`), which runs the
// inner Alloy config via the alloyengine extension, and checks the product name
// in its outbound User-Agent.
func TestMetamonitorOtel(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-metamonitor-otel",
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
		deps.AssertUserAgentPrefix(t, nginx, "Alloy OTel Extension/")
	})

	t.Run("NativeOtelExporterUserAgent", func(t *testing.T) {
		deps.AssertUserAgentPrefix(t, nginx, "Alloy OTel Collector distribution./")
	})
}
