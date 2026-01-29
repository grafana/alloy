//go:build alloyintegrationtests

package main

import (
	"testing"

	"github.com/grafana/alloy/integration-tests/k8s/util"
)

const mimirPort = "12346"

func TestMimirAlerts(t *testing.T) {
	testDataDir := "./testdata/"

	cleanupFunc := util.BootstrapTest(testDataDir, "mimir-alerts-kubernetes")
	defer cleanupFunc()

	terminatePortFwd := util.ExecuteBackgroundCommand(
		"kubectl", []string{"port-forward", "service/mimir-nginx", mimirPort + ":80", "--namespace=mimir-test"},
		"Port forward Mimir")
	defer terminatePortFwd()

	kt := util.NewKubernetesTester(t)
	kt.WaitForPodRunning(t, "testing", "app=grafana-alloy")
	kt.WaitForPodRunning(t, "mimir-test", "app.kubernetes.io/component=alertmanager")

	t.Run("Initial Config", func(t *testing.T) {
		kt.CheckMimirConfig(t, testDataDir, mimirPort, "expected_1.yml")
	})

	t.Run("Deleted Config", func(t *testing.T) {
		util.ExecuteCommand(
			"kubectl", []string{"delete", "alertmanagerconfig", "alertmgr-config2", "-n", "testing"},
			"Delete an Alertmanagerconfig CRD")

		// Mimir's config should now omit the deleted Alertmanagerconfig CRD.
		kt.CheckMimirConfig(t, testDataDir, mimirPort, "expected_2.yml")
	})
}
