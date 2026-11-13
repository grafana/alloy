package cluster

import (
	"log/slog"
	"sync"
	"time"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/client_golang/prometheus"
)

type clusterState int

const (
	// stateNotReady is the initial state: the minimum cluster size has NOT been reached yet and the deadline timer
	// (if any) has NOT expired. It is only ever occupied during startup.
	stateNotReady clusterState = iota
	// stateReady is the state when the minimum cluster size was reached. There is no deadline timer running in this
	// state. This state is terminal: if the cluster later shrinks below the minimum size, it stays ready.
	stateReady
	// stateDeadlinePassed is the state when the deadline timer expired before the minimum cluster size was reached.
	// If the minimum size is reached later, the state moves to stateReady.
	stateDeadlinePassed
)

// Cluster is a read-only view of a cluster.
type Cluster interface {
	// Lookup determines the set of replicationFactor owners for a given key.
	// peer.Peer.Self can be used to determine if the local node is the owner,
	// allowing for short-circuiting logic to connect directly to the local node
	// instead of using the network.
	//
	// Callers can use github.com/grafana/ckit/shard.StringKey or
	// shard.NewKeyBuilder to create a key.
	//
	// An error will be returned if the type of eligible peers for the provided
	// op is less than numOwners.
	//
	// NOTE: If the cluster is not ready to accept traffic as designated by Ready, the local node should not accept
	// traffic to prevent overload. Always use Ready to verify before assigning work to instance.
	Lookup(key shard.Key, replicationFactor int, op shard.Op) ([]peer.Peer, error)

	// Peers returns the current set of peers for a Node.
	//
	// NOTE: If the cluster is not ready to accept traffic as designated by Ready, the local node should not accept
	// traffic to prevent overload. Always use Ready to verify before assigning work to instance.
	Peers() []peer.Peer

	// Ready returns true if the cluster is ready to accept traffic; otherwise, false. The cluster is ready to accept
	// traffic when:
	// - there is no minimum size requirement specified
	// - there is a minimum size requirement and the cluster size has reached that size at least once
	// - there is a minimum size requirement and cluster size is too small, but the configured wait deadline has passed.
	//
	// The minimum size requirement only gates startup. Once the cluster becomes ready, it stays ready even if the
	// number of peers later drops below the minimum size, because refusing traffic then would turn a degraded
	// cluster into a full outage.
	Ready() bool

	// Enabled returns true if clustering is enabled for this instance of Alloy as a whole, as set
	// by the --cluster.enabled flag.
	Enabled() bool
}

// alloyCluster implements the Cluster interface and manages the admission control logic.
type alloyCluster struct {
	log     *slog.Logger
	sharder shard.Sharder
	opts    Options

	clusterChangeCallback func()
	clusterReadyGauge     prometheus.Gauge

	rwMutex       sync.RWMutex
	deadlineTimer *time.Timer
	clusterState  clusterState
}

var _ Cluster = (*alloyCluster)(nil)

func newAlloyCluster(sharder shard.Sharder, clusterChangeCallback func(), opts Options, log *slog.Logger) *alloyCluster {
	c := &alloyCluster{
		log:                   log,
		sharder:               sharder,
		opts:                  opts,
		clusterChangeCallback: clusterChangeCallback,
	}

	c.clusterReadyGauge = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cluster_ready_for_traffic",
		Help: "Reports 1 when the cluster is ready to admit traffic, 0 otherwise.",
		ConstLabels: prometheus.Labels{
			"cluster_name": opts.ClusterName,
		},
	})

	minClusterSizeGauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "cluster_minimum_size",
		Help: "The configured minimum cluster size required before admitting traffic to components that use clustering.",
		ConstLabels: prometheus.Labels{
			"cluster_name": opts.ClusterName,
		},
	})

	// Register metrics if clustering is enabled and metrics are provided
	if opts.EnableClustering && opts.Metrics != nil {
		if err := opts.Metrics.Register(minClusterSizeGauge); err != nil {
			log.Warn("failed to register minimum cluster size metric", "err", err)
		} else {
			// Set the gauge to the configured minimum cluster size
			minClusterSizeGauge.Set(float64(opts.MinimumClusterSize))
		}

		// Register the cluster ready gauge that was created above
		if err := opts.Metrics.Register(c.clusterReadyGauge); err != nil {
			log.Warn("failed to register cluster ready metric", "err", err)
		}
	}

	// For consistency, set cluster to always ready when clustering is disabled or no minimum size is set.
	if !c.opts.EnableClustering || c.opts.MinimumClusterSize == 0 {
		c.clusterState = stateReady
		c.clusterReadyGauge.Set(1)
		return c
	}

	// Wait for the minimum cluster size to be reached.
	c.clusterState = stateNotReady
	c.clusterReadyGauge.Set(0)
	if opts.MinimumSizeWaitTimeout != 0 {
		// Start the deadline timer if the minimum size wait timeout is set
		c.deadlineTimer = time.AfterFunc(c.opts.MinimumSizeWaitTimeout, func() {
			c.rwMutex.Lock()
			defer c.rwMutex.Unlock()
			c.transitionToStateDeadlinePassed()
		})
	}
	return c
}

