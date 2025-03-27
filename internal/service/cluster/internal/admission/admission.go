package admission

import (
	"fmt"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/ckit/shard"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// Controller manages the admission control for a cluster node,
// determining whether the node is ready to admit traffic based on
// cluster size requirements.
type Controller struct {
	log     log.Logger
	sharder shard.Sharder

	// Configuration options
	enableClustering       bool
	minimumClusterSize     int
	minimumSizeWaitTimeout time.Duration

	// State
	minimumSizeDeadline   atomic.Time
	isReadyToAdmitTraffic atomic.Bool
}

// NewController creates a new admission controller with the given configuration.
func NewController(
	log log.Logger,
	sharder shard.Sharder,
	enableClustering bool,
	minimumClusterSize int,
	minimumSizeWaitTimeout time.Duration,
) *Controller {
	c := &Controller{
		log:                    log,
		sharder:                sharder,
		enableClustering:       enableClustering,
		minimumClusterSize:     minimumClusterSize,
		minimumSizeWaitTimeout: minimumSizeWaitTimeout,
	}

	// Initialize isReadyToAdmitTraffic. The cluster is ready when clustering is disabled or no minimum size is required.
	c.isReadyToAdmitTraffic.Store(!enableClustering || minimumClusterSize == 0)

	// Initialize the minimum size deadline if it's set.
	if minimumClusterSize > 0 && minimumSizeWaitTimeout != 0 {
		c.minimumSizeDeadline.Store(time.Now().Add(minimumSizeWaitTimeout))
	} else {
		c.minimumSizeDeadline.Store(time.Time{})
	}

	fmt.Printf("=========> Initial minimum cluster size deadline: %v\n", c.minimumSizeDeadline.Load())

	return c
}

// UpdateReadyToAdmitTraffic updates the isReadyToAdmitTraffic flag based on the current cluster state.
func (c *Controller) UpdateReadyToAdmitTraffic() {
	fmt.Printf("=========> Minimum cluster size deadline: %v\n", c.minimumSizeDeadline.Load())
	// If clustering is disabled, the service is always ready to admit traffic
	if !c.enableClustering {
		c.isReadyToAdmitTraffic.Store(true)
		fmt.Println("=========> clustering is disabled")
		return
	}

	// No minimum cluster size is set = always ready to admit traffic.
	if c.minimumClusterSize == 0 {
		c.isReadyToAdmitTraffic.Store(true)
		fmt.Println("=========> minimum cluster size is not set")
		return
	}

	// Number of peers is greater than the minimum cluster size = ready to admit traffic.
	if len(c.sharder.Peers()) >= c.minimumClusterSize {
		// Reset the deadline if it is configured:
		if c.minimumSizeWaitTimeout != 0 {
			c.minimumSizeDeadline.Store(time.Now().Add(c.minimumSizeWaitTimeout))
		}
		if !c.isReadyToAdmitTraffic.Load() { // log if previously not ready
			level.Info(c.log).Log(
				"msg", "minimum cluster size reached, marking cluster as ready to admit traffic",
				"minimum_cluster_size", c.minimumClusterSize,
				"peers_count", len(c.sharder.Peers()),
			)
		}

		fmt.Println("=========> minimum cluster size reached")
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
				"minimum_cluster_size", c.minimumClusterSize,
				"minimum_size_wait_timeout", c.minimumSizeWaitTimeout,
				"peers_count", len(c.sharder.Peers()),
			)
		}
		c.isReadyToAdmitTraffic.Store(true)
		fmt.Println("=========> deadline passed")
		return
	}

	// Deadline is either not set or it didn't yet pass, and the number of peers is less
	// than the minimum. So we can't admit traffic.
	if c.isReadyToAdmitTraffic.Load() { // log if previously was ready
		level.Warn(c.log).Log(
			"msg", "minimum cluster size requirements are not met - marking cluster as not ready for traffic",
			"minimum_cluster_size", c.minimumClusterSize,
			"peers_count", len(c.sharder.Peers()),
		)
	}
	fmt.Println("=========> minimum cluster size requirements are not met")
	c.isReadyToAdmitTraffic.Store(false) // set as not ready
}

// ReadyToAdmitTraffic checks if the cluster is ready to admit traffic.
func (c *Controller) ReadyToAdmitTraffic() bool {
	// TODO(thampiotr): calling it here each time is probably too expensive...
	c.UpdateReadyToAdmitTraffic()
	return c.isReadyToAdmitTraffic.Load()
}
