package cluster

import (
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"golang.org/x/time/rate"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	// admissionUpdateMinInterval defines the minimum time interval between updates to the admission control state.
	admissionUpdateMinInterval = time.Second
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

	// Ready returns true if the cluster is ready to accept traffic; otherwise, false.
	Ready() bool
}

// alloyCluster implements the Cluster interface and manages the admission control logic.
type alloyCluster struct {
	log     log.Logger
	sharder shard.Sharder
	opts    Options

	clusterChangeCallback func()
	limiter               *rate.Limiter

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
		limiter:               rate.NewLimiter(rate.Every(admissionUpdateMinInterval), 1),
		clusterChangeCallback: clusterChangeCallback,
	}

	// For consistency, set cluster to always ready when clustering is disabled or no minimum size is set.
	if !c.opts.EnableClustering || c.opts.MinimumClusterSize == 0 {
		c.clusterState = stateReady
	} else if opts.MinimumSizeWaitTimeout != 0 {
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

func (c *alloyCluster) Ready() bool {
	// Lock-free path: if clustering is disabled or no minimum size is set, the cluster is always ready.
	if !c.opts.EnableClustering || c.opts.MinimumClusterSize == 0 {
		return true
	}

	// Don't update state too frequently. Use read-only lock if it's not yet time to update the ready state.
	if !c.limiter.Allow() {
		c.rwMutex.RLock()
		defer c.rwMutex.RUnlock()
		return c.clusterState == stateReady || c.clusterState == stateDeadlinePassed
	}

	c.rwMutex.Lock()
	defer c.rwMutex.Unlock()

	// Number of peers is greater or equal the minimum cluster size = ready to admit traffic.
	if len(c.sharder.Peers()) >= c.opts.MinimumClusterSize {
		c.transitionToStateReady()
		return true
	}

	// The number of peers is less than the minimum. If the deadline timer has expired, the cluster is ready to admit traffic.
	if c.clusterState == stateDeadlinePassed {
		return true // Logging and callback already handled in c.transitionToStateDeadlinePassed
	}

	// The number of peers is less than the minimum and the deadline is NOT expired = not ready to admit traffic
	c.transitionToStateNotReady()
	return false
}

// transitionToStateReady is called when the minimum cluster size is reached. rwMutex must be locked for writes by the caller.
func (c *alloyCluster) transitionToStateReady() {
	if c.clusterState == stateReady {
		return
	}
	c.clusterState = stateReady

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
	c.clusterChangeCallback()
}

// transitionToStateNotReady is called when the minimum cluster size is not reached. rwMutex must be locked for writes by the caller.
func (c *alloyCluster) transitionToStateNotReady() {
	if c.clusterState == stateNotReady {
		return
	}
	c.clusterState = stateNotReady

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
	c.clusterChangeCallback()
}

// transitionToStateDeadlinePassed is called when the deadline timer expires. rwMutex must be locked for writes by the caller.
func (c *alloyCluster) transitionToStateDeadlinePassed() {
	if c.clusterState == stateDeadlinePassed {
		return
	}
	c.clusterState = stateDeadlinePassed

	level.Warn(c.log).Log(
		"msg", "deadline passed, marking cluster as ready to admit traffic",
		"minimum_cluster_size", c.opts.MinimumClusterSize,
		"minimum_size_wait_timeout", c.opts.MinimumSizeWaitTimeout,
		"peers_count", len(c.sharder.Peers()),
	)
	c.clusterChangeCallback()
}
