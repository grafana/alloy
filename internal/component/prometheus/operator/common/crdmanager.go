package common

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	promk8s "github.com/prometheus/prometheus/discovery/kubernetes"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/grafana/dskit/backoff"
	promopv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	promopv1alpha1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1alpha1"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/config"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/scrape"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	toolscache "k8s.io/client-go/tools/cache"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/grafana/alloy/internal/component"
	commonk8s "github.com/grafana/alloy/internal/component/common/kubernetes"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/component/prometheus/operator"
	"github.com/grafana/alloy/internal/component/prometheus/operator/configgen"
	compscrape "github.com/grafana/alloy/internal/component/prometheus/scrape"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/http"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/util"
)

type crdManagerInterface interface {
	Run(ctx context.Context) error
	ClusteringUpdated()
	DebugInfo() any
	GetScrapeConfig(ns, name string) []*config.ScrapeConfig
}

type crdManagerFactory interface {
	New(opts component.Options, cluster cluster.Cluster, logger log.Logger, args *operator.Arguments, kind string, ls labelstore.LabelStore) crdManagerInterface
}

type realCrdManagerFactory struct{}

func (realCrdManagerFactory) New(opts component.Options, cluster cluster.Cluster, logger log.Logger, args *operator.Arguments, kind string, ls labelstore.LabelStore) crdManagerInterface {
	return newCrdManager(opts, cluster, logger, args, kind, ls)
}

// CacheFactory creates controller-runtime caches with the given options.
// This is returned by K8sFactory.New and can be called multiple times (e.g., once per namespace).
type CacheFactory func(opts cache.Options) (cache.Cache, error)

// K8sFactory creates Kubernetes clients and cache factories.
// This allows tests to inject fake implementations while production code uses real ones.
type K8sFactory interface {
	// New creates a Kubernetes client and a cache factory.
	// The cache factory can be called multiple times to create caches with different options.
	New(clientConfig commonk8s.ClientArguments, logger log.Logger) (kubernetes.Interface, CacheFactory, error)
}

// realK8sFactory is the production implementation that creates real Kubernetes clients and caches.
type realK8sFactory struct{}

func (realK8sFactory) New(clientConfig commonk8s.ClientArguments, logger log.Logger) (kubernetes.Interface, CacheFactory, error) {
	restConfig, err := clientConfig.BuildRESTConfig(logger)
	if err != nil {
		return nil, nil, fmt.Errorf("creating rest config: %w", err)
	}

	k8sClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("creating kubernetes client: %w", err)
	}

	cacheFactory := func(opts cache.Options) (cache.Cache, error) {
		return cache.New(restConfig, opts)
	}

	return k8sClient, cacheFactory, nil
}

// defaultK8sFactory is the production K8sFactory used when none is injected.
var defaultK8sFactory K8sFactory = realK8sFactory{}

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

	client     kubernetes.Interface
	k8sFactory K8sFactory

	kind string

	// localZone is the availability zone of the local node, used for zone-aware
	// clustering. It is determined at startup by reading the topology zone
	// label from the Kubernetes node object.
	localZone string

	// localZoneRing is a consistent hash ring containing only the peers that
	// are in the same availability zone as the local node. When zone-aware
	// clustering is active, targets whose zone matches localZone are distributed
	// using this ring instead of the global cluster ring, ensuring that only
	// same-zone peers can own same-zone targets.
	localZoneRing shard.Sharder

	// localZonePeerNames stores the names of peers in the local zone ring,
	// for debug info purposes.
	localZonePeerNames []string

	// nodeToZone maps Kubernetes node names to their availability zones.
	// Built from the topology.kubernetes.io/zone label on each node.
	// Used for zone detection via __meta_kubernetes_pod_node_name on targets.
	nodeToZone map[string]string

	// lastFilterStats holds the most recent target filtering statistics,
	// updated after each call to filterTargets.
	lastFilterStats filterStats
}

