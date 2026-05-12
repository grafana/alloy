package deps

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

const (
	logGenImageTag = "log-gen:test"
	logGenSelector = "app=log-gen"
)

// LogGen builds the log-producer image and applies the log-gen manifest into
// the target namespace. The actual work is delegated to CustomImage and
// CustomWorkloads.
type LogGen struct {
	opts      LogGenOptions
	image     *CustomImage
	workloads *CustomWorkloads
}

type LogGenOptions struct {
	Namespace string
	Replicas  int
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

	pkgDir, err := pkgDir()
	fmt.Println(pkgDir)
	if err != nil {
		return err
	}

	l.image = NewCustomImage(CustomImageOptions{
		Tag:            logGenImageTag,
		ContextPath:    filepath.Join(pkgDir, "images"),
		DockerfilePath: filepath.Join(pkgDir, "images", "Dockerfile.loggen"),
	})
	if err := l.image.Install(tc); err != nil {
		return err
	}

	l.workloads = NewCustomWorkloads(CustomWorkloadsOptions{
		Path: filepath.Join(pkgDir, "manifests", "log-gen.yaml"),
		Vars: map[string]string{
			"NAMESPACE": l.opts.Namespace,
			"REPLICAS":  strconv.Itoa(l.opts.Replicas),
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
	if l.image != nil {
		l.image.Cleanup()
	}
}
