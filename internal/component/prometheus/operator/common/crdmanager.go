package common

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/shard"
	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/scrape"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/component/prometheus/operator/configgen"
	compscrape "github.com/grafana/alloy/internal/component/prometheus/scrape"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
)

// Generous timeout period for configuring all informers
const informerSyncTimeout = 10 * time.Second

// crdManager is all of the fields required to run a crd based component.
// on update, this entire thing should be recreated and restarted
type crdManager struct {
	mut sync.Mutex

	// these maps are keyed by job name
	discoveryConfigs map[string]discovery.Configs
	scrapeConfigs    map[string]*config.ScrapeConfig

	// list of keys to the above maps for a given resource by `ns/name`
	crdsToMapKeys map[string][]string
	// debug info by `kind/ns/name`
	debugInfo map[string]*operator.DiscoveredResource

	discoveryManager  discoveryManager
	scrapeManager     scrapeManager
	clusteringUpdated chan struct{}
	ls                labelstore.LabelStore

	opts    component.Options
	logger  log.Logger
	args    *operator.Arguments
	cluster cluster.Cluster

	client *kubernetes.Clientset

	kind string
}

const (
	KindPodMonitor     string = "podMonitor"
	KindServiceMonitor string = "serviceMonitor"
	KindProbe          string = "probe"
)

func newCrdManager(opts component.Options, cluster cluster.Cluster, logger log.Logger, args *operator.Arguments, kind string, ls labelstore.LabelStore) *crdManager {
	switch kind {
	case KindPodMonitor, KindServiceMonitor, KindProbe:
	default:
		panic(fmt.Sprintf("Unknown kind for crdManager: %s", kind))
	}
	return &crdManager{
		opts:              opts,
		logger:            logger,
		args:              args,
		cluster:           cluster,
		discoveryConfigs:  map[string]discovery.Configs{},
		scrapeConfigs:     map[string]*config.ScrapeConfig{},
		crdsToMapKeys:     map[string][]string{},
		debugInfo:         map[string]*operator.DiscoveredResource{},
		kind:              kind,
		clusteringUpdated: make(chan struct{}, 1),
		ls:                ls,
	}
}

func (c *crdManager) Run(ctx context.Context) error {
	restConfig, err := c.args.Client.BuildRESTConfig(c.logger)
	if err != nil {
		return fmt.Errorf("creating rest config: %w", err)
	}
	c.client, err = kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("creating kubernetes client: %w", err)
	}

	unregisterer := util.WrapWithUnregisterer(c.opts.Registerer)
	defer unregisterer.UnregisterAll()

	sdMetrics, err := discovery.CreateAndRegisterSDMetrics(unregisterer)
	if err != nil {
		return fmt.Errorf("creating and registering service discovery metrics: %w", err)
	}

	// Start prometheus service discovery manager
	c.discoveryManager = discovery.NewManager(ctx, c.logger, unregisterer, sdMetrics, discovery.Name(c.opts.ID))
	go func() {
		err := c.discoveryManager.Run()
		if err != nil {
			level.Error(c.logger).Log("msg", "discovery manager stopped", "err", err)
		}
	}()

	// Start prometheus scrape manager.
	alloyAppendable := prometheus.NewFanout(c.args.ForwardTo, c.opts.ID, c.opts.Registerer, c.ls)
	opts := &scrape.Options{}
	c.scrapeManager, err = scrape.NewManager(opts, c.logger, alloyAppendable, unregisterer)
	if err != nil {
		return fmt.Errorf("creating scrape manager: %w", err)
	}
	defer c.scrapeManager.Stop()
	targetSetsChan := make(chan map[string][]*targetgroup.Group)
	go func() {
		err := c.scrapeManager.Run(targetSetsChan)
		level.Info(c.logger).Log("msg", "scrape manager stopped")
		if err != nil {
			level.Error(c.logger).Log("msg", "scrape manager failed", "err", err)
		}
	}()

	// run informers after everything else is running
	if err := c.runInformers(restConfig, ctx); err != nil {
		return err
	}
	level.Info(c.logger).Log("msg", "informers  started")

	var cachedTargets map[string][]*targetgroup.Group
	// Start the target discovery loop to update the scrape manager with new targets.
	for {
		select {
		case <-ctx.Done():
			return nil
		case m := <-c.discoveryManager.SyncCh():
			cachedTargets = m
			if c.args.Clustering.Enabled {
				m = filterTargets(m, c.cluster)
			}
			targetSetsChan <- m
		case <-c.clusteringUpdated:
			// if clustering updates while running, just re-filter the targets and pass them
			// into scrape manager again, instead of reloading everything
			targetSetsChan <- filterTargets(cachedTargets, c.cluster)
		}
	}
}

