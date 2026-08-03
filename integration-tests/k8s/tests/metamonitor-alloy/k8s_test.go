package metamonitoralloy

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

// TestMetamonitorAlloy deploys Alloy with the default engine (`alloy run`, via the
// Helm chart) and checks the product name in its outbound User-Agent.
func TestMetamonitorAlloy(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-metamonitor-alloy",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	nginx := deps.NewNginxProxy(deps.NginxProxyOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-metamonitor-alloy",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})

	// nginx after mimir so the upstream Service exists when nginx starts; alloy
	// last so it pushes once everything downstream is ready.
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, mimir, nginx, alloy},
	})

	t.Run("RemoteWriteUserAgent", func(t *testing.T) {
		deps.AssertUserAgentPrefix(t, nginx, "Alloy/")
	})
}

//TODO: Add more checks. We should get enough metrics so that Alloy Health dashboards can populate?
