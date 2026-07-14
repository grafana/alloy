package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

// Exports holds values which are exported by all discovery components.
type Exports struct {
	Targets []Target `alloy:"targets,attr"`
}

// DiscovererConfig is an alias for Prometheus' DiscovererConfig interface, so users of this package don't need
// to import github.com/prometheus/prometheus/discover as well.
type DiscovererConfig discovery.Config

// Creator is a function provided by an implementation to create a concrete DiscovererConfig instance.
type Creator func(component.Arguments) (DiscovererConfig, error)

// ClusteredConfig is optionally implemented by a discovery component's Arguments
// to opt the component into allocator-mode clustering. When it reports true (and
// the target-allocator feature flag is on), only the elected leader runs the
// discoverer; every other node pulls its assigned slice of targets from the
// leader instead of discovering independently. Arguments that don't implement
// this interface always run the discoverer on every node (the default).
type ClusteredConfig interface {
	ClusteringEnabled() bool
}

// ClusteredRefreshInterval is how often a node re-resolves its allocator state:
// followers re-pull their assigned slice (to pick up targets the leader has newly
// discovered) and the leader re-exports its slice. Exposed as a var so tests can
// shorten it. Membership changes are handled immediately via NotifyClusterChange;
// this interval only bounds how quickly newly discovered targets propagate.
var ClusteredRefreshInterval = 15 * time.Second

// Component is a reusable component for any discovery implementation.
// it will handle dynamic updates and exporting targets appropriately for a scrape implementation.
type Component struct {
	opts component.Options

	discMut       sync.Mutex
	latestDisc    DiscovererWithMetrics
	clustered     bool // whether this component opted into allocator-mode clustering
	newDiscoverer chan struct{}

	// cluster is the cluster service (may be nil if unavailable). clusterChange is
	// signaled by NotifyClusterChange when cluster membership/leadership changes,
	// so Run can re-resolve whether this node should discover or pull.
	cluster       cluster.Cluster
	clusterChange chan struct{}

	creator            Creator
	debugDataPublisher livedebugging.DebugDataPublisher
}

var _ component.Component = (*Component)(nil)
var _ component.LiveDebugging = (*Component)(nil)
var _ cluster.Component = (*Component)(nil)

// New creates a discovery component given arguments and a concrete Discovery implementation function.
func New(o component.Options, args component.Arguments, creator Creator) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	// The cluster service is used only in allocator mode. Fetching it is
	// non-fatal: if it is unavailable (e.g. a test harness that doesn't register
	// it), the component simply runs the discoverer locally as it always has.
	var clusterSvc cluster.Cluster
	if data, err := o.GetServiceData(cluster.ServiceName); err == nil {
		clusterSvc, _ = data.(cluster.Cluster)
	}

	c := &Component{
		opts:    o,
		creator: creator,
		// buffered to avoid deadlock from the first immediate update
		newDiscoverer:      make(chan struct{}, 1),
		cluster:            clusterSvc,
		clusterChange:      make(chan struct{}, 1),
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}
	return c, c.Update(args)
}

// NotifyClusterChange implements cluster.Component. It wakes Run so it can
// re-resolve this node's allocator role and refresh its targets. It is a no-op
// for components that didn't opt into clustering.
func (c *Component) NotifyClusterChange() {
	select {
	case c.clusterChange <- struct{}{}:
	default:
	}
}

// ConvertibleConfig is used to more conveniently convert a configuration struct into a DiscovererConfig.
type ConvertibleConfig interface {
	// Convert converts the struct into a DiscovererConfig.
	Convert() DiscovererConfig
}

// NewFromConvertibleConfig creates a discovery component given a ConvertibleConfig. Convenience function for New.
func NewFromConvertibleConfig[T ConvertibleConfig](opts component.Options, conf T) (component.Component, error) {
	return New(opts, conf, func(args component.Arguments) (DiscovererConfig, error) {
		return args.(T).Convert(), nil
	})
}

// discoveryMode is the role this node plays for a clustering-enabled discovery
// component. It is re-resolved on every config or cluster change.
type discoveryMode int

