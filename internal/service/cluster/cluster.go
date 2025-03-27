package cluster

import (
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"go.uber.org/atomic"
	"golang.org/x/time/rate"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

const (
	// admissionUpdateMinInterval defines the minimum time interval between updates to the admission control state.
	admissionUpdateMinInterval = time.Second
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
	// If the cluster is not ready to accept traffic, (nil, nil) will be returned, indicating there are no peers
	// available.
	Lookup(key shard.Key, replicationFactor int, op shard.Op) ([]peer.Peer, error)

	// Peers returns the current set of peers for a Node. If the cluster is not yet ready to accept traffic, it will
	// return nil.
	Peers() []peer.Peer
}

// alloyCluster implements the Cluster interface and manages the admission control logic.
type alloyCluster struct {
	log     log.Logger
	sharder shard.Sharder
	opts    Options

	limiter *rate.Limiter

	minimumSizeDeadline   atomic.Time
	isReadyToAdmitTraffic atomic.Bool
}

var _ Cluster = (*alloyCluster)(nil)

func newAlloyCluster(
	log log.Logger,
	sharder shard.Sharder,
	opts Options,
) *alloyCluster {
	c := &alloyCluster{
		log:     log,
		sharder: sharder,
		opts:    opts,
		limiter: rate.NewLimiter(rate.Every(admissionUpdateMinInterval), 1),
	}

	// Initialize the minimum size deadline if it's set.
	if opts.MinimumSizeWaitTimeout != 0 {
		c.minimumSizeDeadline.Store(time.Now().Add(opts.MinimumSizeWaitTimeout))
	} else {
		c.minimumSizeDeadline.Store(time.Time{})
	}
	return c
}

// Lookup implements the Cluster interface. It determines the set of replicationFactor owners for a given key.
func (c *alloyCluster) Lookup(key shard.Key, replicationFactor int, op shard.Op) ([]peer.Peer, error) {
	if !c.readyToAdmitTraffic() {
		// Return nil peers when cluster is not ready to admit traffic due to minimum size requirements
		return nil, nil
	}
	return c.sharder.Lookup(key, replicationFactor, op)
}

// Peers implements the Cluster interface. It returns the current set of peers for a Node.
func (c *alloyCluster) Peers() []peer.Peer {
	if !c.readyToAdmitTraffic() {
		// Return nil peers when cluster is not ready to admit traffic due to minimum size requirements
		return nil
	}
	return c.sharder.Peers()
}

// readyToAdmitTraffic checks if the cluster is ready to admit traffic.
func (c *alloyCluster) readyToAdmitTraffic() bool {
	c.updateReadyToAdmitTraffic() // update if needed
	return c.isReadyToAdmitTraffic.Load()
}

// updateReadyToAdmitTraffic updates the isReadyToAdmitTraffic flag based on the current cluster state.
func (c *alloyCluster) updateReadyToAdmitTraffic() {
	// Don't update too frequently.
	if !c.limiter.Allow() {
		return
	}

	// If clustering is disabled, the service is always ready to admit traffic
	if !c.opts.EnableClustering {
		c.isReadyToAdmitTraffic.Store(true)
		return
	}

	// No minimum cluster size is set = always ready to admit traffic.
	if c.opts.MinimumClusterSize == 0 {
		c.isReadyToAdmitTraffic.Store(true)
		return
	}

	// Number of peers is greater than the minimum cluster size = ready to admit traffic.
	if len(c.sharder.Peers()) >= c.opts.MinimumClusterSize {
		// Reset the deadline if it is configured:
		if c.opts.MinimumSizeWaitTimeout != 0 {
			c.minimumSizeDeadline.Store(time.Now().Add(c.opts.MinimumSizeWaitTimeout))
		}
		if !c.isReadyToAdmitTraffic.Load() { // log if previously not ready
			level.Info(c.log).Log(
				"msg", "minimum cluster size reached, marking cluster as ready to admit traffic",
				"minimum_cluster_size", c.opts.MinimumClusterSize,
				"peers_count", len(c.sharder.Peers()),
			)
		}

		c.isReadyToAdmitTraffic.Store(true)
		return
	}

	// Deadline is set, and it's past the deadline = ready to admit traffic.
	deadlineValue := c.minimumSizeDeadline.Load()
	isDeadlineSet := deadlineValue != time.Time{}
	if isDeadlineSet && time.Now().After(deadlineValue) {
		if !c.isReadyToAdmitTraffic.Load() { // log if previously not ready
			level.Warn(c.log).Log(
				"msg", "deadline passed, marking cluster as ready to admit traffic",
				"minimum_cluster_size", c.opts.MinimumClusterSize,
				"minimum_size_wait_timeout", c.opts.MinimumSizeWaitTimeout,
				"peers_count", len(c.sharder.Peers()),
			)
		}
		c.isReadyToAdmitTraffic.Store(true)
		return
	}

	// Deadline is either not set or it didn't yet pass, and the number of peers is less
	// than the minimum. So we can't admit traffic.
	if c.isReadyToAdmitTraffic.Load() { // log if previously was ready
		level.Warn(c.log).Log(
			"msg", "minimum cluster size requirements are not met - marking cluster as not ready for traffic",
			"minimum_cluster_size", c.opts.MinimumClusterSize,
			"peers_count", len(c.sharder.Peers()),
		)
	}
	c.isReadyToAdmitTraffic.Store(false) // set as not ready
}
