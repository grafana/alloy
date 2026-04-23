//go:build alloyintegrationtests && k8sv2integrationtests

package k8sv2

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/buildtag"
	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/imageutil"
	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/template"
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

// runGoTestPackage invokes `go test` on a child package (tests/<name>)
// from inside the harness's own `go test` process. This "go test inside go
// test" design is deliberate:
//
//   - Each test's assertions live in their own Go package under tests/<name>
//     so they can declare their own imports, flags, and helpers without
//     polluting the harness package.
//   - Running them as independent `go test` binaries keeps per-test
//     arguments (kubeconfig, test-id) strictly scoped and avoids cross-test
//     flag collisions while the harness runs subtests in parallel.
//   - It also yields a clean repro command: a failing subtest prints the
//     exact `go test ./tests/<name>` invocation the user can paste to rerun.
//
// The trade-off is an extra compile step per test; compile caches make this
// negligible in practice and the isolation benefit is worth it for an
// integration harness that drives real Kubernetes.
func runGoTestPackage(dir, kubeconfigPath, testID string) error {
	cmd := exec.Command(
		"go",
		"test",
		"-count=1",
		"-v",
		"-tags",
		buildtag.Tags,
		".",
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

	cmd := exec.CommandContext(
		ctx,
		"helm",
		"upgrade",
		"--install",
		release,
		absChartPath,
		"--kubeconfig",
		kubeconfigPath,
		"--namespace",
		namespace,
		"--wait",
		"--timeout",
		readinessTimeout.String(),
		"--values",
		absValuesPath,
	)
	if *alloyImageFlag != "" {
		repo, tag, err := imageutil.SplitReference(*alloyImageFlag)
		if err != nil {
			return fmt.Errorf("parse k8s.v2.alloy-image %q: %w", *alloyImageFlag, err)
		}
		cmd.Args = append(
			cmd.Args,
			"--set-string", "image.repository="+repo,
			"--set-string", "image.tag="+tag,
		)
		if *alloyPullPolicy != "" {
			cmd.Args = append(cmd.Args, "--set-string", "image.pullPolicy="+*alloyPullPolicy)
		}
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("helm install Alloy for %q failed: %w: %s", testName, err, string(out))
	}
	return nil
}

func uninstallAlloyFromChart(ctx context.Context, kubeconfigPath, testName, release, namespace string) error {
	cmd := exec.CommandContext(
		ctx,
		"helm",
		"uninstall",
		release,
		"--kubeconfig",
		kubeconfigPath,
		"--namespace",
		namespace,
	)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "release: not found") {
		return fmt.Errorf("helm uninstall Alloy for %q failed: %w: %s", testName, err, string(out))
	}
	return nil
}

func renderTemplatedFile(path string, vars map[string]string) (string, error) {
	return template.RenderFile(path, vars)
}

func removeTempFile(path string) {
	_ = os.Remove(path)
}

func resolveAlloyChartPath() (string, error) {
	candidates := []string{
		localAlloyChartPath,
		filepath.Join("..", "..", localAlloyChartPath),
	}

	var checked []string
	for _, rel := range candidates {
		abs, err := filepath.Abs(rel)
		if err != nil {
			continue
		}
		checked = append(checked, abs)
		info, err := os.Stat(abs)
		if err == nil && info.IsDir() {
			return abs, nil
		}
	}

	return "", fmt.Errorf("alloy chart not found; checked paths: %s", strings.Join(checked, ", "))
}
