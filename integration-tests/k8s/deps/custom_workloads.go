package deps

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

type CustomWorkloadsOptions struct {
	// Path is a YAML manifest applied on Install, deleted on Cleanup.
	Path string
	// Vars expands ${KEY} placeholders in the manifest. See util.SubstituteVars;
	// unresolved placeholders fail Install loudly.
	Vars map[string]string
}

type CustomWorkloads struct {
	opts    CustomWorkloadsOptions
	absPath string
}

func NewCustomWorkloads(opts CustomWorkloadsOptions) *CustomWorkloads {
	return &CustomWorkloads{opts: opts}
}

// Name includes the manifest's base filename so multiple instances are distinguishable in logs.
func (w *CustomWorkloads) Name() string {
	if w.opts.Path == "" {
		return "custom-workloads"
	}
	return "custom-workloads (" + filepath.Base(w.opts.Path) + ")"
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

	manifest, err := w.renderManifest()
	if err != nil {
		return err
	}
	return harness.ApplyManifest("", manifest)
}

func (w *CustomWorkloads) Cleanup() {
	if w.absPath == "" {
		return
	}
	manifest, err := w.renderManifest()
	if err != nil {
		// Cleanup is best-effort; log and move on.
		util.Logf("custom-workloads cleanup render failed: %v", err)
		return
	}
	_ = harness.DeleteManifest("", manifest)
}

func (w *CustomWorkloads) renderManifest() (string, error) {
	raw, err := os.ReadFile(w.absPath)
	if err != nil {
		return "", fmt.Errorf("read workloads %s: %w", w.absPath, err)
	}
	rendered, err := util.SubstituteVars(string(raw), w.opts.Vars)
	if err != nil {
		return "", fmt.Errorf("workloads %s: %w", w.absPath, err)
	}
	return rendered, nil
}
