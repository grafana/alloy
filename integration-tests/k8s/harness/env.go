package harness

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Env-var names plumbing runner config into test binaries. Single source
// of truth shared by runner, harness and deps.
const (
	ManagedClusterEnv = "ALLOY_TESTS_MANAGED_CLUSTER"
	KubeconfigEnv     = "ALLOY_TESTS_KUBECONFIG"
	KindClusterEnv    = "ALLOY_TESTS_KIND_CLUSTER"
	AlloyImageEnv     = "ALLOY_TESTS_IMAGE"
)

// KindClusterName returns the runner's kind cluster name (or "" if unset).
func KindClusterName() string {
	return os.Getenv(KindClusterEnv)
}

func managedClusterEnabled() bool {
	return os.Getenv(ManagedClusterEnv) == "1"
}

func kubeconfigFromEnv() (string, error) {
	if !managedClusterEnabled() {
		return "", fmt.Errorf("missing %s=1, run tests with make integration-test-k8s or go run ./integration-tests/k8s/runner", ManagedClusterEnv)
	}

	kubeconfig := os.Getenv(KubeconfigEnv)
	if kubeconfig == "" {
		return "", fmt.Errorf("missing %s, run tests with make integration-test-k8s or go run ./integration-tests/k8s/runner", KubeconfigEnv)
	}
	if !filepath.IsAbs(kubeconfig) {
		return "", fmt.Errorf("%s must be an absolute path, got %q", KubeconfigEnv, kubeconfig)
	}
	if _, err := os.Stat(kubeconfig); err != nil {
		return "", fmt.Errorf("%s %q is not accessible: %w", KubeconfigEnv, kubeconfig, err)
	}
	return kubeconfig, nil
}

// CommandEnv returns the process environment with KUBECONFIG pinned to the
// managed test kubeconfig. Any pre-existing KUBECONFIG is stripped first to
// avoid POSIX duplicate-key ambiguity. Use the RunCommand* helpers for
// one-shot invocations; pass this as cmd.Env for long-lived exec.Cmd.
func CommandEnv() []string {
	parent := os.Environ()
	env := make([]string, 0, len(parent)+1)
	for _, kv := range parent {
		if !strings.HasPrefix(kv, "KUBECONFIG=") {
			env = append(env, kv)
		}
	}
	if kubeconfig := os.Getenv(KubeconfigEnv); kubeconfig != "" {
		env = append(env, "KUBECONFIG="+kubeconfig)
	}
	return env
}
