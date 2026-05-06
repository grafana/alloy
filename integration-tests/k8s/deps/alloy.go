package deps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

// alloyImageEnv is the env var the test runner sets to the Alloy image
// (in "repo:tag" form) that tests should install via the helm chart.
const alloyImageEnv = "ALLOY_TESTS_IMAGE"

type AlloyOptions struct {
	// Namespace is the namespace to install the alloy helm chart into. Required.
	// The namespace must already exist; pair this dependency with a Namespace
	// dependency listed earlier in the test setup.
	Namespace string
	// Release is the name of the helm release. Required.
	Release string
	// ConfigPath is an optional path to a .alloy config file. When set, the
	// framework creates a ConfigMap named "alloy-config" from this file and
	// configures the chart (alloy.configMap.create/name/key) to consume it.
	// When empty, no ConfigMap is created and Alloy configuration is left to
	// the chart values (e.g. an inline config in ValuesPath).
	ConfigPath string
	// ValuesPath is an optional path to a helm values file applied to the
	// Alloy chart. Use it to set chart options like controller.type, replicas,
	// stabilityLevel, or alloy.configMap.content for an inline config.
	ValuesPath string
}

type Alloy struct {
	opts      AlloyOptions
	installed bool
}

func NewAlloy(opts AlloyOptions) *Alloy {
	return &Alloy{opts: opts}
}

func (a *Alloy) Name() string {
	return "alloy"
}

func (a *Alloy) Install(ctx *harness.TestContext) error {
	if a.opts.Namespace == "" {
		return fmt.Errorf("alloy namespace is required")
	}
	if a.opts.Release == "" {
		return fmt.Errorf("alloy release name is required")
	}

	if a.opts.ConfigPath != "" {
		if err := createAlloyConfigMap(a.opts.Namespace, a.opts.ConfigPath); err != nil {
			return err
		}
	}

	image := os.Getenv(alloyImageEnv)
	imageRepo, imageTag, ok := strings.Cut(image, ":")
	if !ok {
		return fmt.Errorf("%s must be in repo:tag format (the test runner sets this), got %q", alloyImageEnv, image)
	}
	repoRoot, err := repoRootFromCwd()
	if err != nil {
		return err
	}

	args := []string{
		"helm", "upgrade", "--install",
		a.opts.Release,
		filepath.Join(repoRoot, "operations/helm/charts/alloy"),
		"--namespace", a.opts.Namespace,
		"--wait",
	}
	if a.opts.ValuesPath != "" {
		absValuesPath, valuesErr := filepath.Abs(a.opts.ValuesPath)
		if valuesErr != nil {
			return fmt.Errorf("resolve alloy values path: %w", valuesErr)
		}
		args = append(args, "--values", absValuesPath)
	}
	args = append(args,
		// Framework-managed values: keep last so they override values.yaml.
		"--set", "fullnameOverride="+a.opts.Release,
		"--set", "image.repository="+imageRepo,
		"--set", "image.tag="+imageTag,
	)
	if a.opts.ConfigPath != "" {
		// Wire the chart to the ConfigMap we just created from ConfigPath.
		args = append(args,
			"--set", "alloy.configMap.create=false",
			"--set", "alloy.configMap.name=alloy-config",
			"--set", "alloy.configMap.key=config.alloy",
		)
	}
	if err := runCommand(args[0], args[1:]...); err != nil {
		return err
	}
	a.installed = true

	ctx.AddDiagnosticHook("alloy logs", func(c context.Context) error {
		return runDiagnosticCommands(c, [][]string{
			{"kubectl", "--namespace", a.opts.Namespace, "logs", "-l", "app.kubernetes.io/name=alloy", "--all-containers=true", "--tail", "200"},
		})
	})
	return nil
}

func (a *Alloy) Cleanup() {
	if !a.installed {
		return
	}
	_ = step("uninstall alloy helm release", func() error {
		return runCommand(
			"helm", "uninstall", a.opts.Release,
			"--namespace", a.opts.Namespace,
			"--ignore-not-found",
			"--wait",
		)
	})
	if a.opts.ConfigPath != "" {
		_ = step("delete alloy configmap", func() error {
			return runCommand(
				"kubectl", "delete", "configmap", "alloy-config",
				"--namespace", a.opts.Namespace,
				"--ignore-not-found=true",
			)
		})
	}
}

// createAlloyConfigMap creates the "alloy-config" ConfigMap in the given
// namespace using the contents of the .alloy file at configPath under the
// "config.alloy" key. It expects a fresh namespace and fails if the ConfigMap
// already exists.
func createAlloyConfigMap(namespace, configPath string) error {
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve alloy config path: %w", err)
	}
	if err := runCommand("kubectl", "create", "configmap", "alloy-config",
		"--namespace", namespace,
		"--from-file=config.alloy="+absConfigPath,
	); err != nil {
		return fmt.Errorf("create alloy configmap: %w", err)
	}
	return nil
}

func repoRootFromCwd() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir, nil
		}
		next := filepath.Dir(dir)
		if next == dir {
			return "", fmt.Errorf("unable to find repo root from %s", dir)
		}
		dir = next
	}
}
