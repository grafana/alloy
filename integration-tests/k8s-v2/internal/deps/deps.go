// Package deps installs and uninstalls the shared test backends (Loki,
// Mimir, ...) that k8s-v2 tests assert against. Specs are the single
// source of truth for each backend's namespace/service/port/manifest and
// are also consumed by the assert package for port-forward targets.
package deps

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/kube"
)

const (
	defaultPollInterval   = 2 * time.Second
	defaultReadyTimeout   = 2 * time.Minute
	defaultUninstallGrace = 30 * time.Second
)

// Spec describes a manifest-installed test backend.
type Spec struct {
	Name          string
	Namespace     string
	Service       string
	Deployment    string
	Port          int
	ReadinessPath string
	// ManifestFile is the file name under manifests/ with the kubernetes
	// objects to apply.
	ManifestFile string
}

// Loki / Mimir are the backends k8s-v2 currently supports. Adding a new
// backend is a matter of appending to All, adding an embedded manifest,
// and referencing it by Name in a test's requirements.yaml.
var (
	Loki = Spec{
		Name: "loki", Namespace: "loki", Service: "loki", Deployment: "loki",
		Port: 3100, ReadinessPath: "/ready", ManifestFile: "loki.yaml",
	}
	Mimir = Spec{
		Name: "mimir", Namespace: "mimir", Service: "mimir", Deployment: "mimir",
		Port: 9009, ReadinessPath: "/ready", ManifestFile: "mimir.yaml",
	}
	All = []Spec{Loki, Mimir}
)