// filterStats tracks statistics from a filterTargets call for debug info.
type filterStats struct {
	total    int // total targets seen
	sameZone int // targets in the same zone as this node
	diffZone int // targets in a different zone (dropped)
	noZone   int // targets with no zone info
	kept     int // targets kept (assigned to this node)
	dropped  int // targets dropped (assigned to another node or cross-zone)
}

const (
	KindPodMonitor     string = "podMonitor"
	KindServiceMonitor string = "serviceMonitor"
	KindProbe          string = "probe"
	KindScrapeConfig   string = "scrapeConfig"

	// nodeNameEnvVar is the environment variable used to determine the
	// Kubernetes node name this instance is running on. This is set by the
	// default Alloy Helm chart via the Kubernetes downward API.
	nodeNameEnvVar = "K8S_NODE_NAME"

	// topologyZoneLabel is the well-known Kubernetes node label for
	// availability zone information.
	topologyZoneLabel = "topology.kubernetes.io/zone"

	// metaPodNodeName is the Kubernetes SD meta label for the node name of the
	// pod backing a target. It is available on all roles that resolve pods
	// (endpoint, endpointslice, pod) and does not require attach_metadata.node.
	metaPodNodeName = "__meta_kubernetes_pod_node_name"
)

func newCrdManager(opts component.Options, cluster cluster.Cluster, logger log.Logger, args *operator.Arguments, kind string, ls labelstore.LabelStore) *crdManager {
	switch kind {
	case KindPodMonitor, KindServiceMonitor, KindProbe, KindScrapeConfig:
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
		k8sFactory:        defaultK8sFactory,
	}
}

