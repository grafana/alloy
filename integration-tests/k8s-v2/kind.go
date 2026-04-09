//go:build alloyintegrationtests && k8sv2integrationtests

package k8sv2

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func kindClusterExists(ctx context.Context, name string) (bool, error) {
	cmd := exec.CommandContext(ctx, "kind", "get", "clusters")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("kind get clusters failed: %w: %s", err, string(out))
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(line) == name {
			return true, nil
		}
	}
	return false, nil
}

func kindGetKubeconfig(ctx context.Context, name string) (string, error) {
	cmd := exec.CommandContext(ctx, "kind", "get", "kubeconfig", "--name", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("kind get kubeconfig --name %s failed: %w: %s", name, err, string(out))
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("kind-cluster-%s-kubecfg", name))
	if err != nil {
		return "", fmt.Errorf("create kubeconfig temp file: %w", err)
	}
	if _, err := tmpFile.Write(out); err != nil {
		_ = tmpFile.Close()
		return "", fmt.Errorf("write kubeconfig temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return "", fmt.Errorf("close kubeconfig temp file: %w", err)
	}
	return tmpFile.Name(), nil
}
