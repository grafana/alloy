package deps

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
	"github.com/grafana/alloy/integration-tests/k8s/util"
)

type AlloyOptions struct {
	// Namespace must already exist (pair with a Namespace dep earlier in the list).
	Namespace string
	// Release is the helm release name.
	Release string
	// ConfigPath is an optional .alloy file. When set, mounted via a "alloy-config"
	// ConfigMap and wired to the chart (alloy.configMap.{create,name,key}).
	ConfigPath string
	// ValuesPath is an optional helm values file applied to the Alloy chart.
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

	image := os.Getenv(harness.AlloyImageEnv)
	imageRepo, imageTag, ok := splitImageRef(image)
	if !ok {
		return fmt.Errorf("%s must be in repo:tag format (the test runner sets this), got %q", harness.AlloyImageEnv, image)
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
	if err := harness.RunCommand(args[0], args[1:]...); err != nil {
		return err
	}
	a.installed = true

	ctx.AddDiagnosticHook("alloy logs", func(c context.Context) error {
		return harness.RunDiagnosticCommands(c, [][]string{
			{"kubectl", "--namespace", a.opts.Namespace, "logs", "-l", "app.kubernetes.io/name=alloy", "--all-containers=true", "--tail", "200"},
		})
	})
	return nil
}

func (a *Alloy) Cleanup() {
	if !a.installed {
		return
	}
	_ = util.Step("uninstall alloy helm release", func() error {
		return harness.RunCommand(
			"helm", "uninstall", a.opts.Release,
			"--namespace", a.opts.Namespace,
			"--ignore-not-found",
			"--wait",
		)
	})
	if a.opts.ConfigPath != "" {
		_ = util.Step("delete alloy configmap", func() error {
			return harness.RunCommand(
				"kubectl", "delete", "configmap", "alloy-config",
				"--namespace", a.opts.Namespace,
				"--ignore-not-found=true",
			)
		})
	}
}

// createAlloyConfigMap creates the "alloy-config" ConfigMap with key
// "config.alloy" sourced from configPath. Fails if it already exists.
func createAlloyConfigMap(namespace, configPath string) error {
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve alloy config path: %w", err)
	}
	if err := harness.RunCommand("kubectl", "create", "configmap", "alloy-config",
		"--namespace", namespace,
		"--from-file=config.alloy="+absConfigPath,
	); err != nil {
		return fmt.Errorf("create alloy configmap: %w", err)
	}
	return nil
}

// splitImageRef splits "repo:tag" on the last ":" so registry-port refs like
// "localhost:5000/alloy:dev" parse correctly. ok=false on missing tag.
func splitImageRef(ref string) (repo, tag string, ok bool) {
	idx := strings.LastIndex(ref, ":")
	if idx < 0 {
		return "", "", false
	}
	// ":" before the last "/" is a registry port, not the tag separator.
	if slash := strings.LastIndex(ref, "/"); slash > idx {
		return "", "", false
	}
	repo, tag = ref[:idx], ref[idx+1:]
	if repo == "" || tag == "" {
		return "", "", false
	}
	return repo, tag, true
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