func (c *crdManager) Run(ctx context.Context) error {
	// Create Kubernetes client and cache factory
	var err error
	var cacheFactory CacheFactory
	c.client, cacheFactory, err = c.k8sFactory.New(c.args.Client, c.logger)
	if err != nil {
		return fmt.Errorf("creating kubernetes client and cache factory: %w", err)
	}

	unregisterer := util.WrapWithUnregisterer(c.opts.Registerer)
	defer unregisterer.UnregisterAll()

	sdMetrics, err := discovery.CreateAndRegisterSDMetrics(unregisterer)
	if err != nil {
		return fmt.Errorf("creating and registering service discovery metrics: %w", err)
	}

	// Start prometheus service discovery manager
	c.discoveryManager = discovery.NewManager(ctx, slog.New(logging.NewSlogGoKitHandler(c.logger)), unregisterer, sdMetrics, discovery.Name(c.opts.ID))
	go func() {
		err := c.discoveryManager.Run()
		if err != nil {
			level.Error(c.logger).Log("msg", "discovery manager stopped", "err", err)
		}
	}()

	// Start prometheus scrape manager.
	alloyAppendable := prometheus.NewFanout(c.args.ForwardTo, c.opts.ID, c.opts.Registerer, c.ls)
	defer alloyAppendable.Clear()

	// TODO: Expose EnableCreatedTimestampZeroIngestion: https://github.com/grafana/alloy/issues/4045
	scrapeOpts := &scrape.Options{
		AppendMetadata:          c.args.Scrape.HonorMetadata,
		PassMetadataInContext:   c.args.Scrape.HonorMetadata,
		EnableTypeAndUnitLabels: c.args.Scrape.EnableTypeAndUnitLabels,
	}
	c.scrapeManager, err = scrape.NewManager(scrapeOpts, slog.New(logging.NewSlogGoKitHandler(c.logger)), nil, alloyAppendable, unregisterer)
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
	if err := c.runInformers(cacheFactory, ctx); err != nil {
		return err
	}
	level.Info(c.logger).Log("msg", "informers started")

	// Determine local zone for zone-aware clustering if enabled.
	if c.args.Clustering.Enabled && c.args.Clustering.ZoneAware {
		c.localZone = c.lookupLocalZone(ctx)
		if c.localZone != "" {
			c.rebuildLocalZoneRing(ctx)
		} else {
			level.Warn(c.logger).Log("msg", "zone_aware clustering is enabled but local zone could not be determined; all targets will use the global hash ring")
		}
	}

	var cachedTargets map[string][]*targetgroup.Group
	// Start the target discovery loop to update the scrape manager with new targets.
	for {
		select {
		case <-ctx.Done():
			return nil
		case m := <-c.discoveryManager.SyncCh():
			cachedTargets = m
			if c.args.Clustering.Enabled {
				// Refresh the nodeToZone map on every sync to catch new Kubernetes nodes
				// that may have been added to the cluster. This ensures targets on newly
				// added nodes are correctly routed via the local zone ring.
				if c.localZone != "" {
					c.nodeToZone = c.buildNodeToZoneMap(ctx)
				}
				var stats filterStats
				m, stats = filterTargets(m, c.cluster, c.localZone, c.localZoneRing, c.nodeToZone)
				c.mut.Lock()
				c.lastFilterStats = stats
				c.mut.Unlock()
				level.Debug(c.logger).Log("msg", "target filtering complete",
					"total", stats.total, "kept", stats.kept, "dropped", stats.dropped,
					"same_zone", stats.sameZone, "diff_zone", stats.diffZone, "no_zone", stats.noZone,
					"local_zone", c.localZone, "zone_aware", c.args.Clustering.ZoneAware)
			}
			targetSetsChan <- m
		case <-c.clusteringUpdated:
			// if clustering updates while running, just re-filter the targets and pass them
			// into scrape manager again, instead of reloading everything
			if c.localZone != "" {
				c.rebuildLocalZoneRing(ctx)
			}
			filtered, stats := filterTargets(cachedTargets, c.cluster, c.localZone, c.localZoneRing, c.nodeToZone)
			c.mut.Lock()
			c.lastFilterStats = stats
			c.mut.Unlock()
			level.Info(c.logger).Log("msg", "target filtering complete (cluster change)",
				"total", stats.total, "kept", stats.kept, "dropped", stats.dropped,
				"same_zone", stats.sameZone, "diff_zone", stats.diffZone, "no_zone", stats.noZone,
				"local_zone", c.localZone, "zone_aware", c.args.Clustering.ZoneAware)
			targetSetsChan <- filtered
		}
	}
}

func (c *crdManager) ClusteringUpdated() {
	select {
	case c.clusteringUpdated <- struct{}{}:
	default:
	}
}

// lookupLocalZone determines the availability zone of the local Kubernetes node
// by reading the topology.kubernetes.io/zone label from the node object.
// It requires the K8S_NODE_NAME environment variable to be set (typically via
// the Kubernetes downward API). Returns an empty string if the zone cannot be
// determined.
func (c *crdManager) lookupLocalZone(ctx context.Context) string {
	nodeName := os.Getenv(nodeNameEnvVar)
	if nodeName == "" {
		level.Warn(c.logger).Log("msg", "zone_aware clustering enabled but K8S_NODE_NAME environment variable is not set; set it via the Kubernetes downward API to enable zone-aware target filtering")
		return ""
	}

	node, err := c.client.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		level.Warn(c.logger).Log("msg", "zone_aware clustering enabled but failed to look up local node; zone-aware filtering will be disabled", "node", nodeName, "err", err)
		return ""
	}

	zone, ok := node.Labels[topologyZoneLabel]
	if !ok || zone == "" {
		level.Warn(c.logger).Log("msg", "zone_aware clustering enabled but local node does not have a topology zone label", "node", nodeName, "label", topologyZoneLabel)
		return ""
	}

	level.Info(c.logger).Log("msg", "zone_aware clustering enabled, local availability zone determined", "node", nodeName, "zone", zone)
	return zone
}

