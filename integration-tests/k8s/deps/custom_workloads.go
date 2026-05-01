package deps

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

type CustomWorkloadsOptions struct {
	Path              string
	ManagedNamespaces []string
}

type CustomWorkloads struct {
	opts    CustomWorkloadsOptions
	absPath string
}

func NewCustomWorkloads(opts CustomWorkloadsOptions) *CustomWorkloads {
	return &CustomWorkloads{opts: opts}
}

func (w *CustomWorkloads) Name() string {
	return "custom-workloads"
}

func (w *CustomWorkloads) Install(_ *harness.TestContext) error {
	if w.opts.Path == "" {
		return fmt.Errorf("custom workloads path is required")
	}
	absPath, err := filepath.Abs(w.opts.Path)
	if err != nil {
		return fmt.Errorf("resolve custom workloads path: %w", err)
	}
	w.absPath = absPath
	for _, ns := range w.opts.ManagedNamespaces {
		if ns == "" {
			continue
		}
		if err := runCommand("kubectl", "get", "namespace", ns); err != nil {
			if createErr := runCommand("kubectl", "create", "namespace", ns); createErr != nil && !strings.Contains(createErr.Error(), "already exists") {
				return fmt.Errorf("ensure namespace %q for custom workloads: %w", ns, createErr)
			}
		}
	}
	return runCommand("kubectl", "apply", "-f", w.absPath)
}

func (w *CustomWorkloads) Cleanup() {
	if w.absPath == "" {
		return
	}
	_ = runCommand("kubectl", "delete", "-f", w.absPath, "--ignore-not-found=true")

	seen := make([]string, 0, len(w.opts.ManagedNamespaces))
	for _, ns := range w.opts.ManagedNamespaces {
		if ns == "" || slices.Contains(seen, ns) {
			continue
		}
		seen = append(seen, ns)
		_ = runCommand("kubectl", "delete", "namespace", ns, "--ignore-not-found=true", "--wait=true", "--timeout=10m")
	}
}
