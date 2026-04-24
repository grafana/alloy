package harness

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	managedClusterEnv = "ALLOY_K8S_MANAGED_CLUSTER"
)

func kubeconfigFromEnv() (string, error) {
	if os.Getenv(managedClusterEnv) != "1" {
		return "", fmt.Errorf("missing %s=1, run tests with make integration-test-k8s or integration-tests/k8s/run.sh", managedClusterEnv)
	}

	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		return "", errors.New("missing KUBECONFIG, run tests with make integration-test-k8s or integration-tests/k8s/run.sh")
	}
	if !filepath.IsAbs(kubeconfig) {
		return "", fmt.Errorf("KUBECONFIG must be an absolute path, got %q", kubeconfig)
	}
	if _, err := os.Stat(kubeconfig); err != nil {
		return "", fmt.Errorf("KUBECONFIG %q is not accessible: %w", kubeconfig, err)
	}
	return kubeconfig, nil
}

func newClient(kubeconfig string) (*kubernetes.Clientset, error) {
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}
	return kubernetes.NewForConfig(cfg)
}