// rebuildLocalZoneRing builds a consistent hash ring containing only the cluster
// peers that are in the same availability zone as the local node. It queries
// the Kubernetes API to determine each peer's zone by mapping peer IP -> Pod ->
// Node -> topology zone label. It also refreshes the nodeToZone map used for
// target zone lookups.
func (c *crdManager) rebuildLocalZoneRing(ctx context.Context) {
	// Refresh the nodeToZone map — used both for peer zone resolution and for
	// target zone detection via __meta_kubernetes_pod_node_name.
	c.nodeToZone = c.buildNodeToZoneMap(ctx)

	peers := c.cluster.Peers()
	peerZones := c.lookupPeerZones(ctx, peers)

	// Build a sub-ring with only the peers in the local zone.
	var localZonePeers []peer.Peer
	var peerNames []string
	for _, p := range peers {
		zone, ok := peerZones[p.Name]
		if ok && zone == c.localZone {
			localZonePeers = append(localZonePeers, p)
			peerNames = append(peerNames, p.Name)
		} else if p.Self {
			// Always include self even if lookup failed.
			localZonePeers = append(localZonePeers, p)
			peerNames = append(peerNames, p.Name)
		}
	}

	ring := shard.Ring(512)
	ring.SetPeers(localZonePeers)
	c.mut.Lock()
	c.localZoneRing = ring
	c.localZonePeerNames = peerNames
	c.mut.Unlock()

	level.Info(c.logger).Log("msg", "rebuilt local zone ring", "local_zone", c.localZone, "total_peers", len(peers), "local_zone_peers", len(localZonePeers), "local_zone_peer_names", strings.Join(peerNames, ","), "nodes_with_zone", len(c.nodeToZone))
}

// buildNodeToZoneMap lists all Kubernetes nodes and builds a map of node name
// to availability zone from the topology.kubernetes.io/zone label.
func (c *crdManager) buildNodeToZoneMap(ctx context.Context) map[string]string {
	nodeList, err := c.client.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		level.Warn(c.logger).Log("msg", "failed to list nodes for zone-aware clustering", "err", err)
		return nil
	}
	nodeToZone := make(map[string]string, len(nodeList.Items))
	for _, node := range nodeList.Items {
		if zone, ok := node.Labels[topologyZoneLabel]; ok && zone != "" {
			nodeToZone[node.Name] = zone
		}
	}
	return nodeToZone
}

// lookupPeerZones determines the availability zone for each peer by querying the
// Kubernetes API. It maps peer address (IP) -> Pod -> Node -> zone label.
// Returns a map of peer name -> zone. Peers whose zone cannot be determined are
// omitted from the map. Uses c.nodeToZone which must be populated before calling.
func (c *crdManager) lookupPeerZones(ctx context.Context, peers []peer.Peer) map[string]string {
	if c.nodeToZone == nil {
		return nil
	}

	// For each peer, find the pod by IP and resolve its node's zone.
	peerZones := make(map[string]string, len(peers))
	for _, p := range peers {
		// Extract IP from peer address (host:port).
		peerIP, _, err := net.SplitHostPort(p.Addr)
		if err != nil {
			level.Warn(c.logger).Log("msg", "failed to parse peer address", "peer", p.Name, "addr", p.Addr, "err", err)
			continue
		}

		podList, err := c.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
			FieldSelector: "status.podIP=" + peerIP,
		})
		if err != nil {
			level.Warn(c.logger).Log("msg", "failed to list pods for peer IP", "peer", p.Name, "ip", peerIP, "err", err)
			continue
		}
		if len(podList.Items) == 0 {
			level.Warn(c.logger).Log("msg", "no pod found for peer IP", "peer", p.Name, "ip", peerIP)
			continue
		}

		nodeName := podList.Items[0].Spec.NodeName
		if zone, ok := c.nodeToZone[nodeName]; ok {
			peerZones[p.Name] = zone
			level.Debug(c.logger).Log("msg", "resolved peer zone", "peer", p.Name, "ip", peerIP, "node", nodeName, "zone", zone)
		} else {
			level.Warn(c.logger).Log("msg", "peer node has no topology zone label", "peer", p.Name, "ip", peerIP, "node", nodeName)
		}
	}

	return peerZones
}

