package common

import (
	"context"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promopv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/grafana/alloy/internal/component"
	commonk8s "github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util/syncbuffer"
)

// SetCrdManagerFactory sets a custom crdManagerFactory on the component.
func SetCrdManagerFactory(c *Component, factory crdManagerFactory) {
	c.crdManagerFactory = factory
}

// FakeK8sFactory is a test implementation of K8sFactory that returns fake clients and caches.
// It uses FakeInformer to allow tests to simulate Kubernetes events without a real cluster.
type FakeK8sFactory struct {
	K8sClient kubernetes.Interface

	mu        sync.RWMutex
	informers map[client.Object]*FakeInformer
}

// NewFakeK8sFactory creates a new FakeK8sFactory with the given fake Kubernetes client.
func NewFakeK8sFactory(k8sClient kubernetes.Interface) *FakeK8sFactory {
	return &FakeK8sFactory{
		K8sClient: k8sClient,
		informers: make(map[client.Object]*FakeInformer),
	}
}

// New returns the fake Kubernetes client and a cache factory that creates FakeCaches.
func (f *FakeK8sFactory) New(_ commonk8s.ClientArguments, _ log.Logger) (kubernetes.Interface, CacheFactory, error) {
	cacheFactory := func(opts cache.Options) (cache.Cache, error) {
		return &FakeCache{
			factory: f,
			scheme:  opts.Scheme,
		}, nil
	}
	return f.K8sClient, cacheFactory, nil
}

// GetInformer returns the FakeInformer for the given object type, creating one if it doesn't exist.
func (f *FakeK8sFactory) GetInformer(obj client.Object) *FakeInformer {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Use a type key based on the object type
	for key, informer := range f.informers {
		if sameType(key, obj) {
			return informer
		}
	}

	// Create a new informer for this type
	informer := &FakeInformer{Synced: true}
	f.informers[obj] = informer
	return informer
}

// sameType checks if two client.Objects are of the same underlying type.
func sameType(a, b client.Object) bool {
	switch a.(type) {
	case *promopv1.ServiceMonitor:
		_, ok := b.(*promopv1.ServiceMonitor)
		return ok
	case *promopv1.PodMonitor:
		_, ok := b.(*promopv1.PodMonitor)
		return ok
	case *promopv1.Probe:
		_, ok := b.(*promopv1.Probe)
		return ok
	case *promopv1alpha1.ScrapeConfig:
		_, ok := b.(*promopv1alpha1.ScrapeConfig)
		return ok
	default:
		return false
	}
}

// FakeCache is a test implementation of cache.Cache that uses FakeInformers.
type FakeCache struct {
	factory *FakeK8sFactory
	scheme  *runtime.Scheme
}

// GetInformer implements cache.Informers.
func (c *FakeCache) GetInformer(ctx context.Context, obj client.Object, opts ...cache.InformerGetOption) (cache.Informer, error) {
	return c.factory.GetInformer(obj), nil
}

// GetInformerForKind implements cache.Informers.
func (c *FakeCache) GetInformerForKind(ctx context.Context, gvk schema.GroupVersionKind, opts ...cache.InformerGetOption) (cache.Informer, error) {
	// For simplicity, create a dummy object based on GVK
	return &FakeInformer{Synced: true}, nil
}

// RemoveInformer implements cache.Informers.
func (c *FakeCache) RemoveInformer(ctx context.Context, obj client.Object) error {
	return nil
}

// Start implements cache.Cache. It's a no-op for fake cache.
func (c *FakeCache) Start(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// WaitForCacheSync implements cache.Cache. Always returns true for fake cache.
func (c *FakeCache) WaitForCacheSync(ctx context.Context) bool {
	return true
}

// IndexField implements cache.Cache.
func (c *FakeCache) IndexField(ctx context.Context, obj client.Object, field string, extractValue client.IndexerFunc) error {
	return nil
}

// Get implements client.Reader.
func (c *FakeCache) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	return nil
}

// List implements client.Reader.
func (c *FakeCache) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	return nil
}

// FakeInformer provides fake Informer functionality for testing.
// It implements the cache.Informer interface from controller-runtime.
type FakeInformer struct {
	// Synced is returned by HasSynced to implement the Informer interface.
	Synced bool

	mu       sync.RWMutex
	handlers []toolscache.ResourceEventHandler
}

// AddEventHandler implements cache.Informer.
func (f *FakeInformer) AddEventHandler(handler toolscache.ResourceEventHandler) (toolscache.ResourceEventHandlerRegistration, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers = append(f.handlers, handler)
	return nil, nil
}