func (c *crdManager) ClusteringUpdated() {
	select {
	case c.clusteringUpdated <- struct{}{}:
	default:
	}
}

// TODO: merge this code with the code in prometheus.scrape. This is a copy of that code, mostly because
// we operate on slightly different data structures.
func filterTargets(m map[string][]*targetgroup.Group, c cluster.Cluster) map[string][]*targetgroup.Group {
	// the key in the map is the job name.
	// the targetGroups have zero or more targets inside them.
	// we should keep the same structure even when there are no targets in a group for this node to scrape,
	// since an empty target group tells the scrape manager to stop scraping targets that match.
	m2 := make(map[string][]*targetgroup.Group, len(m))
	for k, groups := range m {
		m2[k] = make([]*targetgroup.Group, len(groups))
		for i, group := range groups {
			g2 := &targetgroup.Group{
				Labels:  group.Labels.Clone(),
				Source:  group.Source,
				Targets: make([]model.LabelSet, 0, len(group.Targets)),
			}
			// Check the hash based on each target's labels
			// We should not need to include the group's common labels, as long
			// as each node does this consistently.
			for _, t := range group.Targets {
				peers, err := c.Lookup(shard.StringKey(nonMetaLabelString(t)), 1, shard.OpReadWrite)
				if len(peers) == 0 || err != nil {
					// If the cluster found no peers or returned an error, we fall
					// back to owning the target ourselves.
					g2.Targets = append(g2.Targets, t)
				} else if peers[0].Self {
					g2.Targets = append(g2.Targets, t)
				}
			}
			m2[k][i] = g2
		}
	}
	return m2
}

// nonMetaLabelString returns a string representation of the given label set, excluding meta labels.
func nonMetaLabelString(l model.LabelSet) string {
	lstrs := make([]string, 0, len(l))
	for l, v := range l {
		if !strings.HasPrefix(string(l), model.MetaLabelPrefix) {
			lstrs = append(lstrs, fmt.Sprintf("%s=%q", l, v))
		}
	}
	sort.Strings(lstrs)
	return fmt.Sprintf("{%s}", strings.Join(lstrs, ", "))
}

// DebugInfo returns debug information for the CRDManager.
func (c *crdManager) DebugInfo() interface{} {
	c.mut.Lock()
	defer c.mut.Unlock()

	var info operator.DebugInfo
	for _, pm := range c.debugInfo {
		info.DiscoveredCRDs = append(info.DiscoveredCRDs, pm)
	}

	// c.scrapeManager can be nil if the client failed to build.
	if c.scrapeManager != nil {
		info.Targets = compscrape.BuildTargetStatuses(c.scrapeManager.TargetsActive())
	}
	return info
}

func (c *crdManager) getScrapeConfig(ns, name string) []*config.ScrapeConfig {
	prefix := fmt.Sprintf("%s/%s/%s", c.kind, ns, name)
	matches := []*config.ScrapeConfig{}
	for k, v := range c.scrapeConfigs {
		if strings.HasPrefix(k, prefix) {
			matches = append(matches, v)
		}
	}
	return matches
}

