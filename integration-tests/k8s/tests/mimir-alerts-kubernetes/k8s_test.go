package mimiralertskubernetes

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/stretchr/testify/require"
)

func TestMimirAlerts(t *testing.T) {
	kt := harness.Setup(t, harness.Options{
		Name:       "mimir-alerts-kubernetes",
		ConfigPath: "./config/config.alloy",
		Workloads:  "./config/workloads.yaml",
		Backends:   []harness.Backend{harness.BackendMimir},
		Controller: "deployment",
	})
	defer kt.Cleanup(t)
	mimir := kt.Mimir(t)

	kt.WaitForPodRunning(t, kt.Namespace(), "app.kubernetes.io/name=alloy")
	kt.WaitForPodRunning(t, kt.Namespace(), "app.kubernetes.io/component=alertmanager")

	t.Run("Initial Config", func(t *testing.T) {
		mimir.CheckConfig(t, "./expected/expected_1.yml")
	})

	t.Run("Deleted Config", func(t *testing.T) {
		require.NoError(t, harness.Kubectl("delete", "alertmanagerconfig", "alertmgr-config2", "--namespace", kt.Namespace()))

		// Mimir's config should now omit the deleted Alertmanagerconfig CRD.
		mimir.CheckConfig(t, "./expected/expected_2.yml")
	})
}
