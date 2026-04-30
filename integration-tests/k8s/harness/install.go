package harness

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func runCommandOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Env = os.Environ()
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%s %v failed: %w: %s", name, args, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

func installMimir(namespace string) error {
	if err := step("helm repo add grafana", func() error {
		return runCommand("helm", "repo", "add", "grafana", "https://grafana.github.io/helm-charts")
	}); err != nil {
		return err
	}
	if err := step("helm repo update", func() error {
		return runCommand("helm", "repo", "update")
	}); err != nil {
		return err
	}
	return step("install mimir", func() error {
		return runCommand(
			"helm",
			"upgrade",
			"--install",
			"mimir",
			"grafana/mimir-distributed",
			"--version", "5.8.0",
			"--namespace", namespace,
			"--wait",
		)
	})
}

func installAlloy(ctx *TestContext, configPath string) error {
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve alloy config path: %w", err)
	}

	manifest, err := runCommandOutput(
		"kubectl",
		"create",
		"configmap",
		"alloy-config",
		"--namespace", ctx.Namespace,
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
	cmd.Env = os.Environ()
	if err := step("apply alloy configmap", cmd.Run); err != nil {
		return err
	}

	repoRoot, err := repoRootFromCwd()
	if err != nil {
		return err
	}

	return step("install alloy via helm chart", func() error {
		// TODO: Add optional per-test custom values.yaml support for Alloy helm installs.
		args := []string{
			"helm",
			"upgrade",
			"--install",
			"alloy",
			filepath.Join(repoRoot, "operations/helm/charts/alloy"),
			"--namespace", ctx.Namespace,
			"--wait",
			"--set", "fullnameOverride=alloy-" + ctx.Name,
			"--set", "image.repository=" + ctx.AlloyImageRepository,
			"--set", "image.tag=" + ctx.AlloyImageTag,
			"--set", "controller.type=" + ctx.ControllerType,
			"--set", "alloy.stabilityLevel=experimental",
			"--set", "alloy.configMap.create=false",
			"--set", "alloy.configMap.name=alloy-config",
			"--set", "alloy.configMap.key=config.alloy",
		}
		if ctx.ControllerType == "deployment" {
			args = append(args, "--set", "controller.replicas=1")
		}
		return runCommand(args[0], args[1:]...)
	})
}

func applyWorkloads(path, namespace string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve workloads path: %w", err)
	}
	_ = namespace
	return step("apply workloads", func() error {
		return runCommand("kubectl", "apply", "-f", absPath)
	})
}

func DeleteAlertmanagerConfig(namespace, name string) error {
	return step("delete alertmanagerconfig "+name, func() error {
		return runCommand("kubectl", "delete", "alertmanagerconfig", name, "--namespace", namespace)
	})
}