// resolveTargetZone determines the availability zone of a target by looking up
// __meta_kubernetes_pod_node_name in the nodeToZone map. The pod node name is
// available at the target level for endpoint and endpointslice roles, and at
// the group level for the pod role. Returns an empty string if the zone cannot
// be determined.
func resolveTargetZone(target, groupLabels model.LabelSet, nodeToZone map[string]string) string {
	if nodeToZone == nil {
		return ""
	}
	if nodeName := string(target[metaPodNodeName]); nodeName != "" {
		if zone, ok := nodeToZone[nodeName]; ok {
			return zone
		}
	}
	if nodeName := string(groupLabels[metaPodNodeName]); nodeName != "" {
		if zone, ok := nodeToZone[nodeName]; ok {
			return zone
		}
	}
	return ""
}

// TODO: merge this code with the code in prometheus.scrape. This is a copy of that code, mostly because
// we operate on slightly different data structures.
func filterTargets(m map[string][]*targetgroup.Group, c cluster.Cluster, localZone string, localZoneRing shard.Sharder, nodeToZone map[string]string) (map[string][]*targetgroup.Group, filterStats) {
	var stats filterStats
	if !c.Ready() { // if cluster not ready, we don't take any traffic locally
		return make(map[string][]*targetgroup.Group), stats
	}
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
				stats.total++
				targetKey := shard.StringKey(nonMetaLabelString(t))

				// When zone-aware clustering is active, targets in the local zone
				// are distributed using the local-zone-only ring so that only
				// same-zone peers can own them. Targets in a different zone are
				// skipped (peers in that zone will handle them via their own
				// local ring). Targets with unknown zone use the global ring.
				if localZone != "" && localZoneRing != nil {
					tZone := resolveTargetZone(t, group.Labels, nodeToZone)
					if tZone != "" && tZone != localZone {
						// Target is in a different zone; skip it. The peers
						// in that zone will pick it up using their own ring.
						stats.diffZone++
						stats.dropped++
						continue
					}
					if tZone == localZone {
						// Use local zone ring for targets in the same zone.
						peers, err := localZoneRing.Lookup(targetKey, 1, shard.OpReadWrite)
						if len(peers) == 0 || err != nil || peers[0].Self {
							g2.Targets = append(g2.Targets, t)
							stats.kept++
						} else {
							stats.dropped++
						}
						stats.sameZone++
						continue
					}
					// Target zone is unknown; fall through to global ring.
					stats.noZone++
				}

				// Use global ring for targets with unknown zone or when zone-aware clustering is disabled.
				peers, err := c.Lookup(targetKey, 1, shard.OpReadWrite)
				if len(peers) == 0 || err != nil || peers[0].Self {
					g2.Targets = append(g2.Targets, t)
					stats.kept++
				} else {
					stats.dropped++
				}
			}
			m2[k][i] = g2
		}
	}
	return m2, stats
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
func (c *crdManager) DebugInfo() any {
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

	// Populate clustering debug info if clustering is enabled.
	if c.args.Clustering.Enabled {
		totalPeers := 0
		if c.cluster != nil {
			totalPeers = len(c.cluster.Peers())
		}
		clusterInfo := &operator.ClusteringDebugInfo{
			Enabled:         true,
			ZoneAware:       c.args.Clustering.ZoneAware,
			LocalZone:       c.localZone,
			LocalZonePeers:  c.localZonePeerNames,
			TotalPeers:      totalPeers,
			TargetsTotal:    c.lastFilterStats.total,
			TargetsSameZone: c.lastFilterStats.sameZone,
			TargetsDiffZone: c.lastFilterStats.diffZone,
			TargetsNoZone:   c.lastFilterStats.noZone,
			TargetsKept:     c.lastFilterStats.kept,
			TargetsDropped:  c.lastFilterStats.dropped,
		}
		info.ClusteringInfo = clusterInfo
	}

	return info
}

