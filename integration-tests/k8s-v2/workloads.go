package k8sv2

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const localAlloyChartPath = "operations/helm/charts/alloy"

func ensureNamespace(ctx context.Context, kubeconfigPath, namespace string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "create", "namespace", namespace)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "AlreadyExists") {
		return fmt.Errorf("create namespace %q failed: %w: %s", namespace, err, string(out))
	}
	return nil
}

func applyWorkloadManifest(ctx context.Context, kubeconfigPath, manifestPath string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", manifestPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply -f %q failed: %w: %s", manifestPath, err, string(out))
	}
	return nil
}

func deleteWorkloadManifest(ctx context.Context, kubeconfigPath, manifestPath string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "delete", "--ignore-not-found=true", "-f", manifestPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl delete -f %q failed: %w: %s", manifestPath, err, string(out))
	}
	return nil
}

func runGoTestPackage(dir, kubeconfigPath string) error {
	cmd := exec.Command(
		"go",
		"test",
		"-count=1",
		"-v",
		"-tags",
		"alloyintegrationtests k8sv2integrationtests",
		".",
		"-args",
		"-k8s.v2.kubeconfig="+kubeconfigPath,
	)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func installAlloyFromChart(ctx context.Context, kubeconfigPath, testName, valuesPath string) error {
	if _, err := os.Stat(valuesPath); err != nil {
		return fmt.Errorf("helm values for test %q are required at %q: %w", testName, valuesPath, err)
	}
	absChartPath, err := filepath.Abs(localAlloyChartPath)
	if err != nil {
		return fmt.Errorf("resolve chart path: %w", err)
	}
	absValuesPath, err := filepath.Abs(valuesPath)
	if err != nil {
		return fmt.Errorf("resolve values path: %w", err)
	}

	cmd := exec.CommandContext(
		ctx,
		"helm",
		"upgrade",
		"--install",
		alloyRelease,
		absChartPath,
		"--kubeconfig",
		kubeconfigPath,
		"--namespace",
		alloyNamespace,
		"--wait",
		"--timeout",
		readinessTimeout.String(),
		"--values",
		absValuesPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm install Alloy for %q failed: %w: %s", testName, err, string(out))
	}
	return nil
}

func uninstallAlloyFromChart(ctx context.Context, kubeconfigPath, testName string) error {
	cmd := exec.CommandContext(
		ctx,
		"helm",
		"uninstall",
		alloyRelease,
		"--kubeconfig",
		kubeconfigPath,
		"--namespace",
		alloyNamespace,
	)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "release: not found") {
		return fmt.Errorf("helm uninstall Alloy for %q failed: %w: %s", testName, err, string(out))
	}
	return nil
}
