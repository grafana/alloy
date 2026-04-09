//go:build alloyintegrationtests && k8sv2integrationtests

package metricsmimir

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os/exec"
	"testing"

	k8sassert "github.com/grafana/alloy/integration-tests/k8s-v2/internal/assert"
)

var kubeconfigFlag = flag.String("k8s.v2.kubeconfig", "", "Path to kubeconfig used by k8s-v2 harness child tests")

func TestAssertions(t *testing.T) {
	kubeconfig := *kubeconfigFlag
	if kubeconfig == "" {
		t.Skip("skipping k8s-v2 assertion test: -k8s.v2.kubeconfig is required")
	}

	cancelPortForward, err := startPortForward(kubeconfig, "k8s-v2-observability", "mimir", "39009:9009")
	if err != nil {
		t.Fatalf("start mimir port-forward: %v", err)
	}
	defer cancelPortForward()

	if err := k8sassert.EventuallyMimirQueryHasSeries(context.Background(), "http://127.0.0.1:39009", "prometheus_build_info"); err != nil {
		t.Fatalf("metrics to mimir assertion failed: %v", err)
	}
}

func startPortForward(kubeconfig, namespace, service, ports string) (func(), error) {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(
		ctx,
		"kubectl",
		"--kubeconfig", kubeconfig,
		"-n", namespace,
		"port-forward",
		"svc/"+service,
		ports,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, fmt.Errorf("start port-forward: %w", err)
	}

	return func() {
		cancel()
		_, _ = io.ReadAll(stdout)
		_, _ = io.ReadAll(stderr)
		_ = cmd.Wait()
	}, nil
}
