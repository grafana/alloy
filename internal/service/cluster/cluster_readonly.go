package cluster

import (
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

type clusterState int

const (
	// stateNotReady is the state when the minimum cluster size is NOT reached and the deadline timer is NOT expired.
	stateNotReady clusterState = iota
	// stateReady is the state when the minimum cluster size is reached. There should be no deadline timer running in this state.
	stateReady
	// stateDeadlinePassed is the state when the minimum cluster size is NOT reached and the deadline timer is expired.
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
	// - there is a minimum size requirement and the cluster size is >= that size
	// - there is a minimum size requirement and cluster size is too small, but the configured wait deadline has passed.
	Ready() bool
}

// alloyCluster implements the Cluster interface and manages the admission control logic.
type alloyCluster struct {
	log     log.Logger
	sharder shard.Sharder
	opts    Options

	clusterChangeCallback func()
	clusterReadyGauge     prometheus.Gauge

	rwMutex       sync.RWMutex
	deadlineTimer *time.Timer
	clusterState  clusterState
}

var _ Cluster = (*alloyCluster)(nil)

func newAlloyCluster(sharder shard.Sharder, clusterChangeCallback func(), opts Options, log log.Logger) *alloyCluster {
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
			level.Warn(log).Log("msg", "failed to register minimum cluster size metric", "err", err)
		} else {
			// Set the gauge to the configured minimum cluster size
			minClusterSizeGauge.Set(float64(opts.MinimumClusterSize))
		}

		// Register the cluster ready gauge that was created above
		if err := opts.Metrics.Register(c.clusterReadyGauge); err != nil {
			level.Warn(log).Log("msg", "failed to register cluster ready metric", "err", err)
		}
	}

	// For consistency, set cluster to always ready when clustering is disabled or no minimum size is set.
	if !c.opts.EnableClustering || c.opts.MinimumClusterSize == 0 {
		c.clusterState = stateReady
		c.clusterReadyGauge.Set(1)
	} else if opts.MinimumSizeWaitTimeout != 0 {
		// Start the deadline timer if the minimum size wait timeout is set
		c.deadlineTimer = time.AfterFunc(c.opts.MinimumSizeWaitTimeout, func() {
			c.rwMutex.Lock()
			defer c.rwMutex.Unlock()
			c.transitionToStateDeadlinePassed()
		})
		c.clusterReadyGauge.Set(0)
	}
	return c
}

func (c *alloyCluster) Lookup(key shard.Key, replicationFactor int, op shard.Op) ([]peer.Peer, error) {
	return c.sharder.Lookup(key, replicationFactor, op)
}

func (c *alloyCluster) Peers() []peer.Peer {
	return c.sharder.Peers()
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

	// Number of peers is greater or equal the minimum cluster size = update to ready to admit traffic.
	if len(c.sharder.Peers()) >= c.opts.MinimumClusterSize {
		c.transitionToStateReady()
		return
	}

	// The number of peers is less than the minimum and the deadline timer has expired, the cluster is ready to admit
	// traffic and should remain in this state.
	if c.clusterState == stateDeadlinePassed {
		return // Logging and callback already handled in c.transitionToStateDeadlinePassed
	}

	// The number of peers is less than the minimum, and the deadline is NOT expired = update to not ready to admit traffic
	c.transitionToStateNotReady()
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
	level.Info(c.log).Log(
		"msg", "minimum cluster size reached, marking cluster as ready to admit traffic",
		"minimum_cluster_size", c.opts.MinimumClusterSize,
		"peers_count", len(c.sharder.Peers()),
	)
}

// transitionToStateNotReady is called when the minimum cluster size is not reached. rwMutex must be locked for writes by the caller.
func (c *alloyCluster) transitionToStateNotReady() {
	if c.clusterState == stateNotReady {
		return
	}
	c.clusterState = stateNotReady
	c.clusterReadyGauge.Set(0)

	// Restart the deadline timer if it is configured and we just transitioned to not ready
	if c.opts.MinimumSizeWaitTimeout != 0 {
		if c.deadlineTimer != nil {
			c.deadlineTimer.Stop()
		}
		c.deadlineTimer = time.AfterFunc(c.opts.MinimumSizeWaitTimeout, func() {
			c.rwMutex.Lock()
			defer c.rwMutex.Unlock()
			c.transitionToStateDeadlinePassed()
		})
	}
	level.Warn(c.log).Log(
		"msg", "minimum cluster size requirements are not met - marking cluster as not ready for traffic",
		"minimum_cluster_size", c.opts.MinimumClusterSize,
		"minimum_size_wait_timeout", c.opts.MinimumSizeWaitTimeout,
		"peers_count", len(c.sharder.Peers()),
	)
}

// transitionToStateDeadlinePassed is called when the deadline timer expires. rwMutex must be locked for writes by the caller.
func (c *alloyCluster) transitionToStateDeadlinePassed() {
	if c.clusterState == stateReady {
		level.Info(c.log).Log(
			"msg", "minimum cluster size deadline passed, but cluster is already ready to admit traffic - ignoring",
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

	level.Warn(c.log).Log(
		"msg", "deadline passed, marking cluster as ready to admit traffic",
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