// runInformers starts all the informers that are required to discover CRDs.
func (c *crdManager) runInformers(restConfig *rest.Config, ctx context.Context) error {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		promopv1.AddToScheme,
	} {
		if err := add(scheme); err != nil {
			return fmt.Errorf("unable to register scheme: %w", err)
		}
	}

	ls, err := c.args.LabelSelector.BuildSelector()
	if err != nil {
		return fmt.Errorf("building label selector: %w", err)
	}
	for _, ns := range c.args.Namespaces {
		// TODO: This is going down an unnecessary extra step in the cache when `c.args.Namespaces` defaults to NamespaceAll.
		// This code path should be simplified and support a scenario when len(c.args.Namespace) == 0.
		defaultNamespaces := map[string]cache.Config{}
		defaultNamespaces[ns] = cache.Config{}
		opts := cache.Options{
			Scheme:            scheme,
			DefaultNamespaces: defaultNamespaces,
		}

		if ls != labels.Nothing() {
			opts.DefaultLabelSelector = ls
		}
		cache, err := cache.New(restConfig, opts)
		if err != nil {
			return err
		}

		informers := cache

		go func() {
			err := informers.Start(ctx)
			// If the context was canceled, we don't want to log an error.
			if err != nil && ctx.Err() == nil {
				level.Error(c.logger).Log("msg", "failed to start informers", "err", err)
			}
		}()
		if !informers.WaitForCacheSync(ctx) {
			return fmt.Errorf("informer caches failed to sync")
		}
		if err := c.configureInformers(ctx, informers); err != nil {
			return fmt.Errorf("failed to configure informers: %w", err)
		}
	}

	return nil
}

// configureInformers configures the informers for the CRDManager to watch for crd changes.
func (c *crdManager) configureInformers(ctx context.Context, informers cache.Informers) error {
	var prototype client.Object
	switch c.kind {
	case KindPodMonitor:
		prototype = &promopv1.PodMonitor{}
	case KindServiceMonitor:
		prototype = &promopv1.ServiceMonitor{}
	case KindProbe:
		prototype = &promopv1.Probe{}
	default:
		return fmt.Errorf("unknown kind to configure Informers: %s", c.kind)
	}

	informerCtx, cancel := context.WithTimeout(ctx, informerSyncTimeout)
	defer cancel()

	informer, err := informers.GetInformer(informerCtx, prototype)
	if err != nil {
		if errors.Is(informerCtx.Err(), context.DeadlineExceeded) { // Check the context to prevent GetInformer returning a fake timeout
			return fmt.Errorf("timeout exceeded while configuring informers. Check the connection"+
				" to the Kubernetes API is stable and that Alloy has appropriate RBAC permissions for %v", prototype)
		}

		return err
	}
	const resync = 5 * time.Minute
	switch c.kind {
	case KindPodMonitor:
		_, err = informer.AddEventHandlerWithResyncPeriod((toolscache.ResourceEventHandlerFuncs{
			AddFunc:    c.onAddPodMonitor,
			UpdateFunc: c.onUpdatePodMonitor,
			DeleteFunc: c.onDeletePodMonitor,
		}), resync)
	case KindServiceMonitor:
		_, err = informer.AddEventHandlerWithResyncPeriod((toolscache.ResourceEventHandlerFuncs{
			AddFunc:    c.onAddServiceMonitor,
			UpdateFunc: c.onUpdateServiceMonitor,
			DeleteFunc: c.onDeleteServiceMonitor,
		}), resync)
	case KindProbe:
		_, err = informer.AddEventHandlerWithResyncPeriod((toolscache.ResourceEventHandlerFuncs{
			AddFunc:    c.onAddProbe,
			UpdateFunc: c.onUpdateProbe,
			DeleteFunc: c.onDeleteProbe,
		}), resync)
	default:
		return fmt.Errorf("unknown kind to configure Informers: %s", c.kind)
	}

	if err != nil {
		return err
	}
	return nil
}

// apply applies the current state of the Manager to the Prometheus discovery manager and scrape manager.
func (c *crdManager) apply() error {
	c.mut.Lock()
	defer c.mut.Unlock()
	err := c.discoveryManager.ApplyConfig(c.discoveryConfigs)
	if err != nil {
		level.Error(c.logger).Log("msg", "error applying discovery configs", "err", err)
		return err
	}
	scs := []*config.ScrapeConfig{}
	for _, sc := range c.scrapeConfigs {
		scs = append(scs, sc)
	}
	err = c.scrapeManager.ApplyConfig(&config.Config{
		ScrapeConfigs: scs,
	})
	if err != nil {
		level.Error(c.logger).Log("msg", "error applying scrape configs", "err", err)
		return err
	}
	level.Debug(c.logger).Log("msg", "scrape config was updated")
	return nil
}

