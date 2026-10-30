package deps

import (
	"context"
	_ "embed"
	"encoding/json"
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
	// alloyOtelCommandPlaceholder is replaced with the container command, as a
	// JSON array, which YAML reads as a flow sequence.
	alloyOtelCommandPlaceholder = "__ALLOY_OTEL_COMMAND__"
)

var (
	alloyOtelCommand = []string{"/bin/alloy", "otel"}
	// otelcolCommand is the image's wrapper for tools that expect an otelcol
	// binary. It runs `alloy otel` with the same arguments.
	otelcolCommand = []string{"/bin/otelcol"}
)

//go:embed manifests/alloy-otel.yaml
var alloyOtelManifest string

// Compile-time check that *AlloyOtel satisfies the harness.Dependency interface.
var _ harness.Dependency = (*AlloyOtel)(nil)

// AlloyOtel deploys Alloy running the OTel engine (`alloy otel`) via a raw
// manifest. The Helm chart can't run `alloy otel` (its args hardcode `run`),
// hence the raw Deployment.
type AlloyOtel struct {
	opts      AlloyOtelOptions
	manifest  string
	installed bool
}

type AlloyOtelOptions struct {
	Namespace string
	// CollectorConfigPath is the collector YAML.
	CollectorConfigPath string
	// AlloyConfigPath is the inner .alloy config, mounted as
	// /etc/alloy/config.alloy for the alloyengine extension to run. Optional,
	// for collector configs without the extension.
	AlloyConfigPath string
	// OtelcolEntrypoint starts the collector through the image's /bin/otelcol
	// wrapper, as tools that expect an otelcol binary do, instead of running
	// `alloy otel` directly.
	OtelcolEntrypoint bool
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

	command := alloyOtelCommand
	if a.opts.OtelcolEntrypoint {
		command = otelcolCommand
	}
	commandJSON, err := json.Marshal(command)
	if err != nil {
		return fmt.Errorf("encode alloy-otel command: %w", err)
	}
	a.manifest = strings.NewReplacer(
		alloyImagePlaceholder, image,
		alloyOtelCommandPlaceholder, string(commandJSON),
	).Replace(alloyOtelManifest)
	if err := util.Step("apply alloy-otel manifest", func() error {
		return harness.ApplyManifest(a.opts.Namespace, a.manifest)
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
	if a.manifest != "" {
		_ = harness.DeleteManifest(a.opts.Namespace, a.manifest)
	}
	_ = harness.RunCommand("kubectl", "delete", "configmap", alloyOtelConfigMap,
		"--namespace", a.opts.Namespace,
		"--ignore-not-found=true",
	)
}

// createConfigMap creates the "alloy-otel-config" ConfigMap with key
// collector.yaml, and config.alloy when AlloyConfigPath is set, sourced from
// the given paths.
func (a *AlloyOtel) createConfigMap() error {
	collectorAbs, err := filepath.Abs(a.opts.CollectorConfigPath)
	if err != nil {
		return fmt.Errorf("resolve collector config path: %w", err)
	}
	args := []string{"create", "configmap", alloyOtelConfigMap,
		"--namespace", a.opts.Namespace,
		"--from-file=collector.yaml=" + collectorAbs,
	}
	if a.opts.AlloyConfigPath != "" {
		alloyAbs, err := filepath.Abs(a.opts.AlloyConfigPath)
		if err != nil {
			return fmt.Errorf("resolve alloy config path: %w", err)
		}
		args = append(args, "--from-file=config.alloy="+alloyAbs)
	}
	if err := harness.RunCommand("kubectl", args...); err != nil {
		return fmt.Errorf("create alloy-otel configmap: %w", err)
	}
	return nil
}
