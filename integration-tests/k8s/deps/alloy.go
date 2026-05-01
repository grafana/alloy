package deps

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/grafana/alloy/integration-tests/k8s/harness"
)

type AlloyOptions struct {
	Namespace  string
	ConfigPath string
	Controller string
	Release    string
}

type Alloy struct {
	opts AlloyOptions
}

func NewAlloy(opts AlloyOptions) *Alloy {
	return &Alloy{opts: opts}
}

func (a *Alloy) Name() string {
	return "alloy"
}

func (a *Alloy) Install(ctx *harness.TestContext) error {
	namespace := a.opts.Namespace
	if namespace == "" {
		namespace = ctx.Namespace()
	}
	if namespace == "" {
		return fmt.Errorf("alloy namespace is required")
	}

	configPath := a.opts.ConfigPath
	if configPath == "" {
		return fmt.Errorf("alloy config path is required")
	}

	controller := a.opts.Controller
	if controller == "" {
		controller = "deployment"
	}
	if controller != "deployment" && controller != "daemonset" && controller != "statefulset" {
		return fmt.Errorf("invalid alloy controller type %q (expected deployment|daemonset|statefulset)", controller)
	}

	release := a.opts.Release
	if release == "" {
		release = "alloy-" + ctx.Name()
	}

	if err := runCommand("kubectl", "get", "namespace", namespace); err != nil {
		if createErr := runCommand("kubectl", "create", "namespace", namespace); createErr != nil && !strings.Contains(createErr.Error(), "already exists") {
			return fmt.Errorf("ensure namespace %q: %w", namespace, createErr)
		}
	}

	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve alloy config path: %w", err)
	}
	manifest, err := runCommandOutput(
		"kubectl",
		"create",
		"configmap",
		"alloy-config",
		"--namespace", namespace,
		"--from-file=config.alloy="+absConfigPath,
		"--dry-run=client",
		"-o", "yaml",
	)
	if err != nil {
		return err
	}
	cmd := exec.Command("kubectl", "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = commandEnv()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("apply alloy configmap: %w", err)
	}

	imageRepo, imageTag := parseAlloyImage(os.Getenv("ALLOY_IMAGE"))
	repoRoot, err := repoRootFromCwd()
	if err != nil {
		return err
	}

	args := []string{
		"helm",
		"upgrade",
		"--install",
		release,
		filepath.Join(repoRoot, "operations/helm/charts/alloy"),
		"--namespace", namespace,
		"--create-namespace",
		"--wait",
		"--set", "fullnameOverride=" + release,
		"--set", "image.repository=" + imageRepo,
		"--set", "image.tag=" + imageTag,
		"--set", "controller.type=" + controller,
		"--set", "alloy.stabilityLevel=experimental",
		"--set", "alloy.configMap.create=false",
		"--set", "alloy.configMap.name=alloy-config",
		"--set", "alloy.configMap.key=config.alloy",
	}
	if controller == "deployment" {
		args = append(args, "--set", "controller.replicas=1")
	}
	if err := runCommand(args[0], args[1:]...); err != nil {
		return err
	}

	ctx.AddDiagnosticHook("alloy logs", func(c context.Context) error {
		return runDiagnosticCommands(c, [][]string{
			{"kubectl", "--namespace", namespace, "logs", "-l", "app.kubernetes.io/name=alloy", "--all-containers=true", "--tail", "200"},
		})
	})
	return nil
}

func (a *Alloy) Cleanup() {
	// Namespace/workloads cleanup is managed by other dependencies.
}

func parseAlloyImage(image string) (string, string) {
	if image == "" {
		return "grafana/alloy", "latest"
	}
	if idx := strings.LastIndex(image, ":"); idx > 0 {
		return image[:idx], image[idx+1:]
	}
	return image, "latest"
}

func runCommandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = commandEnv()
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s %v failed: %w: %s", name, args, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
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
