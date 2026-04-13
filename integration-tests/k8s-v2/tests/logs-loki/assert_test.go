//go:build alloyintegrationtests && k8sv2integrationtests

package logsloki

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
	expectedLine := "INFO sync complete test_id=" + *testIDFlag + " component=writer seq=07"

	baseURL, cancelPortForward, err := k8sassert.StartBackendPortForward(context.Background(), kubeconfig, k8sassert.LokiBackend)
	if err != nil {
		t.Fatalf("start loki port-forward: %v", err)
	}
	defer cancelPortForward()

	if err := k8sassert.EventuallyLokiQueryContainsExactLine(
		context.Background(),
		baseURL,
		`{test_id="`+*testIDFlag+`"}`,
		expectedLine,
	); err != nil {
		t.Fatalf("logs to loki assertion failed: %v", err)
	}
}
