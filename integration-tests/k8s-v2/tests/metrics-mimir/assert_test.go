//go:build alloyintegrationtests && k8sv2integrationtests

package metricsmimir

import (
	"context"
	"flag"
	"testing"

	k8sassert "github.com/grafana/alloy/integration-tests/k8s-v2/internal/assert"
)

var kubeconfigFlag = flag.String("k8s.v2.kubeconfig", "", "Path to kubeconfig used by k8s-v2 harness child tests")
var testIDFlag = flag.String("k8s.v2.test-id", "", "Runtime test ID used for isolated assertions")

func TestAssertions(t *testing.T) {
	kubeconfig := *kubeconfigFlag
	if kubeconfig == "" {
		t.Skip("skipping k8s-v2 assertion test: -k8s.v2.kubeconfig is required")
	}
	if *testIDFlag == "" {
		t.Skip("skipping k8s-v2 assertion test: -k8s.v2.test-id is required")
	}

	baseURL, cancelPortForward, err := k8sassert.StartBackendPortForward(context.Background(), kubeconfig, k8sassert.MimirBackend)
	if err != nil {
		t.Fatalf("start mimir port-forward: %v", err)
	}
	defer cancelPortForward()

	query := `prometheus_build_info{test_id="` + *testIDFlag + `"}`
	if err := k8sassert.EventuallyMimirQueryHasSeries(context.Background(), baseURL, query); err != nil {
		t.Fatalf("metrics to mimir assertion failed: %v", err)
	}
}