const (
	// modeUnclustered: run the discoverer locally and export the full set. This is
	// the default and the only mode unless the component opted into clustering and
	// the target-allocator feature flag is on.
	modeUnclustered discoveryMode = iota
	// modeLeader: this node is the elected allocator leader. Run the discoverer,
	// register the full discovered set with the allocator, and export this node's
	// assigned slice.
	modeLeader
	// modeFollower: a non-leader in allocator mode. Do NOT run the discoverer;
	// pull this node's assigned slice from the leader and export it.
	modeFollower
)

// resolveMode determines this node's role from the clustering opt-in, the feature
// flag, and current leadership.
func (c *Component) resolveMode(clustered bool) discoveryMode {
	if !clustered || c.cluster == nil || !c.cluster.AllocatorEnabled() {
		return modeUnclustered
	}
	if c.cluster.IsAllocatorLeader() {
		return modeLeader
	}
	return modeFollower
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	// Defensive defaults for Components constructed directly in tests rather than
	// via New.
	if c.clusterChange == nil {
		c.clusterChange = make(chan struct{}, 1)
	}

	var (
		stopFn          func()
		discoveryActive bool
		stopIfRunning   = func() {
			if stopFn != nil {
				stopFn()
				stopFn = nil
			}
			discoveryActive = false
		}
	)
	defer stopIfRunning()

	// The ticker drives follower re-pulls and leader re-exports so newly
	// discovered targets propagate; it is harmless in unclustered mode.
	ticker := time.NewTicker(ClusteredRefreshInterval)
	defer ticker.Stop()

	// reconcile applies the desired mode. discChanged is true when the discoverer
	// configuration changed (via Update) and must be (re)started even if a
	// discoverer is already running.
	reconcile := func(discChanged bool) {
		c.discMut.Lock()
		disc := c.latestDisc
		clustered := c.clustered
		c.discMut.Unlock()

		mode := c.resolveMode(clustered)
		switch mode {
		case modeFollower:
			// Followers never run the discoverer; they export the leader's slice.
			stopIfRunning()
			c.publishAssigned()
		default: // modeUnclustered or modeLeader
			if discChanged || !discoveryActive {
				stopIfRunning()
				runFn, stop := c.newRunAndStopForDisc(ctx, disc, c.publishFnFor(mode))
				stopFn = stop
				discoveryActive = true
				go runFn()
			} else if mode == modeLeader {
				// Already discovering and config unchanged: a cluster change. The
				// leader re-exports its (possibly rebalanced) slice. An unclustered
				// node does nothing here — its export is driven by the discoverer.
				c.publishAssigned()
			}
		}
	}

	// No initial reconcile: Update is always called before Run (via New) and
	// signals newDiscoverer, so the first loop iteration establishes the mode.
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.newDiscoverer:
			reconcile(true)
		case <-c.clusterChange:
			reconcile(false)
		case <-ticker.C:
			reconcile(false)
		}
	}
}

// publishAssigned fetches this node's assigned slice from the allocator (locally
// on the leader, over HTTP on a follower) and exports it. On error it keeps the
// previously exported targets so a brief leader blip doesn't drop all scraping.
// It is a no-op in unclustered mode (resolveMode never routes here without a
// cluster), so it is only called when c.cluster is non-nil.
func (c *Component) publishAssigned() {
	entries, err := c.cluster.AssignedTargets(c.opts.ID)
	if err != nil {
		c.opts.Logger.Warn("failed to fetch assigned targets from allocator leader; keeping previous targets", "err", err)
		return
	}
	c.exportTargets(entriesToTargets(entries))
}

// exportTargets publishes a target set to live debugging and downstream
// components. It is the single export path for every mode.
func (c *Component) exportTargets(allTargets []Target) {
	componentID := livedebugging.ComponentID(c.opts.ID)
	c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
		componentID,
		livedebugging.Target,
		uint64(len(allTargets)),
		func() string { return fmt.Sprintf("%s", allTargets) },
	))
	c.opts.OnStateChange(Exports{Targets: allTargets})
}

// publishFnFor returns the function runDiscovery calls with the full discovered
// target set. In unclustered mode it exports the set directly; on the leader it
// registers the full set with the allocator and then exports this node's slice.
func (c *Component) publishFnFor(mode discoveryMode) func([]Target) {
	if mode == modeLeader {
		return func(all []Target) {
			c.cluster.RegisterDiscoveredTargets(c.opts.ID, targetsToEntries(all))
			c.publishAssigned()
		}
	}
	return c.exportTargets
}

