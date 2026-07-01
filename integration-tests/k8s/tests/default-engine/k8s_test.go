package defaultengine

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

// TestDefaultEngine deploys Alloy with the default engine (`alloy run`, via the
// Helm chart) and checks the product name in its outbound User-Agent.
func TestDefaultEngine(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-default-engine",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	nginx := deps.NewNginxProxy(deps.NginxProxyOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-default-engine",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})

	// nginx after mimir so the upstream Service exists when nginx starts; alloy
	// last so it pushes once everything downstream is ready.
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, mimir, nginx, alloy},
	})

	t.Run("RemoteWriteUserAgent", func(t *testing.T) {
		// The default engine is deployed via Helm (deploy mode "helm").
		deps.AssertUserAgentEquals(t, nginx, "Alloy/v123.456.789 (linux; helm)")
	})
}

//TODO: Add more checks. We should get enough metrics so that Alloy Health dashboards can populate?
