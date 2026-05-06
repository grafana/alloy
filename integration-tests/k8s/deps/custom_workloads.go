package deps

import (
	"fmt"
	"path/filepath"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

type CustomWorkloadsOptions struct {
	// Path is the path to a YAML manifest applied with `kubectl apply -f` on
	// Install and removed with `kubectl delete -f` on Cleanup. Any namespaces
	// the test needs (including the test namespace) should be declared in this
	// file so they are created on Install and deleted on Cleanup.
	Path string
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
	return runCommand("kubectl", "apply", "-f", w.absPath)
}

func (w *CustomWorkloads) Cleanup() {
	if w.absPath == "" {
		return
	}
	_ = runCommand("kubectl", "delete", "-f", w.absPath, "--ignore-not-found=true", "--wait=true", "--timeout=10m")
}
