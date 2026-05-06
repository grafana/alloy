package mimiralertskubernetes

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/stretchr/testify/require"
)

func TestMimirAlerts(t *testing.T) {
	ns := deps.NewNamespace(deps.NamespaceOptions{
		Name:   "test-mimir-alerts-kubernetes",
		Labels: map[string]string{"alloy-integration-test": "true"},
	})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-mimir-alerts-kubernetes",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	workloads := deps.NewCustomWorkloads(deps.CustomWorkloadsOptions{
		Path: "./config/workloads.yaml",
	})
	kt := harness.Setup(t, harness.Options{
		Name:         "mimir-alerts-kubernetes",
		Dependencies: []harness.Dependency{ns, workloads, mimir, alloy},
	})
	defer kt.Cleanup(t)

	kt.WaitForAllPodsRunning(t, ns.Name(), "app.kubernetes.io/name=alloy")
	kt.WaitForAllPodsRunning(t, ns.Name(), "app.kubernetes.io/component=alertmanager")

	t.Run("Initial Config loaded", func(t *testing.T) {
		mimir.CheckConfig(t, "./expected/expected_1.yml")
	})

	t.Run("Deleted Config works", func(t *testing.T) {
		require.NoError(t, harness.Kubectl("delete", "alertmanagerconfig", "alertmgr-config2", "--namespace", ns.Name()))

		// Mimir's config should now omit the deleted Alertmanagerconfig CRD.
		mimir.CheckConfig(t, "./expected/expected_2.yml")
	})
}