func (c *crdManager) addDebugInfo(ns string, name string, err error) {
	c.mut.Lock()
	defer c.mut.Unlock()
	debug := &operator.DiscoveredResource{}
	debug.Namespace = ns
	debug.Name = name
	debug.LastReconcile = time.Now()
	if err != nil {
		debug.ReconcileError = err.Error()
	} else {
		debug.ReconcileError = ""
	}
	if data, err := c.opts.GetServiceData(http.ServiceName); err == nil {
		if hdata, ok := data.(http.Data); ok {
			debug.ScrapeConfigsURL = fmt.Sprintf("%s%s/scrapeConfig/%s/%s", hdata.HTTPListenAddr, hdata.HTTPPathForComponent(c.opts.ID), ns, name)
		}
	}
	prefix := fmt.Sprintf("%s/%s/%s", c.kind, ns, name)
	c.debugInfo[prefix] = debug
}

func (c *crdManager) addPodMonitor(pm *promopv1.PodMonitor) {
	var err error
	gen := configgen.ConfigGenerator{
		Secrets:                  configgen.NewSecretManager(c.client),
		Client:                   &c.args.Client,
		AdditionalRelabelConfigs: c.args.RelabelConfigs,
		ScrapeOptions:            c.args.Scrape,
	}
	mapKeys := []string{}
	for i, ep := range pm.Spec.PodMetricsEndpoints {
		var scrapeConfig *config.ScrapeConfig
		scrapeConfig, err = gen.GeneratePodMonitorConfig(pm, ep, i)
		if err != nil {
			// TODO(jcreixell): Generate Kubernetes event to inform of this error when running `kubectl get <podmonitor>`.
			level.Error(c.logger).Log("name", pm.Name, "err", err, "msg", "error generating scrapeconfig from podmonitor")
			break
		}
		mapKeys = append(mapKeys, scrapeConfig.JobName)
		c.mut.Lock()
		c.discoveryConfigs[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
		c.scrapeConfigs[scrapeConfig.JobName] = scrapeConfig
		c.mut.Unlock()
	}
	if err != nil {
		c.addDebugInfo(pm.Namespace, pm.Name, err)
		return
	}
	c.mut.Lock()
	c.crdsToMapKeys[fmt.Sprintf("%s/%s", pm.Namespace, pm.Name)] = mapKeys
	c.mut.Unlock()
	if err = c.apply(); err != nil {
		level.Error(c.logger).Log("name", pm.Name, "err", err, "msg", "error applying scrape configs from "+c.kind)
	}
	c.addDebugInfo(pm.Namespace, pm.Name, err)
}

func (c *crdManager) onAddPodMonitor(obj interface{}) {
	pm := obj.(*promopv1.PodMonitor)
	level.Info(c.logger).Log("msg", "found pod monitor", "name", pm.Name)
	c.addPodMonitor(pm)
}
func (c *crdManager) onUpdatePodMonitor(oldObj, newObj interface{}) {
	pm := oldObj.(*promopv1.PodMonitor)
	c.clearConfigs(pm.Namespace, pm.Name)
	c.addPodMonitor(newObj.(*promopv1.PodMonitor))
}

func (c *crdManager) onDeletePodMonitor(obj interface{}) {
	pm := obj.(*promopv1.PodMonitor)
	c.clearConfigs(pm.Namespace, pm.Name)
	if err := c.apply(); err != nil {
		level.Error(c.logger).Log("name", pm.Name, "err", err, "msg", "error applying scrape configs after deleting "+c.kind)
	}
}

func (c *crdManager) addServiceMonitor(sm *promopv1.ServiceMonitor) {
	var err error
	gen := configgen.ConfigGenerator{
		Secrets:                  configgen.NewSecretManager(c.client),
		Client:                   &c.args.Client,
		AdditionalRelabelConfigs: c.args.RelabelConfigs,
		ScrapeOptions:            c.args.Scrape,
	}

	mapKeys := []string{}
	for i, ep := range sm.Spec.Endpoints {
		var scrapeConfig *config.ScrapeConfig
		scrapeConfig, err = gen.GenerateServiceMonitorConfig(sm, ep, i)
		if err != nil {
			// TODO(jcreixell): Generate Kubernetes event to inform of this error when running `kubectl get <servicemonitor>`.
			level.Error(c.logger).Log("name", sm.Name, "err", err, "msg", "error generating scrapeconfig from serviceMonitor")
			break
		}
		mapKeys = append(mapKeys, scrapeConfig.JobName)
		c.mut.Lock()
		c.discoveryConfigs[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
		c.scrapeConfigs[scrapeConfig.JobName] = scrapeConfig
		c.mut.Unlock()
	}
	if err != nil {
		c.addDebugInfo(sm.Namespace, sm.Name, err)
		return
	}
	c.mut.Lock()
	c.crdsToMapKeys[fmt.Sprintf("%s/%s", sm.Namespace, sm.Name)] = mapKeys
	c.mut.Unlock()
	if err = c.apply(); err != nil {
		level.Error(c.logger).Log("name", sm.Name, "err", err, "msg", "error applying scrape configs from "+c.kind)
	}
	c.addDebugInfo(sm.Namespace, sm.Name, err)
}

func (c *crdManager) onAddServiceMonitor(obj interface{}) {
	pm := obj.(*promopv1.ServiceMonitor)
	level.Info(c.logger).Log("msg", "found service monitor", "name", pm.Name)
	c.addServiceMonitor(pm)
}
func (c *crdManager) onUpdateServiceMonitor(oldObj, newObj interface{}) {
	pm := oldObj.(*promopv1.ServiceMonitor)
	c.clearConfigs(pm.Namespace, pm.Name)
	c.addServiceMonitor(newObj.(*promopv1.ServiceMonitor))
}

func (c *crdManager) onDeleteServiceMonitor(obj interface{}) {
	pm := obj.(*promopv1.ServiceMonitor)
	c.clearConfigs(pm.Namespace, pm.Name)
	if err := c.apply(); err != nil {
		level.Error(c.logger).Log("name", pm.Name, "err", err, "msg", "error applying scrape configs after deleting "+c.kind)
	}
}

func (c *crdManager) addProbe(p *promopv1.Probe) {
	var err error
	gen := configgen.ConfigGenerator{
		Secrets:                  configgen.NewSecretManager(c.client),
		Client:                   &c.args.Client,
		AdditionalRelabelConfigs: c.args.RelabelConfigs,
		ScrapeOptions:            c.args.Scrape,
	}
	var pmc *config.ScrapeConfig
	pmc, err = gen.GenerateProbeConfig(p)
	if err != nil {
		// TODO(jcreixell): Generate Kubernetes event to inform of this error when running `kubectl get <probe>`.
		level.Error(c.logger).Log("name", p.Name, "err", err, "msg", "error generating scrapeconfig from probe")
		c.addDebugInfo(p.Namespace, p.Name, err)
		return
	}
	c.mut.Lock()
	c.discoveryConfigs[pmc.JobName] = pmc.ServiceDiscoveryConfigs
	c.scrapeConfigs[pmc.JobName] = pmc
	c.crdsToMapKeys[fmt.Sprintf("%s/%s", p.Namespace, p.Name)] = []string{pmc.JobName}
	c.mut.Unlock()

	if err = c.apply(); err != nil {
		level.Error(c.logger).Log("name", p.Name, "err", err, "msg", "error applying scrape configs from "+c.kind)
	}
	c.addDebugInfo(p.Namespace, p.Name, err)
}

func (c *crdManager) onAddProbe(obj interface{}) {
	pm := obj.(*promopv1.Probe)
	level.Info(c.logger).Log("msg", "found probe", "name", pm.Name)
	c.addProbe(pm)
}
func (c *crdManager) onUpdateProbe(oldObj, newObj interface{}) {
	pm := oldObj.(*promopv1.Probe)
	c.clearConfigs(pm.Namespace, pm.Name)
	c.addProbe(newObj.(*promopv1.Probe))
}

func (c *crdManager) onDeleteProbe(obj interface{}) {
	pm := obj.(*promopv1.Probe)
	c.clearConfigs(pm.Namespace, pm.Name)
	if err := c.apply(); err != nil {
		level.Error(c.logger).Log("name", pm.Name, "err", err, "msg", "error applying scrape configs after deleting "+c.kind)
	}
}

func (c *crdManager) clearConfigs(ns, name string) {
	c.mut.Lock()
	defer c.mut.Unlock()

	for _, k := range c.crdsToMapKeys[fmt.Sprintf("%s/%s", ns, name)] {
		delete(c.discoveryConfigs, k)
		delete(c.scrapeConfigs, k)
	}
	delete(c.debugInfo, fmt.Sprintf("%s/%s/%s", c.kind, ns, name))
}
