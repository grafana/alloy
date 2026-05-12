package deps

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const logGenSelector = "app=log-gen"

// LogGen applies a ConfigMap + StatefulSet that prints the lines from the
// supplied file to stdout. The container uses busybox directly, so no image
// build is needed.
type LogGen struct {
	opts      LogGenOptions
	workloads *CustomWorkloads
}

type LogGenOptions struct {
	Namespace string
	Replicas  int
	// FilePath is a text file whose contents become the log output. Resolved
	// relative to the test's working directory.
	FilePath string
}

func NewLogGen(opts LogGenOptions) *LogGen {
	return &LogGen{opts: opts}
}

func (l *LogGen) Name() string { return "log-gen" }

func (l *LogGen) Install(tc *harness.TestContext) error {
	if l.opts.Namespace == "" {
		return fmt.Errorf("log-gen namespace is required")
	}
	if l.opts.Replicas <= 0 {
		return fmt.Errorf("log-gen replicas must be > 0")
	}
	if l.opts.FilePath == "" {
		return fmt.Errorf("log-gen file path is required")
	}

	dir, err := pkgDir()
	if err != nil {
		return err
	}

	content, err := readContent(l.opts.FilePath)
	if err != nil {
		return err
	}

	l.workloads = NewCustomWorkloads(CustomWorkloadsOptions{
		Path: filepath.Join(dir, "manifests", "log-gen.yaml"),
		Vars: map[string]string{
			"NAMESPACE": l.opts.Namespace,
			"REPLICAS":  strconv.Itoa(l.opts.Replicas),
			"CONTENT":   content,
		},
	})
	if err := l.workloads.Install(tc); err != nil {
		return err
	}

	return harness.WaitForReady(l.opts.Namespace, logGenSelector)
}

func (l *LogGen) Cleanup() {
	if l.workloads != nil {
		l.workloads.Cleanup()
	}
}

func readContent(path string) (string, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve file path path: %w", err)
	}
	raw, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("read file content: %w", err)
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}
