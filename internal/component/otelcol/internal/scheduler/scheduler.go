// Package scheduler exposes utilities for scheduling and running OpenTelemetry
// Collector components.
package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	otelcomponent "go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"

	"github.com/grafana/alloy/internal/component"
)

// DefaultShutdownTimeout is the default maximum time to wait for a component
// to shut down during a hot-reload (Schedule call). This prevents a component
// whose Shutdown() blocks (e.g. a gRPC server draining long-lived connections)
// from hanging /-/reload indefinitely.
//
// Note: this timeout only applies to the hot-reload path (Schedule). The final
// shutdown triggered by Run() cancellation always uses context.Background() so
// Alloy can drain gracefully on process exit.
const DefaultShutdownTimeout = 30 * time.Second

// Scheduler implements manages a set of OpenTelemetry Collector components.
// Scheduler is intended to be used from Alloy components which need to
// schedule OpenTelemetry Collector components; it does not implement the full
// component.Component interface.
//
// Each OpenTelemetry Collector component has one instance per supported
// telemetry signal, which is why Scheduler supports multiple components. For
// example, when creating the otlpreceiver component, you would have three
// total instances: one for logs, one for metrics, and one for traces.
// Scheduler should only be used to manage the different signals of the same
// OpenTelemetry Collector component; this means that otlpreceiver and
// jaegerreceiver should not share the same Scheduler.
type Scheduler struct {
	log *slog.Logger

	healthMut sync.RWMutex
	health    component.Health

	schedMut        sync.Mutex
	schedComponents []otelcomponent.Component // Most recently created components
	host            otelcomponent.Host
	running         bool

	// onPause is called when scheduler is making changes to running components.
	onPause func()
	// onResume is called when scheduler is done making changes to running components.
	onResume func()

	// shutdownTimeout is the maximum duration to wait for components to shut
	// down during a hot-reload (Schedule). 0 means wait forever (no timeout).
	shutdownTimeout time.Duration
}

// New creates a new unstarted Scheduler. Call Run to start it, and call
// Schedule to schedule components to run.
//
// The scheduler uses DefaultShutdownTimeout when stopping components during a
// hot-reload.
func New(l *slog.Logger) *Scheduler {
	return &Scheduler{
		log:             l,
		onPause:         func() {},
		onResume:        func() {},
		shutdownTimeout: DefaultShutdownTimeout,
	}
}

// NewWithPauseCallbacks is like New, but allows to specify onPause() and onResume() callbacks.
// The callbacks are a useful way of pausing and resuming the ingestion of data by the components:
// * onPause() is called before the scheduler stops the components.
// * onResume() is called after the scheduler starts the components.
// The callbacks are used by the Schedule() and Run() functions.
// The scheduler is assumed to start paused; Schedule() won't call onPause() if Run() was never ran.
func NewWithPauseCallbacks(l *slog.Logger, onPause func(), onResume func()) *Scheduler {
	return &Scheduler{
		log:             l,
		onPause:         onPause,
		onResume:        onResume,
		shutdownTimeout: DefaultShutdownTimeout,
	}
}

// SetShutdownTimeout overrides the maximum time that Schedule() will wait for
// old components to shut down during a hot-reload. A value of 0 disables the
// timeout (components are allowed to block indefinitely).
//
// SetShutdownTimeout is intended for testing. Production callers should rely on
// the default set by New or NewWithPauseCallbacks.
func (cs *Scheduler) SetShutdownTimeout(d time.Duration) {
	cs.schedMut.Lock()
	defer cs.schedMut.Unlock()
	cs.shutdownTimeout = d
}

