//go:build alloyintegrationtests && k8sv2integrationtests

package k8sv2

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/imageutil"
)

// integrationGoTags is the build-tag set the k8s-v2 harness requires and
// also passes through to the per-test `go test` subprocess. The //go:build
// directives in source files must stay literal (Go compiler requirement),
// so keep this string in sync if the tag names change.
const integrationGoTags = "alloyintegrationtests k8sv2integrationtests"

const localAlloyChartPath = "operations/helm/charts/alloy"

func ensureNamespace(ctx context.Context, kubeconfigPath, namespace string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "create", "namespace", namespace)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "AlreadyExists") {
		return fmt.Errorf("create namespace %q failed: %w: %s", namespace, err, out)
	}
	return nil
}

func applyWorkloadManifest(ctx context.Context, kubeconfigPath, manifestPath string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", manifestPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl apply -f %q failed: %w: %s", manifestPath, err, out)
	}
	return nil
}

func deleteWorkloadManifest(ctx context.Context, kubeconfigPath, manifestPath string) error {
	cmd := exec.CommandContext(ctx, "kubectl", "--kubeconfig", kubeconfigPath, "delete", "--ignore-not-found=true", "-f", manifestPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("kubectl delete -f %q failed: %w: %s", manifestPath, err, out)
	}
	return nil
}

// runGoTestPackage invokes `go test` on a child package (tests/<name>) from
// inside the harness's own `go test` process. Each test ships its
// assertions in its own Go package so it can declare its own imports and
// flags without polluting the harness; running them as independent `go
// test` binaries keeps per-test args strictly scoped and yields a clean
// repro command on failure.
func runGoTestPackage(dir, kubeconfigPath, testID string) error {
	cmd := exec.Command(
		"go", "test", "-count=1", "-v",
		"-tags", integrationGoTags, ".",
		"-args",
		"-k8s.v2.kubeconfig="+kubeconfigPath,
		"-k8s.v2.test-id="+testID,
	)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func installAlloyFromChart(ctx context.Context, kubeconfigPath, testName, valuesPath, release, namespace string) error {
	if _, err := os.Stat(valuesPath); err != nil {
		return fmt.Errorf("helm values for test %q are required at %q: %w", testName, valuesPath, err)
	}
	absChartPath, err := resolveAlloyChartPath()
	if err != nil {
		return fmt.Errorf("resolve Alloy chart path: %w", err)
	}
	absValuesPath, err := filepath.Abs(valuesPath)
	if err != nil {
		return fmt.Errorf("resolve values path: %w", err)
	}

	args := []string{
		"upgrade", "--install", release, absChartPath,
		"--kubeconfig", kubeconfigPath,
		"--namespace", namespace,
		"--wait", "--timeout", readinessTimeout.String(),
		"--values", absValuesPath,
	}
	if *alloyImageFlag != "" {
		repo, tag, err := imageutil.SplitReference(*alloyImageFlag)
		if err != nil {
			return fmt.Errorf("parse k8s.v2.alloy-image %q: %w", *alloyImageFlag, err)
		}
		args = append(args, "--set-string", "image.repository="+repo, "--set-string", "image.tag="+tag)
		if *alloyPullPolicy != "" {
			args = append(args, "--set-string", "image.pullPolicy="+*alloyPullPolicy)
		}
	}
	cmd := exec.CommandContext(ctx, "helm", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("helm install Alloy for %q failed: %w: %s", testName, err, out)
	}
	return nil
}

func uninstallAlloyFromChart(ctx context.Context, kubeconfigPath, testName, release, namespace string) error {
	cmd := exec.CommandContext(ctx, "helm", "uninstall", release,
		"--kubeconfig", kubeconfigPath, "--namespace", namespace)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "release: not found") {
		return fmt.Errorf("helm uninstall Alloy for %q failed: %w: %s", testName, err, out)
	}
	return nil
}

// renderTemplatedFile does a single-pass ${KEY} -> value substitution and
// writes the result to a temp file. strings.NewReplacer does one
// left-to-right pass so a value that contains another placeholder is not
// recursively expanded. Unknown placeholders pass through verbatim.
func renderTemplatedFile(path string, vars map[string]string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file %q: %w", path, err)
	}

	pairs := make([]string, 0, 2*len(vars))
	for k, v := range vars {
		pairs = append(pairs, "${"+k+"}", v)
	}
	content := strings.NewReplacer(pairs...).Replace(string(raw))

	tmp, err := os.CreateTemp("", "k8s-v2-rendered-*.yaml")
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	if _, err := tmp.WriteString(content); err != nil {
		_ = tmp.Close()
		return "", fmt.Errorf("write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close temp file: %w", err)
	}
	return tmp.Name(), nil
}

func removeTempFile(path string) { _ = os.Remove(path) }

func resolveAlloyChartPath() (string, error) {
	candidates := []string{localAlloyChartPath, filepath.Join("..", "..", localAlloyChartPath)}
	var checked []string
	for _, rel := range candidates {
		abs, err := filepath.Abs(rel)
		if err != nil {
			continue
		}
		checked = append(checked, abs)
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			return abs, nil
		}
	}
	return "", fmt.Errorf("alloy chart not found; checked paths: %s", strings.Join(checked, ", "))
}
