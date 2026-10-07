package snmp

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	prom_discovery "github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/refresh"

	"github.com/grafana/alloy/internal/component"
)

type snmpDiscoveryConfig struct {
	args Arguments
	opts component.Options
}

var _ prom_discovery.Config = (*snmpDiscoveryConfig)(nil)

// Name implements discovery.DiscovererConfig.
func (k *snmpDiscoveryConfig) Name() string {
	return "snmp"
}

// NewDiscoverer implements discovery.DiscovererConfig.
func (k *snmpDiscoveryConfig) NewDiscoverer(discOpts prom_discovery.DiscovererOptions) (prom_discovery.Discoverer, error) {
	m, ok := discOpts.Metrics.(*snmpMetrics)
	if !ok {
		return nil, fmt.Errorf("invalid discovery metrics type")
	}

	snmpDiscovery, err := NewSNMPDiscovery(k.args, k.opts.Logger)
	if err != nil {
		return nil, err
	}

	interval := defaultSnmpRefreshInterval
	if k.args.Interval != 0 {
		interval = k.args.Interval
	}

	return refresh.NewDiscovery(refresh.Options{
		Logger:              k.opts.Logger,
		Mech:                "snmp",
		Interval:            interval,
		RefreshF:            snmpDiscovery.Refresh,
		MetricsInstantiator: m.refreshMetrics,
	}), nil
}

// NewDiscovererMetrics implements discovery.DiscovererConfig.
func (*snmpDiscoveryConfig) NewDiscovererMetrics(_ prometheus.Registerer, rmi prom_discovery.RefreshMetricsInstantiator) prom_discovery.DiscovererMetrics {
	return newDiscovererMetrics(rmi)
}

var _ prom_discovery.DiscovererMetrics = (*snmpMetrics)(nil)

type snmpMetrics struct {
	refreshMetrics prom_discovery.RefreshMetricsInstantiator
}

func newDiscovererMetrics(rmi prom_discovery.RefreshMetricsInstantiator) prom_discovery.DiscovererMetrics {
	m := &snmpMetrics{
		refreshMetrics: rmi,
	}
	return m
}

// Register implements discovery.DiscovererMetrics.
func (m *snmpMetrics) Register() error {
	return nil
}

// Unregister implements discovery.DiscovererMetrics.
func (m *snmpMetrics) Unregister() {}