func (c *alloyCluster) Lookup(key shard.Key, replicationFactor int, op shard.Op) ([]peer.Peer, error) {
	return c.sharder.Lookup(key, replicationFactor, op)
}

func (c *alloyCluster) Peers() []peer.Peer {
	return c.sharder.Peers()
}

func (c *alloyCluster) Enabled() bool {
	return c.opts.EnableClustering
}

func (c *alloyCluster) Ready() bool {
	// Lock-free path: if clustering is disabled or no minimum size is set, the cluster is always ready.
	if !c.opts.EnableClustering || c.opts.MinimumClusterSize == 0 {
		return true
	}

	c.rwMutex.RLock()
	defer c.rwMutex.RUnlock()
	return c.clusterState == stateReady || c.clusterState == stateDeadlinePassed
}

func (c *alloyCluster) updateReadyState() {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	// Once the cluster has reached the minimum size, it stays ready. The minimum size is only meant to protect the
	// instances during startup, when a single instance could otherwise be asked to handle all the traffic. If the
	// cluster shrinks later, it is safer to keep serving with the remaining instances than to stop serving entirely.
	if c.clusterState == stateReady {
		return
	}

	// Number of peers is greater or equal the minimum cluster size = update to ready to admit traffic. This also
	// applies when the deadline has already passed.
	if len(c.sharder.Peers()) >= c.opts.MinimumClusterSize {
		c.transitionToStateReady()
	}

	// Otherwise the cluster is either still waiting (stateNotReady) or the deadline has passed (stateDeadlinePassed),
	// in which case it remains ready to admit traffic. Nothing to change in both cases.
}

// transitionToStateReady is called when the minimum cluster size is reached. rwMutex must be locked for writes by the caller.
func (c *alloyCluster) transitionToStateReady() {
	if c.clusterState == stateReady {
		return
	}
	c.clusterState = stateReady
	c.clusterReadyGauge.Set(1)

	// Stop the deadline timer if it was running
	if c.deadlineTimer != nil {
		c.deadlineTimer.Stop()
		c.deadlineTimer = nil
	}
	c.log.Info(
		"minimum cluster size reached, marking cluster as ready to admit traffic",
		"minimum_cluster_size", c.opts.MinimumClusterSize,
		"peers_count", len(c.sharder.Peers()),
	)
}

// transitionToStateDeadlinePassed is called when the deadline timer expires. rwMutex must be locked for writes by the caller.
func (c *alloyCluster) transitionToStateDeadlinePassed() {
	if c.clusterState == stateReady {
		c.log.Info(
			"minimum cluster size deadline passed, but cluster is already ready to admit traffic - ignoring",
			"minimum_cluster_size", c.opts.MinimumClusterSize,
			"minimum_size_wait_timeout", c.opts.MinimumSizeWaitTimeout,
			"peers_count", len(c.sharder.Peers()),
		)
		return
	}
	if c.clusterState == stateDeadlinePassed {
		return
	}
	c.clusterState = stateDeadlinePassed
	c.clusterReadyGauge.Set(1)

	c.log.Warn(
		"deadline passed, marking cluster as ready to admit traffic",
		"minimum_cluster_size", c.opts.MinimumClusterSize,
		"minimum_size_wait_timeout", c.opts.MinimumSizeWaitTimeout,
		"peers_count", len(c.sharder.Peers()),
	)
	// The timer must trigger notification of all components as we may have no changes to peers, but there is
	// a change to cluster readiness that components need to handle.
	c.clusterChangeCallback()
}

func (c *alloyCluster) shutdown() {
	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()
	if c.deadlineTimer != nil {
		c.deadlineTimer.Stop()
	}
}
