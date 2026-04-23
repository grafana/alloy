package deps

import (
	"context"
	"embed"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/kube"
)

const (
	defaultPollInterval   = 2 * time.Second
	defaultReadyTimeout   = 2 * time.Minute
	defaultUninstallGrace = 30 * time.Second
	LokiNamespace         = "loki"
	MimirNamespace        = "mimir"
)

// Env carries the configuration each installer needs at call time. It is
// passed explicitly so that deps has no mutable package-level state.
type Env struct {
	Kubeconfig       string
	Logger           *slog.Logger
	ReadinessTimeout time.Duration
	PollInterval     time.Duration
	UninstallGrace   time.Duration
	Debug            bool
}

func (e Env) withDefaults() Env {
	if e.ReadinessTimeout <= 0 {
		e.ReadinessTimeout = defaultReadyTimeout
	}
	if e.PollInterval <= 0 {
		e.PollInterval = defaultPollInterval
	}
	if e.UninstallGrace <= 0 {
		e.UninstallGrace = defaultUninstallGrace
	}
	if e.Logger == nil {
		e.Logger = slog.New(slog.NewTextHandler(os.Stderr, nil))
	}
	return e
}

//go:embed manifests/*.yaml
var manifestFS embed.FS

func readManifest(filename string) (string, error) {
	raw, err := manifestFS.ReadFile("manifests/" + filename)
	if err != nil {
		return "", fmt.Errorf("read embedded manifest %q: %w", filename, err)
	}
	return string(raw), nil
}

func runKubectl(ctx context.Context, env Env, args ...string) error {
	if env.Debug {
		env.Logger.Debug("kubectl command", "kubeconfig", env.Kubeconfig, "args", strings.Join(args, " "))
	}
	fullArgs := append([]string{"--kubeconfig", env.Kubeconfig}, args...)
	cmd := exec.CommandContext(ctx, "kubectl", fullArgs...)
	out, err := cmd.CombinedOutput()
	if env.Debug && len(out) > 0 {
		env.Logger.Debug("kubectl output", "output", string(out))
	}
	if err != nil {
		return fmt.Errorf("kubectl %v failed: %w: %s", args, err, string(out))
	}
	return nil
}

func applyManifest(ctx context.Context, env Env, manifest string) error {
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
	env.Logger.Info("applying manifest", "path", tmp.Name())
	return runKubectl(ctx, env, "apply", "-f", tmp.Name())
}

func deleteManifest(ctx context.Context, env Env, manifest string) error {
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

	deleteCtx, cancel := context.WithTimeout(ctx, env.UninstallGrace)
	defer cancel()
	env.Logger.Info("deleting manifest", "path", tmp.Name())
	return runKubectl(deleteCtx, env, "delete", "--ignore-not-found=true", "-f", tmp.Name())
}

func waitForDeployment(ctx context.Context, env Env, namespace, deployment string) error {
	waitCtx, cancel := context.WithTimeout(ctx, env.ReadinessTimeout)
	defer cancel()
	env.Logger.Info("waiting for deployment availability", "deployment", deployment, "namespace", namespace)

	err := runKubectl(waitCtx, env,
		"-n", namespace,
		"wait",
		"--for=condition=Available",
		"--timeout="+env.ReadinessTimeout.String(),
		"deployment/"+deployment,
	)
	if err != nil {
		return fmt.Errorf("kubernetes readiness check failed for deployment/%s: timeout=%s: %w", deployment, env.ReadinessTimeout, err)
	}
	env.Logger.Info("deployment is available", "deployment", deployment, "namespace", namespace)
	return nil
}

func checkServiceReadyEndpoint(ctx context.Context, env Env, namespace, service string, servicePort int, readinessPath string) error {
	handle, err := kube.StartPortForwardAndWait(ctx, kube.PortForwardConfig{
		Kubeconfig:    env.Kubeconfig,
		Namespace:     namespace,
		Service:       service,
		TargetPort:    servicePort,
		ReadinessPath: readinessPath,
		PollInterval:  env.PollInterval,
		ReadyTimeout:  env.ReadinessTimeout,
	})
	if err != nil {
		return fmt.Errorf("service readiness check failed for dependency=%s: %w", service, err)
	}
	if err := handle.Close(); err != nil {
		env.Logger.Debug("port-forward close warning", "service", service, "error", err)
	}
	env.Logger.Info("service readiness endpoint responded", "service", service, "namespace", namespace)
	return nil
}