func (c *crdManager) GetScrapeConfig(ns, name string) []*config.ScrapeConfig {
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
func (c *crdManager) runInformers(cacheFactory CacheFactory, ctx context.Context) error {
	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{
		promopv1.AddToScheme,
		promopv1alpha1.AddToScheme,
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
		informerCache, err := cacheFactory(opts)
		if err != nil {
			return err
		}

		informers := informerCache

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

func getInformer(ctx context.Context, informers cache.Informers, prototype client.Object, timeout time.Duration) (cache.Informer, error) {
	informerCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	informer, err := informers.GetInformer(informerCtx, prototype)
	if err != nil {
		if errors.Is(informerCtx.Err(), context.DeadlineExceeded) { // Check the context to prevent GetInformer returning a fake timeout
			return nil, fmt.Errorf("timeout exceeded while configuring informers. Check the connection"+
				" to the Kubernetes API is stable and that Alloy has appropriate RBAC permissions for %T", prototype)
		}

		return nil, err
	}

	return informer, err
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
	case KindScrapeConfig:
		prototype = &promopv1alpha1.ScrapeConfig{}
	default:
		return fmt.Errorf("unknown kind to configure Informers: %s", c.kind)
	}

	// On node restart, the API server is not always immediately available.
	// Retry with backoff to give time for the network to initialize.
	var informer cache.Informer
	var err error

	timeoutCtx, cancel := context.WithTimeout(ctx, c.args.InformerSyncTimeout)
	deadline, _ := timeoutCtx.Deadline()
	defer cancel()
	backoff := backoff.New(
		timeoutCtx,
		backoff.Config{
			MinBackoff: 1 * time.Second,
			MaxBackoff: 10 * time.Second,
			MaxRetries: 0, // Will retry until InformerSyncTimeout is reached
		},
	)
	for {
		// Retry to get the informer in case of a timeout.
		informer, err = getInformer(ctx, informers, prototype, c.args.InformerSyncTimeout)
		nextDelay := backoff.NextDelay()
		// exit loop on success, timeout, max retries reached, or if next backoff exceeds timeout
		if err == nil || !backoff.Ongoing() || time.Now().Add(nextDelay).After(deadline) {
			break
		}
		level.Warn(c.logger).Log("msg", "failed to get informer, retrying", "next backoff", nextDelay, "err", err)
		backoff.Wait()
	}
	if err != nil {
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
	case KindScrapeConfig:
		_, err = informer.AddEventHandlerWithResyncPeriod((toolscache.ResourceEventHandlerFuncs{
			AddFunc:    c.onAddScrapeConfig,
			UpdateFunc: c.onUpdateScrapeConfig,
			DeleteFunc: c.onDeleteScrapeConfig,
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

	cfg, err := config.Load("", slog.New(logging.NewSlogGoKitHandler(c.logger)))
	if err != nil {
		return fmt.Errorf("loading empty config: %w", err)
	}
	cfg.ScrapeConfigs = scs

	err = c.scrapeManager.ApplyConfig(cfg)
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

func (c *crdManager) onAddPodMonitor(obj any) {
	pm := obj.(*promopv1.PodMonitor)
	level.Info(c.logger).Log("msg", "found pod monitor", "name", pm.Name)
	c.addPodMonitor(pm)
}

func (c *crdManager) onUpdatePodMonitor(oldObj, newObj any) {
	pm := oldObj.(*promopv1.PodMonitor)
	c.clearConfigs(pm.Namespace, pm.Name)
	c.addPodMonitor(newObj.(*promopv1.PodMonitor))
}

func (c *crdManager) onDeletePodMonitor(obj any) {
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
		scrapeConfig, err = gen.GenerateServiceMonitorConfig(sm, ep, i, promk8s.Role(c.args.KubernetesRole))
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

func (c *crdManager) onAddServiceMonitor(obj any) {
	pm := obj.(*promopv1.ServiceMonitor)
	level.Info(c.logger).Log("msg", "found service monitor", "name", pm.Name)
	c.addServiceMonitor(pm)
}

func (c *crdManager) onUpdateServiceMonitor(oldObj, newObj any) {
	pm := oldObj.(*promopv1.ServiceMonitor)
	c.clearConfigs(pm.Namespace, pm.Name)
	c.addServiceMonitor(newObj.(*promopv1.ServiceMonitor))
}

func (c *crdManager) onDeleteServiceMonitor(obj any) {
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

func (c *crdManager) onAddProbe(obj any) {
	pm := obj.(*promopv1.Probe)
	level.Info(c.logger).Log("msg", "found probe", "name", pm.Name)
	c.addProbe(pm)
}

func (c *crdManager) onUpdateProbe(oldObj, newObj any) {
	pm := oldObj.(*promopv1.Probe)
	c.clearConfigs(pm.Namespace, pm.Name)
	c.addProbe(newObj.(*promopv1.Probe))
}

func (c *crdManager) onDeleteProbe(obj any) {
	pm := obj.(*promopv1.Probe)
	c.clearConfigs(pm.Namespace, pm.Name)
	if err := c.apply(); err != nil {
		level.Error(c.logger).Log("name", pm.Name, "err", err, "msg", "error applying scrape configs after deleting "+c.kind)
	}
}

func (c *crdManager) addScrapeConfig(pm *promopv1alpha1.ScrapeConfig) {
	var err error
	gen := configgen.ConfigGenerator{
		Secrets:                  configgen.NewSecretManager(c.client),
		Client:                   &c.args.Client,
		AdditionalRelabelConfigs: c.args.RelabelConfigs,
		ScrapeOptions:            c.args.Scrape,
	}
	mapKeys := []string{}
	scrapeConfigs, errs := gen.GenerateScrapeConfigConfigs(pm)
	objName := fmt.Sprintf("%s/%s", pm.Namespace, pm.Name)
	for _, err := range errs {
		level.Warn(c.logger).Log("msg", "error in scrape config", "source", objName, "err", err)
	}
	if len(errs) > 0 {
		c.addDebugInfo(pm.Namespace, pm.Name, errors.Join(errs...))
		if len(scrapeConfigs) == 0 {
			return
		}
	}
	c.mut.Lock()
	for _, scrapeConfig := range scrapeConfigs {
		mapKeys = append(mapKeys, scrapeConfig.JobName)
		c.discoveryConfigs[scrapeConfig.JobName] = scrapeConfig.ServiceDiscoveryConfigs
		c.scrapeConfigs[scrapeConfig.JobName] = scrapeConfig
	}
	c.crdsToMapKeys[objName] = mapKeys
	c.mut.Unlock()
	if err = c.apply(); err != nil {
		level.Error(c.logger).Log("name", pm.Name, "err", err, "msg", "error applying scrape configs from "+c.kind)
	}
	c.addDebugInfo(pm.Namespace, pm.Name, err)
}

func (c *crdManager) onAddScrapeConfig(obj any) {
	pm := obj.(*promopv1alpha1.ScrapeConfig)
	level.Info(c.logger).Log("msg", "found scrape config", "name", pm.Name)
	c.addScrapeConfig(pm)
}

func (c *crdManager) onUpdateScrapeConfig(oldObj, newObj any) {
	pm := oldObj.(*promopv1alpha1.ScrapeConfig)
	c.clearConfigs(pm.Namespace, pm.Name)
	c.addScrapeConfig(newObj.(*promopv1alpha1.ScrapeConfig))
}

func (c *crdManager) onDeleteScrapeConfig(obj any) {
	pm := obj.(*promopv1alpha1.ScrapeConfig)
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
