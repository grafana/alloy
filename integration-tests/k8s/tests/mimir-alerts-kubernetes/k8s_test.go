package mimiralertskubernetes

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/deps"
	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/stretchr/testify/require"
)

func TestMimirAlerts(t *testing.T) {
	testNamespace := "test-mimir-alerts-kubernetes"
	mimir := deps.NewMimir(deps.MimirOptions{Namespace: testNamespace})
	alloy := deps.NewAlloy(deps.AlloyOptions{
		Namespace:  testNamespace,
		ConfigPath: "./config/config.alloy",
		Controller: "deployment",
		Release:    "alloy-mimir-alerts-kubernetes",
	})
	workloads := deps.NewCustomWorkloads(deps.CustomWorkloadsOptions{
		Path:              "./config/workloads.yaml",
		ManagedNamespaces: []string{testNamespace, "othernamespace"},
	})
	kt := harness.Setup(t, harness.Options{
		Name:         "mimir-alerts-kubernetes",
		Dependencies: []harness.Dependency{mimir, workloads, alloy},
		Namespace:    testNamespace,
	})
	defer kt.Cleanup(t)

	kt.WaitForAllPodsRunning(t, testNamespace, "app.kubernetes.io/name=alloy")
	kt.WaitForAllPodsRunning(t, testNamespace, "app.kubernetes.io/component=alertmanager")

	t.Run("Initial Config loaded", func(t *testing.T) {
		mimir.CheckConfig(t, "./expected/expected_1.yml")
	})

	t.Run("Deleted Config works", func(t *testing.T) {
		require.NoError(t, harness.Kubectl("delete", "alertmanagerconfig", "alertmgr-config2", "--namespace", testNamespace))

		// Mimir's config should now omit the deleted Alertmanagerconfig CRD.
		mimir.CheckConfig(t, "./expected/expected_2.yml")
	})
}
