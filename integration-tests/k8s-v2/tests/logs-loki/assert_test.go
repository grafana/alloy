package logsloki

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

	cancelPortForward, err := startPortForward(kubeconfig, "k8s-v2-observability", "loki", "33100:3100")
	if err != nil {
		t.Fatalf("start loki port-forward: %v", err)
	}
	defer cancelPortForward()

	if err := k8sassert.EventuallyLokiQueryContainsLine(
		context.Background(),
		"http://127.0.0.1:33100",
		`{job="k8s-v2-file-logs"}`,
		"k8s-v2-log-line",
	); err != nil {
		t.Fatalf("logs to loki assertion failed: %v", err)
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
