package harness

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	managedClusterEnv = "ALLOY_K8S_MANAGED_CLUSTER"
	kubeconfigEnv     = "ALLOY_K8S_KUBECONFIG"
)

func managedClusterEnabled() bool {
	return os.Getenv(managedClusterEnv) == "1"
}

func kubeconfigFromEnv() (string, error) {
	if !managedClusterEnabled() {
		return "", fmt.Errorf("missing %s=1, run tests with make integration-test-k8s or go run ./integration-tests/k8s/runner", managedClusterEnv)
	}

	kubeconfig := os.Getenv(kubeconfigEnv)
	if kubeconfig == "" {
		return "", fmt.Errorf("missing %s, run tests with make integration-test-k8s or go run ./integration-tests/k8s/runner", kubeconfigEnv)
	}
	if !filepath.IsAbs(kubeconfig) {
		return "", fmt.Errorf("%s must be an absolute path, got %q", kubeconfigEnv, kubeconfig)
	}
	if _, err := os.Stat(kubeconfig); err != nil {
		return "", fmt.Errorf("%s %q is not accessible: %w", kubeconfigEnv, kubeconfig, err)
	}
	return kubeconfig, nil
}

func commandEnv() []string {
	env := os.Environ()
	if kubeconfig := os.Getenv(kubeconfigEnv); kubeconfig != "" {
		env = append(env, "KUBECONFIG="+kubeconfig)
	}
	return env
}

func newClient(kubeconfig string) (*kubernetes.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}
