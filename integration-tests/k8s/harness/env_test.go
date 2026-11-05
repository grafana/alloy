package harness

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPinContext(t *testing.T) {
	t.Setenv(KindClusterEnv, "alloy-k8s-integration")

	assert.Equal(t,
		[]string{"--context", "kind-alloy-k8s-integration", "get", "pods"},
		PinContext("kubectl", []string{"get", "pods"}))
	assert.Equal(t,
		[]string{"--kube-context", "kind-alloy-k8s-integration", "uninstall", "alloy"},
		PinContext("helm", []string{"uninstall", "alloy"}))
	assert.Equal(t,
		[]string{"load", "docker-image", "alloy:latest"},
		PinContext("kind", []string{"load", "docker-image", "alloy:latest"}))
}

func TestPinContextWithoutCluster(t *testing.T) {
	t.Setenv(KindClusterEnv, "")

	// "kind-" is never a real context, so kubectl fails rather than using the
	// kubeconfig's current context.
	assert.Equal(t,
		[]string{"--context", "kind-", "get", "pods"},
		PinContext("kubectl", []string{"get", "pods"}))
}
