package assert

import (
	"context"
	"fmt"

	"github.com/grafana/alloy/integration-tests/k8s-v2/internal/kube"
)

type Backend struct {
	Name          string
	Namespace     string
	Service       string
	Port          int
	ReadinessPath string
}

var (
	LokiBackend = Backend{
		Name:          "loki",
		Namespace:     "loki",
		Service:       "loki",
		Port:          3100,
		ReadinessPath: "/ready",
	}
	MimirBackend = Backend{
		Name:          "mimir",
		Namespace:     "mimir",
		Service:       "mimir",
		Port:          9009,
		ReadinessPath: "/ready",
	}
)

func StartBackendPortForward(ctx context.Context, kubeconfig string, backend Backend) (string, func(), error) {
	handle, err := kube.StartPortForwardAndWait(ctx, kube.PortForwardConfig{
		Kubeconfig:    kubeconfig,
		Namespace:     backend.Namespace,
		Service:       backend.Service,
		TargetPort:    backend.Port,
		ReadinessPath: backend.ReadinessPath,
		PollInterval:  DefaultInterval,
		ReadyTimeout:  DefaultTimeout,
	})
	if err != nil {
		return "", nil, fmt.Errorf("start %s port-forward: %w", backend.Name, err)
	}
	return handle.BaseURL, func() {
		_ = handle.Close()
	}, nil
}
