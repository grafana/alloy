package main

import (
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/integration-tests-k8s/util"
)

// Running the test in a stateful way means that k8s resources won't be deleted at the end.
// It's useful for debugging. You can run the test like this:
// ALLOY_STATEFUL_K8S_TEST=true make integration-test-k8s
//
// After you're done with the test, run a command like this to clean up:
// minikube delete -p mimir-alerts-kubernetes
func isStateful() bool {
	stateful, _ := strconv.ParseBool(os.Getenv("ALLOY_STATEFUL_K8S_TEST"))
	return stateful
}

func TestMimirAlerts(t *testing.T) {
	testDataDir := "./testdata/"

	cleanupFunc := util.BootstrapTest(testDataDir, "mimir-alerts-kubernetes", isStateful())
	defer cleanupFunc()

	terminatePortFwd := util.ExecuteBackgroundCommand(
		"kubectl", []string{"port-forward", "service/mimir-nginx", "12346:80", "--namespace=mimir-test"},
		"Port forward Mimir")
	defer terminatePortFwd()

	tester := util.NewKubernetesTester(t)
	tester.WaitForPodRunning("testing", "app=grafana-alloy")
	tester.WaitForPodRunning("mimir-test", "app.kubernetes.io/component=alertmanager")

	checkMimirConfig(t, testDataDir, "expected_1.yml")

	util.ExecuteCommand(
		"kubectl", []string{"delete", "alertmanagerconfig", "alertmgr-config2", "-n", "testing"},
		"Delete an Alertmanagerconfig CRD")

	// Mimir's config should now omit the deleted Alertmanagerconfig CRD.
	checkMimirConfig(t, testDataDir, "expected_2.yml")
}

func checkMimirConfig(t *testing.T, testDataDir, expectedFile string) {
	expectedMimirConfigBytes, err := os.ReadFile(testDataDir + expectedFile)
	require.NoError(t, err)
	expectedMimirConfig := string(expectedMimirConfigBytes)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		actualMimirConfig := util.Curl(c, "http://localhost:12346/api/v1/alerts")
		require.Equal(c, expectedMimirConfig, actualMimirConfig)
	}, 5*time.Minute, 500*time.Millisecond)
}