// AddEventHandlerWithResyncPeriod implements cache.Informer.
func (f *FakeInformer) AddEventHandlerWithResyncPeriod(handler toolscache.ResourceEventHandler, resyncPeriod time.Duration) (toolscache.ResourceEventHandlerRegistration, error) {
	return f.AddEventHandler(handler)
}

// RemoveEventHandler implements cache.Informer.
func (f *FakeInformer) RemoveEventHandler(handle toolscache.ResourceEventHandlerRegistration) error {
	return nil
}

// AddIndexers implements cache.Informer.
func (f *FakeInformer) AddIndexers(indexers toolscache.Indexers) error {
	return nil
}

// HasSynced implements cache.Informer.
func (f *FakeInformer) HasSynced() bool {
	return f.Synced
}

// IsStopped implements cache.Informer.
func (f *FakeInformer) IsStopped() bool {
	return false
}

// Add triggers an Add event for the given object.
func (f *FakeInformer) Add(obj any) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, h := range f.handlers {
		h.OnAdd(obj, false)
	}
}

// Update triggers an Update event for the given objects.
func (f *FakeInformer) Update(oldObj, newObj any) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, h := range f.handlers {
		h.OnUpdate(oldObj, newObj)
	}
}

// Delete triggers a Delete event for the given object.
func (f *FakeInformer) Delete(obj any) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	for _, h := range f.handlers {
		h.OnDelete(obj)
	}
}

// TestCrdManagerFactory creates crdManagers configured for testing.
// It injects a FakeK8sFactory that provides fake Kubernetes clients and caches.
type TestCrdManagerFactory struct {
	K8sClient kubernetes.Interface
	// LogBuffer is a synchronized buffer that captures log output.
	LogBuffer *syncbuffer.Buffer

	mu         sync.RWMutex
	manager    *crdManager
	k8sFactory *FakeK8sFactory
}

// New implements crdManagerFactory.
func (f *TestCrdManagerFactory) New(opts component.Options, cluster cluster.Cluster, logger log.Logger, args *operator.Arguments, kind string, ls labelstore.LabelStore) crdManagerInterface {
	m := newCrdManager(opts, cluster, logger, args, kind, ls)

	// Create and inject the FakeK8sFactory
	f.mu.Lock()
	if f.k8sFactory == nil {
		f.k8sFactory = NewFakeK8sFactory(f.K8sClient)
	}
	m.k8sFactory = f.k8sFactory
	f.manager = m
	f.mu.Unlock()

	return m
}

// readyManager returns the manager if it exists and is ready (informers started).
// Returns nil if not ready. Checks the LogBuffer for "informers started" to avoid data races.
func (f *TestCrdManagerFactory) readyManager() *crdManager {
	f.mu.RLock()
	m := f.manager
	f.mu.RUnlock()
	if m == nil {
		return nil
	}
	// Check if the manager is ready by looking for "informers started" in the log buffer
	if f.LogBuffer == nil || !strings.Contains(f.LogBuffer.String(), "informers started") {
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

// getServiceMonitorInformer returns the FakeInformer for ServiceMonitors.
func (f *TestCrdManagerFactory) getServiceMonitorInformer() *FakeInformer {
	f.mu.RLock()
	factory := f.k8sFactory
	f.mu.RUnlock()
	if factory == nil {
		return nil
	}
	return factory.GetInformer(&promopv1.ServiceMonitor{})
}

// TriggerServiceMonitorAdd triggers the add handler for a ServiceMonitor.
// Returns true if the trigger was executed, false if the manager is not ready yet.
func (f *TestCrdManagerFactory) TriggerServiceMonitorAdd(sm *promopv1.ServiceMonitor) bool {
	m := f.readyManager()
	if m == nil {
		return false
	}
	informer := f.getServiceMonitorInformer()
	if informer == nil {
		return false
	}
	informer.Add(sm)
	return true
}

// TriggerServiceMonitorUpdate triggers the update handler for a ServiceMonitor.
// Returns true if the trigger was executed, false if the manager is not ready yet.
func (f *TestCrdManagerFactory) TriggerServiceMonitorUpdate(oldSm, newSm *promopv1.ServiceMonitor) bool {
	m := f.readyManager()
	if m == nil {
		return false
	}
	informer := f.getServiceMonitorInformer()
	if informer == nil {
		return false
	}
	informer.Update(oldSm, newSm)
	return true
}

// TriggerServiceMonitorDelete triggers the delete handler for a ServiceMonitor.
// Returns true if the trigger was executed, false if the manager is not ready yet.
func (f *TestCrdManagerFactory) TriggerServiceMonitorDelete(sm *promopv1.ServiceMonitor) bool {
	m := f.readyManager()
	if m == nil {
		return false
	}
	informer := f.getServiceMonitorInformer()
	if informer == nil {
		return false
	}
	informer.Delete(sm)
	return true
}
