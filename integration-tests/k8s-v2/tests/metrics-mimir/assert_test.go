package metricsmimir

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"

	k8sassert "github.com/grafana/alloy/integration-tests/k8s-v2/internal/assert"
)

func TestAssertions(t *testing.T) {
	kubeconfig := os.Getenv("ALLOY_K8S_V2_KUBECONFIG")
	if kubeconfig == "" {
		t.Fatal("ALLOY_K8S_V2_KUBECONFIG is required")
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
