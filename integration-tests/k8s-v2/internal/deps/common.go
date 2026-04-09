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
	lokiNamespace         = "loki"
	mimirNamespace        = "mimir"
)

type Config struct {
	ReadinessTimeout time.Duration
	PollInterval     time.Duration
	UninstallGrace   time.Duration
	Debug            bool
	Logger           *slog.Logger
}

var cfg = Config{
	ReadinessTimeout: defaultReadyTimeout,
	PollInterval:     defaultPollInterval,
	UninstallGrace:   defaultUninstallGrace,
	Debug:            false,
	Logger:           slog.New(slog.NewTextHandler(os.Stderr, nil)),
}

func Configure(config Config) {
	if config.ReadinessTimeout > 0 {
		cfg.ReadinessTimeout = config.ReadinessTimeout
	}
	if config.PollInterval > 0 {
		cfg.PollInterval = config.PollInterval
	}
	if config.UninstallGrace > 0 {
		cfg.UninstallGrace = config.UninstallGrace
	}
	cfg.Debug = config.Debug
	if config.Logger != nil {
		cfg.Logger = config.Logger.With("component", "deps")
	}
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

func runKubectl(ctx context.Context, kubeconfig string, args ...string) error {
	if cfg.Debug {
		cfg.Logger.Debug("kubectl command", "kubeconfig", kubeconfig, "args", strings.Join(args, " "))
	}
	fullArgs := append([]string{"--kubeconfig", kubeconfig}, args...)
	cmd := exec.CommandContext(ctx, "kubectl", fullArgs...)
	out, err := cmd.CombinedOutput()
	if cfg.Debug && len(out) > 0 {
		cfg.Logger.Debug("kubectl output", "output", string(out))
	}
	if err != nil {
		return fmt.Errorf("kubectl %v failed: %w: %s", args, err, string(out))
	}
	return nil
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
	if cfg.Debug {
		cfg.Logger.Debug("applying manifest file", "path", tmp.Name())
	}
	cfg.Logger.Info("applying manifest", "path", tmp.Name())

	return runKubectl(ctx, kubeconfig, "apply", "-f", tmp.Name())
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

	deleteCtx, cancel := context.WithTimeout(ctx, cfg.UninstallGrace)
	defer cancel()
	if cfg.Debug {
		cfg.Logger.Debug("deleting manifest file", "path", tmp.Name(), "timeout", cfg.UninstallGrace)
	}
	cfg.Logger.Info("deleting manifest", "path", tmp.Name())

	return runKubectl(deleteCtx, kubeconfig, "delete", "--ignore-not-found=true", "-f", tmp.Name())
}

func waitForDeployment(ctx context.Context, kubeconfig, namespace, deployment string) error {
	waitCtx, cancel := context.WithTimeout(ctx, cfg.ReadinessTimeout)
	defer cancel()
	if cfg.Debug {
		cfg.Logger.Debug("waiting for deployment availability", "deployment", deployment, "namespace", namespace, "timeout", cfg.ReadinessTimeout)
	}
	cfg.Logger.Info("waiting for deployment availability", "deployment", deployment, "namespace", namespace)

	err := runKubectl(waitCtx, kubeconfig,
		"-n", namespace,
		"wait",
		"--for=condition=Available",
		"--timeout="+cfg.ReadinessTimeout.String(),
		"deployment/"+deployment,
	)
	if err != nil {
		return fmt.Errorf("kubernetes readiness check failed for deployment/%s: timeout=%s: %w", deployment, cfg.ReadinessTimeout, err)
	}
	cfg.Logger.Info("deployment is available", "deployment", deployment, "namespace", namespace)
	return nil
}

func checkServiceReadyEndpoint(
	ctx context.Context,
	kubeconfig, namespace, service string,
	localPort, servicePort int,
	readyURL string,
) error {

	handle, err := kube.StartPortForwardAndWait(ctx, kube.PortForwardConfig{
		Kubeconfig:    kubeconfig,
		Namespace:     namespace,
		Service:       service,
		TargetPort:    servicePort,
		ReadinessPath: strings.TrimPrefix(readyURL, fmt.Sprintf("http://127.0.0.1:%d", localPort)),
		PollInterval:  cfg.PollInterval,
		ReadyTimeout:  cfg.ReadinessTimeout,
	})
	if err != nil {
		return fmt.Errorf("service readiness check failed for dependency=%s: %w", service, err)
	}
	if err := handle.Close(); err != nil {
		cfg.Logger.Debug("port-forward close warning", "service", service, "error", err)
	}
	cfg.Logger.Info("service readiness endpoint responded", "service", service, "namespace", namespace)
	return nil
}
