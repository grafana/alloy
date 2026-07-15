package deps

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

const (
	// Must match manifests/alloy-otel.yaml.
	alloyOtelSelector  = "app=alloy-otel"
	alloyOtelConfigMap = "alloy-otel-config"
	// alloyImagePlaceholder is replaced with the image under test at install time.
	alloyImagePlaceholder = "__ALLOY_IMAGE__"
)

//go:embed manifests/alloy-otel.yaml
var alloyOtelManifest string

// Compile-time check that *AlloyOtel satisfies the harness.Dependency interface.
var _ harness.Dependency = (*AlloyOtel)(nil)

// AlloyOtel deploys Alloy running the OTel engine (`alloy otel`) via a raw
// manifest, with the alloyengine extension running an inner Alloy config. The
// Helm chart can't run `alloy otel` (its args hardcode `run`), hence the raw
// Deployment. CollectorConfigPath is the collector YAML; AlloyConfigPath is the
// inner .alloy config the alloyengine extension runs.
type AlloyOtel struct {
	opts      AlloyOtelOptions
	installed bool
}

type AlloyOtelOptions struct {
	Namespace           string
	CollectorConfigPath string
	AlloyConfigPath     string
}

func NewAlloyOtel(opts AlloyOtelOptions) *AlloyOtel {
	return &AlloyOtel{opts: opts}
}

func (a *AlloyOtel) Name() string { return "alloy-otel" }

func (a *AlloyOtel) Install(ctx *harness.TestContext) error {
	if a.opts.Namespace == "" {
		return fmt.Errorf("alloy-otel namespace is required")
	}
	image := os.Getenv(harness.AlloyImageEnv)
	if image == "" {
		return fmt.Errorf("%s must be set (the test runner sets this)", harness.AlloyImageEnv)
	}

	if err := util.Step("create alloy-otel configmap", func() error {
		return a.createConfigMap()
	}); err != nil {
		return err
	}
	a.installed = true

	manifest := strings.ReplaceAll(alloyOtelManifest, alloyImagePlaceholder, image)
	if err := util.Step("apply alloy-otel manifest", func() error {
		return harness.ApplyManifest(a.opts.Namespace, manifest)
	}); err != nil {
		return err
	}

	if err := util.Step("wait for alloy-otel pod ready", func() error {
		return harness.WaitForReady(a.opts.Namespace, alloyOtelSelector)
	}); err != nil {
		return err
	}

	ctx.AddDiagnosticHook("alloy-otel logs", func(c context.Context) error {
		return harness.RunDiagnosticCommands(c, [][]string{
			{"kubectl", "--namespace", a.opts.Namespace, "logs", "-l", alloyOtelSelector, "--all-containers=true", "--tail", "200"},
		})
	})
	return nil
}

func (a *AlloyOtel) Cleanup() {
	if !a.installed || a.opts.Namespace == "" {
		return
	}
	manifest := strings.ReplaceAll(alloyOtelManifest, alloyImagePlaceholder, os.Getenv(harness.AlloyImageEnv))
	_ = harness.DeleteManifest(a.opts.Namespace, manifest)
	_ = harness.RunCommand("kubectl", "delete", "configmap", alloyOtelConfigMap,
		"--namespace", a.opts.Namespace,
		"--ignore-not-found=true",
	)
}

// createConfigMap creates the "alloy-otel-config" ConfigMap with keys
// collector.yaml and config.alloy sourced from the given paths.
func (a *AlloyOtel) createConfigMap() error {
	collectorAbs, err := filepath.Abs(a.opts.CollectorConfigPath)
	if err != nil {
		return fmt.Errorf("resolve collector config path: %w", err)
	}
	alloyAbs, err := filepath.Abs(a.opts.AlloyConfigPath)
	if err != nil {
		return fmt.Errorf("resolve alloy config path: %w", err)
	}
	if err := harness.RunCommand("kubectl", "create", "configmap", alloyOtelConfigMap,
		"--namespace", a.opts.Namespace,
		"--from-file=collector.yaml="+collectorAbs,
		"--from-file=config.alloy="+alloyAbs,
	); err != nil {
		return fmt.Errorf("create alloy-otel configmap: %w", err)
	}
	return nil
}
