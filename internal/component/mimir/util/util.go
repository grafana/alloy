package util

import (
	"context"
	"fmt"

	"go.uber.org/atomic"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/ckit/shard"
)

// TODO: Use the functions in this file in the mimir.rules.kubernetes component.

// healthReporter encapsulates the logic for marking a component as healthy or
// not healthy to make testing portions of the Component easier.
type HealthReporter interface {
	// reportUnhealthy marks the owning component as unhealthy
	ReportUnhealthy(err error)
	// reportHealthy marks the owning component as healthy
	ReportHealthy()
}

// lifecycle encapsulates state transitions and mutable state to make testing
// portions of the Component easier.
type Lifecycle[T any] interface {
	// update updates the Arguments used for configuring the Component.
	LifecycleUpdate(args T)

	// startup starts processing events from Kubernetes object changes.
	Startup(ctx context.Context) error

	// restart stops the component if running and then starts it again.
	Restart(ctx context.Context) error

	// shutdown stops the component, blocking until existing events are processed.
	Shutdown()

	// syncState requests that Mimir ruler state be synced independent of any
	// changes made to Kubernetes objects.
	SyncState()
}

// leadership encapsulates the logic for checking if this instance of the Component
// is the leader among all instances to avoid conflicting updates of the Mimir API.
type Leadership interface {
	// update checks if this component instance is still the leader, stores the result,
	// and returns true if the leadership status has changed since the last time update
	// was called.
	Update() (bool, error)

	// isLeader returns true if this component instance is the leader, false otherwise.
	IsLeader() bool
}

// componentLeadership implements leadership based on checking ownership of a specific
// key using a cluster.Cluster service.
type ComponentLeadership struct {
	id      string
	logger  log.Logger
	cluster cluster.Cluster
	leader  atomic.Bool
}

func NewComponentLeadership(id string, logger log.Logger, cluster cluster.Cluster) *ComponentLeadership {
	return &ComponentLeadership{
		id:      id,
		logger:  logger,
		cluster: cluster,
	}
}

func (l *ComponentLeadership) Update() (bool, error) {
	peers, err := l.cluster.Lookup(shard.StringKey(l.id), 1, shard.OpReadWrite)
	if err != nil {
		return false, fmt.Errorf("unable to determine leader for %s: %w", l.id, err)
	}

	if len(peers) != 1 {
		return false, fmt.Errorf("unexpected peers from leadership check: %+v", peers)
	}

	isLeader := peers[0].Self
	level.Info(l.logger).Log("msg", "checked leadership of component", "is_leader", isLeader)
	return l.leader.Swap(isLeader) != isLeader, nil
}

func (l *ComponentLeadership) IsLeader() bool {
	return l.leader.Load()
}
