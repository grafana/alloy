package common

import (
	"sync"

	"github.com/go-kit/log"
	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"k8s.io/client-go/kubernetes"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/labelstore"
)

// SetCrdManagerFactory sets a custom crdManagerFactory on the component.
func SetCrdManagerFactory(c *Component, factory crdManagerFactory) {
	c.crdManagerFactory = factory
}

// TestCrdManagerFactory creates crdManagers configured for testing.
// It injects a fake k8s client and disables informers (since they require a real k8s cluster).
type TestCrdManagerFactory struct {
	K8sClient kubernetes.Interface

	mu      sync.RWMutex
	manager *crdManager
}

// New implements crdManagerFactory.
func (f *TestCrdManagerFactory) New(opts component.Options, cluster cluster.Cluster, logger log.Logger, args *operator.Arguments, kind string, ls labelstore.LabelStore) crdManagerInterface {
	m := newCrdManager(opts, cluster, logger, args, kind, ls)
	m.client = f.K8sClient // Setting client skips restConfig build, which skips informers
	f.mu.Lock()
	f.manager = m
	f.mu.Unlock()
	return m
}

// readyManager returns the manager if it exists and is ready (discovery and scrape managers initialized).
// Returns nil if not ready. All access is properly synchronized.
func (f *TestCrdManagerFactory) readyManager() *crdManager {
	f.mu.RLock()
	m := f.manager
	f.mu.RUnlock()
	if m == nil {
		return nil
	}
	m.mut.Lock()
	ready := m.discoveryManager != nil && m.scrapeManager != nil
	m.mut.Unlock()
	if !ready {
		return nil
	}
	return m
}

// GetScrapeConfigJobNames returns the job names of all registered scrape configs.
func (f *TestCrdManagerFactory) GetScrapeConfigJobNames() []string {
	f.mu.RLock()
	m := f.manager
	f.mu.RUnlock()
	if m == nil {
		return nil
	}
	m.mut.Lock()
	defer m.mut.Unlock()
	var names []string
	for name := range m.scrapeConfigs {
		names = append(names, name)
	}
	return names
}

// InjectStaticTargets injects static targets for a job, replacing k8s service discovery.
// This is useful for testing since k8s SD won't work without a real cluster.
// Returns false if the manager is not ready yet.
func (f *TestCrdManagerFactory) InjectStaticTargets(jobName, targetAddr string) (bool, error) {
	m := f.readyManager()
	if m == nil {
		return false, nil
	}

	staticConfig := discovery.StaticConfig{
		&targetgroup.Group{
			Targets: []model.LabelSet{
				{model.AddressLabel: model.LabelValue(targetAddr)},
			},
		},
	}

	m.mut.Lock()
	m.discoveryConfigs[jobName] = discovery.Configs{staticConfig}

	// Update the scrape config to use static targets and clear relabeling rules
	// (since they expect k8s labels that won't be present)
	if sc, ok := m.scrapeConfigs[jobName]; ok {
		sc.ServiceDiscoveryConfigs = discovery.Configs{staticConfig}
		sc.RelabelConfigs = nil
	}
	m.mut.Unlock()

	return true, m.apply()
}

// TriggerServiceMonitorAdd triggers the add handler for a ServiceMonitor.
// Returns true if the trigger was executed, false if the manager is not ready yet.
func (f *TestCrdManagerFactory) TriggerServiceMonitorAdd(sm *promopv1.ServiceMonitor) bool {
	m := f.readyManager()
	if m == nil {
		return false
	}
	m.onAddServiceMonitor(sm)
	return true
}

// TriggerServiceMonitorUpdate triggers the update handler for a ServiceMonitor.
// Returns true if the trigger was executed, false if the manager is not ready yet.
func (f *TestCrdManagerFactory) TriggerServiceMonitorUpdate(oldSm, newSm *promopv1.ServiceMonitor) bool {
	m := f.readyManager()
	if m == nil {
		return false
	}
	m.onUpdateServiceMonitor(oldSm, newSm)
	return true
}

// TriggerServiceMonitorDelete triggers the delete handler for a ServiceMonitor.
// Returns true if the trigger was executed, false if the manager is not ready yet.
func (f *TestCrdManagerFactory) TriggerServiceMonitorDelete(sm *promopv1.ServiceMonitor) bool {
	m := f.readyManager()
	if m == nil {
		return false
	}
	m.onDeleteServiceMonitor(sm)
	return true
}
