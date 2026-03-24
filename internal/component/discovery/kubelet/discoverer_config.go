package kubelet

import (
	"fmt"
	"log/slog"

	"github.com/prometheus/client_golang/prometheus"
	prom_discovery "github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/refresh"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/logging"
)

type kubeletDiscoveryConfig struct {
	args Arguments
	opts component.Options
}

var _ prom_discovery.Config = (*kubeletDiscoveryConfig)(nil)

// Name implements discovery.DiscovererConfig.
func (k *kubeletDiscoveryConfig) Name() string {
	return "kubelet"
}

// NewDiscoverer implements discovery.DiscovererConfig.
func (k *kubeletDiscoveryConfig) NewDiscoverer(discOpts prom_discovery.DiscovererOptions) (prom_discovery.Discoverer, error) {
	m, ok := discOpts.Metrics.(*kubeletMetrics)
	if !ok {
		return nil, fmt.Errorf("invalid discovery metrics type")
	}

	kubeletDiscovery, err := NewKubeletDiscovery(k.args)
	if err != nil {
		return nil, err
	}

	interval := defaultKubeletRefreshInterval
	if k.args.Interval != 0 {
		interval = k.args.Interval
	}

	return refresh.NewDiscovery(refresh.Options{
		Logger:              slog.New(logging.NewSlogGoKitHandler(k.opts.Logger)),
		Mech:                "kubelet",
		Interval:            interval,
		RefreshF:            kubeletDiscovery.Refresh,
		MetricsInstantiator: m.refreshMetrics,
	}), nil
}

// NewDiscovererMetrics implements discovery.DiscovererConfig.
func (*kubeletDiscoveryConfig) NewDiscovererMetrics(_ prometheus.Registerer, rmi prom_discovery.RefreshMetricsInstantiator) prom_discovery.DiscovererMetrics {
	return newDiscovererMetrics(rmi)
}

var _ prom_discovery.DiscovererMetrics = (*kubeletMetrics)(nil)

type kubeletMetrics struct {
	refreshMetrics prom_discovery.RefreshMetricsInstantiator
}

func newDiscovererMetrics(rmi prom_discovery.RefreshMetricsInstantiator) prom_discovery.DiscovererMetrics {
	m := &kubeletMetrics{
		refreshMetrics: rmi,
	}
	return m
}

// Register implements discovery.DiscovererMetrics.
func (m *kubeletMetrics) Register() error {
	return nil
}

// Unregister implements discovery.DiscovererMetrics.
func (m *kubeletMetrics) Unregister() {}
