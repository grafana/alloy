package deps

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

const (
	defaultNamespace      = "k8s-v2-observability"
	defaultPollInterval   = 2 * time.Second
	defaultReadyTimeout   = 2 * time.Minute
	defaultUninstallGrace = 30 * time.Second
)

func runKubectl(ctx context.Context, kubeconfig string, args ...string) ([]byte, error) {
	fullArgs := append([]string{"--kubeconfig", kubeconfig}, args...)
	cmd := exec.CommandContext(ctx, "kubectl", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return out, fmt.Errorf("kubectl %v failed: %w: %s", args, err, string(out))
	}
	return out, nil
}

func applyManifest(ctx context.Context, kubeconfig string, manifest string) error {
	tmp, err := os.CreateTemp("", "k8s-v2-manifest-*.yaml")
	if err != nil {
		return fmt.Errorf("create manifest temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(manifest); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write manifest temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close manifest temp file: %w", err)
	}

	_, err = runKubectl(ctx, kubeconfig, "apply", "-f", tmp.Name())
	return err
}

func deleteManifest(ctx context.Context, kubeconfig string, manifest string) error {
	tmp, err := os.CreateTemp("", "k8s-v2-manifest-delete-*.yaml")
	if err != nil {
		return fmt.Errorf("create manifest temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.WriteString(manifest); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write manifest temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close manifest temp file: %w", err)
	}

	deleteCtx, cancel := context.WithTimeout(ctx, defaultUninstallGrace)
	defer cancel()

	_, err = runKubectl(deleteCtx, kubeconfig, "delete", "--ignore-not-found=true", "-f", tmp.Name())
	return err
}

func waitForDeployment(ctx context.Context, kubeconfig, namespace, deployment string) error {
	waitCtx, cancel := context.WithTimeout(ctx, defaultReadyTimeout)
	defer cancel()

	_, err := runKubectl(waitCtx, kubeconfig,
		"-n", namespace,
		"wait",
		"--for=condition=Available",
		"--timeout=120s",
		"deployment/"+deployment,
	)
	if err != nil {
		return fmt.Errorf("kubernetes readiness check failed for deployment/%s: timeout=%s: %w", deployment, defaultReadyTimeout, err)
	}
	return nil
}

func checkServiceReadyEndpoint(
	ctx context.Context,
	kubeconfig, namespace, service string,
	localPort, servicePort int,
	readyURL string,
) error {
	portForwardCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(
		portForwardCtx,
		"kubectl",
		"--kubeconfig", kubeconfig,
		"-n", namespace,
		"port-forward",
		"svc/"+service,
		fmt.Sprintf("%d:%d", localPort, servicePort),
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("create port-forward stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("create port-forward stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start port-forward for %s: %w", service, err)
	}
	defer func() {
		cancel()
		_, _ = io.ReadAll(stdout)
		_, _ = io.ReadAll(stderr)
		_ = cmd.Wait()
	}()

	readyCtx, readyCancel := context.WithTimeout(ctx, defaultReadyTimeout)
	defer readyCancel()

	var lastErr error
	for {
		req, err := http.NewRequestWithContext(readyCtx, http.MethodGet, readyURL, nil)
		if err != nil {
			return fmt.Errorf("build readiness request for %s: %w", service, err)
		}

		resp, err := http.DefaultClient.Do(req)
		if err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			_ = resp.Body.Close()
			return nil
		}
		if resp != nil {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("status %d", resp.StatusCode)
		} else {
			lastErr = err
		}

		select {
		case <-readyCtx.Done():
			return fmt.Errorf(
				"service readiness check failed: dependency=%s check=GET %s timeout=%s last_state=%v",
				service, readyURL, defaultReadyTimeout, lastErr,
			)
		case <-time.After(defaultPollInterval):
		}
	}
}