// Schedule a new set of OpenTelemetry Components to run.
// Components will only be started when the Scheduler's Run() function has been called.
//
// Schedule() completely overrides the set of previously running components.
// Components which have been removed since the last call to Schedule will be stopped.
//
// updateConsumers is called after the components are paused and before starting the new components.
// It is expected that this function will set the new set of consumers to the wrapping consumer that's assigned to the Alloy component.
func (cs *Scheduler) Schedule(ctx context.Context, updateConsumers func(), h otelcomponent.Host, cc ...otelcomponent.Component) {
	cs.schedMut.Lock()
	defer cs.schedMut.Unlock()

	// If the scheduler isn't running yet, just update the state.
	// That way the Run function is ready to go.
	if !cs.running {
		cs.schedComponents = cc
		cs.host = h
		updateConsumers()
		return
	}

	// The new components must be setup in this order:
	//
	// 1. Pause consumers
	// 2. Stop the old components
	// 3. Change the consumers
	// 4. Start the new components
	// 5. Start the consumer
	//
	// There could be race conditions if the order above is not followed.

	// 1. Pause consumers
	// This prevents them from accepting new data while we're shutting them down.
	cs.onPause()

	// 2. Stop the old components.
	// Use a bounded context so that a component whose Shutdown() blocks (e.g. a
	// gRPC server draining long-lived connections) cannot hang a hot-reload
	// indefinitely. The caller's ctx is kept for reference but we always derive
	// the stop context from context.Background() so cancellation of ctx (e.g.
	// the component being torn down) does not race with shutdown here.
	stopCtx := ctx
	var stopCancel context.CancelFunc
	if cs.shutdownTimeout > 0 {
		stopCtx, stopCancel = context.WithTimeout(context.Background(), cs.shutdownTimeout)
		defer stopCancel()
	}
	stopComponents(stopCtx, cs.log, cs.schedComponents...)

	// 3. Change the consumers
	// This can only be done after stopping the previous components and before starting the new ones.
	updateConsumers()

	// 4. Start the new components
	cs.log.Debug("scheduling otelcol components", "count", len(cc))
	var err error
	cs.schedComponents, err = startComponents(ctx, cs.log, cs, h, cc...)
	if err != nil {
		cs.log.Error("failed to start some scheduled components", "err", err)
	}
	cs.host = h
	//TODO: What if the trace component failed but the metrics one didn't? Should we resume all consumers?

	// 5. Start the consumer
	// The new components will now start accepting telemetry data.
	cs.onResume()
}

// Run starts the Scheduler and stops the components when the context is cancelled.
func (cs *Scheduler) Run(ctx context.Context) error {
	cs.schedMut.Lock()
	cs.running = true

	cs.onPause()
	started, err := startComponents(ctx, cs.log, cs, cs.host, cs.schedComponents...)
	cs.onResume()

	cs.schedMut.Unlock()

	if len(started) == 0 && err != nil {
		return fmt.Errorf("no components started successfully: %w", err)
	}

	// Make sure we terminate all of our running components on shutdown.
	defer func() {
		cs.schedMut.Lock()
		defer cs.schedMut.Unlock()
		stopComponents(context.Background(), cs.log, cs.schedComponents...)
		// this Resume call should not be needed but is added for robustness to ensure that
		// it does not ever exit in "paused" state.
		cs.onResume()
	}()

	<-ctx.Done()
	return nil
}

// CurrentHealth implements component.HealthComponent. The component is
// reported as healthy when the most recent set of scheduled components were
// started successfully.
func (cs *Scheduler) CurrentHealth() component.Health {
	cs.healthMut.RLock()
	defer cs.healthMut.RUnlock()
	return cs.health
}

func (cs *Scheduler) setHealth(h component.Health) {
	cs.healthMut.Lock()
	defer cs.healthMut.Unlock()
	cs.health = h
}

// stopComponents stops all provided components from cc.
func stopComponents(ctx context.Context, logger *slog.Logger, cc ...otelcomponent.Component) {
	for _, c := range cc {
		if err := c.Shutdown(ctx); err != nil {
			logger.Error("failed to stop scheduled component; future updates may fail", "err", err)
		}
	}
}

type healthScheduler interface {
	setHealth(h component.Health)
}

// startComponent schedules the provided components from cc. It then returns
// the list of components which started successfully.
func startComponents(ctx context.Context, logger *slog.Logger, s healthScheduler, h otelcomponent.Host, cc ...otelcomponent.Component) (started []otelcomponent.Component, errs error) {
	for _, c := range cc {
		if err := c.Start(ctx, h); err != nil {
			logger.Error("failed to start scheduled component", "err", err)
			errs = multierr.Append(errs, err)
		} else {
			started = append(started, c)
		}
	}

	if errs != nil {
		s.setHealth(component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    fmt.Sprintf("failed to create components: %s", errs),
			UpdateTime: time.Now(),
		})
	} else {
		s.setHealth(component.Health{
			Health:     component.HealthTypeHealthy,
			Message:    "started scheduled components",
			UpdateTime: time.Now(),
		})
	}

	return started, errs
}
