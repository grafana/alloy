package util

import (
	"context"

	"github.com/grafana/alloy/internal/component/common/kubernetes"
)

// TODO: Use the functions in this file in the mimir.rules.kubernetes component.

const (
	EventTypeSyncMimir kubernetes.EventType = "sync-mimir"
)

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
