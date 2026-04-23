package assert

import (
	"context"
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/backendspec"
	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/kube"
)

// Re-export the shared backend specs so assertion tests have a single
// import for backend metadata.
var (
	LokiBackend  = backendspec.Loki
	MimirBackend = backendspec.Mimir
)

// StartBackendPortForward starts a kubectl port-forward to spec and returns
// a base URL to use for HTTP queries plus a cancel function.
func StartBackendPortForward(ctx context.Context, kubeconfig string, spec backendspec.Spec) (string, func(), error) {
	handle, err := kube.StartPortForwardAndWait(ctx, kube.PortForwardConfig{
		Kubeconfig:    kubeconfig,
		Namespace:     spec.Namespace,
		Service:       spec.Service,
		TargetPort:    spec.Port,
		ReadinessPath: spec.ReadinessPath,
		PollInterval:  DefaultInterval,
		ReadyTimeout:  DefaultTimeout,
	})
	if err != nil {
		return "", nil, fmt.Errorf("start %s port-forward: %w", spec.Name, err)
	}
	return handle.BaseURL, func() {
		_ = handle.Close()
	}, nil
}