// Env carries the per-run configuration install/uninstall need. It is
// passed explicitly so this package has no mutable state.
type Env struct {
	Kubeconfig       string
	Logger           *slog.Logger
	ReadinessTimeout time.Duration
	PollInterval     time.Duration
	UninstallGrace   time.Duration
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

// Resolve returns the Specs for the given names in the order given. An
// unknown name yields an error so callers fail early.
func Resolve(names []string) ([]Spec, error) {
	out := make([]Spec, 0, len(names))
	var unknown []string
	for _, n := range names {
		found := false
		for _, s := range All {
			if s.Name == n {
				out = append(out, s)
				found = true
				break
			}
		}
		if !found {
			unknown = append(unknown, n)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return nil, fmt.Errorf("unknown dependencies: %v", unknown)
	}
	return out, nil
}

// Install concurrently installs all specs and waits for readiness. On any
// failure it rolls back successfully installed specs in reverse order so
// the cluster does not leak resources.
func Install(ctx context.Context, env Env, specs []Spec) error {
	env = env.withDefaults()

	installed := make([]Spec, 0, len(specs))
	failures := make(map[string]error)
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, spec := range specs {
		spec := spec
		wg.Add(1)
		go func() {
			defer wg.Done()
			env.Logger.Info("installing dependency", "dependency", spec.Name)
			if err := installOne(ctx, env, spec); err != nil {
				env.Logger.Warn("dependency install failed", "dependency", spec.Name, "error", err)
				mu.Lock()
				failures[spec.Name] = err
				mu.Unlock()
				return
			}
			env.Logger.Info("dependency ready", "dependency", spec.Name)
			mu.Lock()
			installed = append(installed, spec)
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(failures) == 0 {
		return nil
	}
	if len(installed) > 0 {
		env.Logger.Info("rolling back partially installed dependencies",
			"installed", specNames(installed))
		if err := Uninstall(ctx, env, installed); err != nil {
			return fmt.Errorf("installs failed (%s) and rollback failed: %w", failureSummary(failures), err)
		}
	}
	return fmt.Errorf("dependency installs failed: %s", failureSummary(failures))
}

// Uninstall removes every spec. A single failure does not abort the run:
// remaining deps are still uninstalled and all errors are joined.
// Uninstall order is the reverse of the caller-supplied specs for symmetry
// with a future dependency-ordered install path; today installs run
// concurrently so the reverse is cosmetic.
func Uninstall(ctx context.Context, env Env, specs []Spec) error {
	env = env.withDefaults()
	reversed := slices.Clone(specs)
	slices.Reverse(reversed)

	var errs []error
	for _, spec := range reversed {
		env.Logger.Info("uninstalling dependency", "dependency", spec.Name)
		if err := uninstallOne(ctx, env, spec); err != nil {
			env.Logger.Warn("dependency uninstall failed", "dependency", spec.Name, "error", err)
			errs = append(errs, fmt.Errorf("uninstall %q: %w", spec.Name, err))
			continue
		}
		env.Logger.Info("dependency removed", "dependency", spec.Name)
	}
	return errors.Join(errs...)
}

func installOne(ctx context.Context, env Env, spec Spec) error {
	manifest, err := readManifest(spec.ManifestFile)
	if err != nil {
		return err
	}
	if err := applyManifest(ctx, env, manifest); err != nil {
		return err
	}
	if err := waitForDeployment(ctx, env, spec); err != nil {
		return err
	}
	if err := checkReady(ctx, env, spec); err != nil {
		return fmt.Errorf("dependency=%s: %w", spec.Name, err)
	}
	return nil
}

func uninstallOne(ctx context.Context, env Env, spec Spec) error {
	manifest, err := readManifest(spec.ManifestFile)
	if err != nil {
		return err
	}
	return deleteManifest(ctx, env, manifest)
}

//go:embed manifests/*.yaml
var manifestFS embed.FS

func readManifest(name string) (string, error) {
	raw, err := manifestFS.ReadFile("manifests/" + name)
	if err != nil {
		return "", fmt.Errorf("read embedded manifest %q: %w", name, err)
	}
	return string(raw), nil
}

func applyManifest(ctx context.Context, env Env, manifest string) error {
	return runWithManifest(ctx, env, manifest, "apply", 0)
}

func deleteManifest(ctx context.Context, env Env, manifest string) error {
	return runWithManifest(ctx, env, manifest, "delete", env.UninstallGrace)
}

func runWithManifest(ctx context.Context, env Env, manifest, action string, timeout time.Duration) error {
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

	runCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		runCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	args := []string{"--kubeconfig", env.Kubeconfig, action}
	if action == "delete" {
		args = append(args, "--ignore-not-found=true")
	}
	args = append(args, "-f", tmp.Name())
	cmd := exec.CommandContext(runCtx, "kubectl", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl %s failed: %w: %s", action, err, out)
	}
	return nil
}

func waitForDeployment(ctx context.Context, env Env, spec Spec) error {
	waitCtx, cancel := context.WithTimeout(ctx, env.ReadinessTimeout)
	defer cancel()
	env.Logger.Info("waiting for deployment availability",
		"deployment", spec.Deployment, "namespace", spec.Namespace)
	cmd := exec.CommandContext(waitCtx, "kubectl",
		"--kubeconfig", env.Kubeconfig,
		"-n", spec.Namespace,
		"wait", "--for=condition=Available",
		"--timeout="+env.ReadinessTimeout.String(),
		"deployment/"+spec.Deployment,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("readiness check failed for deployment/%s: %w: %s",
			spec.Deployment, err, out)
	}
	return nil
}

func checkReady(ctx context.Context, env Env, spec Spec) error {
	handle, err := kube.StartPortForwardAndWait(ctx, kube.PortForwardConfig{
		Kubeconfig:    env.Kubeconfig,
		Namespace:     spec.Namespace,
		Service:       spec.Service,
		TargetPort:    spec.Port,
		ReadinessPath: spec.ReadinessPath,
		PollInterval:  env.PollInterval,
		ReadyTimeout:  env.ReadinessTimeout,
	})
	if err != nil {
		return fmt.Errorf("service readiness check for %s: %w", spec.Service, err)
	}
	// Close cancels the port-forward process; kubectl exits with "signal:
	// killed" which is expected here (readiness was already confirmed).
	if err := handle.Close(); err != nil {
		env.Logger.Debug("port-forward close warning", "service", spec.Service, "error", err)
	}
	return nil
}

func specNames(specs []Spec) []string {
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = s.Name
	}
	return out
}

func failureSummary(failures map[string]error) string {
	names := make([]string, 0, len(failures))
	for n := range failures {
		names = append(names, n)
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, n := range names {
		parts[i] = fmt.Sprintf("%s=%v", n, failures[n])
	}
	return strings.Join(parts, ", ")
}
