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
	promOp := deps.NewPrometheusOperator(deps.PrometheusOperatorOptions{})
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: ns.Name()})
	extraManifests := deps.NewCustomWorkloads(deps.CustomWorkloadsOptions{
		Path: "./config/workloads.yaml",
		Vars: map[string]string{"NAMESPACE": ns.Name()},
	})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  ns.Name(),
		Release:    "alloy-test-mimir-alerts",
		ConfigPath: "./config/config.alloy",
		ValuesPath: "./config/alloy-values.yaml",
	})
	harness.Setup(t, harness.Options{
		Dependencies: []harness.Dependency{ns, promOp, extraManifests, mimir, alloy},
	})

	t.Run("Initial Config loaded", func(t *testing.T) {
		mimir.CheckAlertsConfig(t, "./expected/expected_1.yml")
	})

	t.Run("Deleted Config works", func(t *testing.T) {
		require.NoError(t, harness.RunCommand("kubectl", "delete", "alertmanagerconfig", "alertmgr-config2", "--namespace", ns.Name()))

		// Mimir's config should now omit the deleted Alertmanagerconfig CRD.
		mimir.CheckAlertsConfig(t, "./expected/expected_2.yml")
	})
}