// newRunAndStopForDisc creates a new runFn and stopFn functions for a given DiscovererWithMetrics. The run function will
// register the metrics and run discoverer until stopFn is called. After that it will unregister the metrics. The stop
// function will block until run function is finished cleaning up.
func (c *Component) newRunAndStopForDisc(ctx context.Context, disc DiscovererWithMetrics, publish func([]Target)) (runFn func(), stopFn func()) {
	// create new context, so we can cancel it if we get any future updates
	// since it is derived from the main run context, it only needs to be
	// canceled directly if we receive new updates
	newCtx, cancelCtx := context.WithCancel(ctx)

	doneRunning := make(chan struct{})
	runFn = func() {
		// DiscovererWithMetrics needs to have its metrics registered before running.
		err := disc.Register()
		if err != nil {
			c.opts.Logger.Warn("failed to register discoverer metrics", "err", err)
		}

		// Run the discoverer.
		c.runDiscovery(newCtx, disc, publish)

		// DiscovererWithMetrics needs to have its metrics unregistered after running.
		disc.Unregister()
		doneRunning <- struct{}{}
	}

	stopFn = func() {
		cancelCtx()
		// Wait till the runFn is done and cleaned up / unregistered the metrics.
		<-doneRunning
	}

	return runFn, stopFn
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	discConfig, err := c.creator(args)
	if err != nil {
		return err
	}
	disc, err := NewDiscovererWithMetrics(discConfig, c.opts.Registerer, c.opts.Logger)
	if err != nil {
		return err
	}

	// Re-read the clustering opt-in: it can change across Update calls.
	clustered := false
	if cc, ok := args.(ClusteredConfig); ok {
		clustered = cc.ClusteringEnabled()
	}

	c.discMut.Lock()
	c.latestDisc = disc
	c.clustered = clustered
	c.discMut.Unlock()

	select {
	case c.newDiscoverer <- struct{}{}:
	default:
	}

	return nil
}

// MaxUpdateFrequency is the minimum time to wait between updating targets.
// Prometheus uses a static threshold. Do not recommend changing this, except for tests.
var MaxUpdateFrequency = 5 * time.Second

// runDiscovery is a utility for consuming and forwarding target groups from a discoverer.
// It will handle collating targets (and clearing), as well as time based throttling of updates.
func (c *Component) runDiscovery(ctx context.Context, d DiscovererWithMetrics, publish func([]Target)) {
	// all targets we have seen so far
	cache := map[string]*targetgroup.Group{}

	ch := make(chan []*targetgroup.Group)
	runExited := make(chan struct{})
	go func() {
		d.Run(ctx, ch)
		runExited <- struct{}{}
	}()

	// function to convert and send targets in format scraper expects
	send := func() {
		publish(toAlloyTargets(cache))
	}

	ticker := time.NewTicker(MaxUpdateFrequency)
	// true if we have received new targets and need to send. Initially set it to true to send empty targets in case
	// the discoverer never sends any targets.
	haveUpdates := true
	for {
		select {
		case <-ticker.C:
			if haveUpdates {
				send()
				haveUpdates = false
			}
		case <-ctx.Done():
			// shut down the discoverer - send latest targets and wait for discoverer goroutine to exit
			send()
			<-runExited
			return
		case groups := <-ch:
			for _, group := range groups {
				// Discoverer will send an empty target set to indicate the group (keyed by Source field)
				// should be removed
				if len(group.Targets) == 0 {
					delete(cache, group.Source)
				} else {
					cache[group.Source] = group
				}
			}
			haveUpdates = true
		}
	}
}

func toAlloyTargets(cache map[string]*targetgroup.Group) []Target {
	targetsCount := 0
	for _, group := range cache {
		targetsCount += len(group.Targets)
	}
	allTargets := make([]Target, 0, targetsCount)

	for _, group := range cache {
		for _, target := range group.Targets {
			allTargets = append(allTargets, NewTargetFromSpecificAndBaseLabelSet(target, group.Labels))
		}
	}
	return allTargets
}

func (c *Component) LiveDebugging() {}
