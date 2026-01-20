package common

import (
	"context"
	"time"

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
	manager   *crdManager
	// RunStarted is closed when the manager's Run method starts. Create this before calling Update().
	RunStarted chan struct{}
}

// New implements crdManagerFactory.
func (f *TestCrdManagerFactory) New(opts component.Options, cluster cluster.Cluster, logger log.Logger, args *operator.Arguments, kind string, ls labelstore.LabelStore) crdManagerInterface {
	m := newCrdManager(opts, cluster, logger, args, kind, ls)
	m.client = f.K8sClient // Setting client skips restConfig build, which skips informers
	f.manager = m
	return &testCrdManagerWrapper{
		crdManager: m,
		runStarted: f.RunStarted,
	}
}

// testCrdManagerWrapper wraps crdManager to signal when Run initializes managers.
type testCrdManagerWrapper struct {
	*crdManager
	runStarted chan struct{}
}

func (w *testCrdManagerWrapper) Run(ctx context.Context) error {
	// Start the real Run in a goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- w.crdManager.Run(ctx)
	}()

	// Poll until managers are initialized, then signal ready
	if w.runStarted != nil {
		go func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
					if w.crdManager.discoveryManager != nil && w.crdManager.scrapeManager != nil {
						close(w.runStarted)
						return
					}
					// Brief sleep to avoid busy loop
					select {
					case <-ctx.Done():
						return
					case <-time.After(10 * time.Millisecond):
					}
				}
			}
		}()
	}

	return <-errCh
}

// GetScrapeConfigJobNames returns the job names of all registered scrape configs.
func (f *TestCrdManagerFactory) GetScrapeConfigJobNames() []string {
	if f.manager == nil {
		return nil
	}
	f.manager.mut.Lock()
	defer f.manager.mut.Unlock()
	var names []string
	for name := range f.manager.scrapeConfigs {
		names = append(names, name)
	}
	return names
}

// InjectStaticTargets injects static targets for a job, replacing k8s service discovery.
// This is useful for testing since k8s SD won't work without a real cluster.
func (f *TestCrdManagerFactory) InjectStaticTargets(jobName, targetAddr string) error {
	if f.manager == nil {
		return nil
	}

	staticConfig := discovery.StaticConfig{
		&targetgroup.Group{
			Targets: []model.LabelSet{
				{model.AddressLabel: model.LabelValue(targetAddr)},
			},
		},
	}

	f.manager.mut.Lock()
	f.manager.discoveryConfigs[jobName] = discovery.Configs{staticConfig}

	// Update the scrape config to use static targets and clear relabeling rules
	// (since they expect k8s labels that won't be present)
	if sc, ok := f.manager.scrapeConfigs[jobName]; ok {
		sc.ServiceDiscoveryConfigs = discovery.Configs{staticConfig}
		sc.RelabelConfigs = nil
	}
	f.manager.mut.Unlock()

	return f.manager.apply()
}

// TriggerServiceMonitorAdd triggers the add handler for a ServiceMonitor.
func (f *TestCrdManagerFactory) TriggerServiceMonitorAdd(sm *promopv1.ServiceMonitor) {
	if f.manager != nil {
		f.manager.onAddServiceMonitor(sm)
	}
}

// TriggerServiceMonitorUpdate triggers the update handler for a ServiceMonitor.
func (f *TestCrdManagerFactory) TriggerServiceMonitorUpdate(oldSm, newSm *promopv1.ServiceMonitor) {
	if f.manager != nil {
		f.manager.onUpdateServiceMonitor(oldSm, newSm)
	}
}

// TriggerServiceMonitorDelete triggers the delete handler for a ServiceMonitor.
func (f *TestCrdManagerFactory) TriggerServiceMonitorDelete(sm *promopv1.ServiceMonitor) {
	if f.manager != nil {
		f.manager.onDeleteServiceMonitor(sm)
	}
}
