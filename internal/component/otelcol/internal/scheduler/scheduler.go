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
//
// Schedule only hands the components to Run, which owns the running
// components and switches to the new ones. Schedule is called from the Update
// of an Alloy component, and Update must not wait on the data path: during a
// config reload, a component that this one sends data to may be paused until
// every Update of the reload has returned. Stopping the old components or
// swapping the consumers can wait for data that is blocked on it. See
// https://github.com/grafana/alloy/issues/7762.
type Scheduler struct {
	log *slog.Logger

	healthMut sync.RWMutex
	health    component.Health

	// next holds the latest components given to Schedule that Run hasn't
	// switched to yet.
	next chan scheduled

	// onPause is called when scheduler is making changes to running components.
	onPause func()
	// onResume is called when scheduler is done making changes to running components.
	onResume func()
}

// scheduled is a set of components given to Schedule.
type scheduled struct {
	ctx             context.Context
	updateConsumers func()
	host            otelcomponent.Host
	components      []otelcomponent.Component
}

// New creates a new unstarted Scheduler. Call Run to start it, and call
// Schedule to schedule components to run.
func New(l *slog.Logger) *Scheduler {
	return NewWithPauseCallbacks(l, func() {}, func() {})
}

// NewWithPauseCallbacks is like New, but allows to specify onPause() and onResume() callbacks.
// The callbacks are a useful way of pausing and resuming the ingestion of data by the components:
// * onPause() is called before the scheduler stops the components.
// * onResume() is called after the scheduler starts the components.
// The callbacks are used by the Run() function.
func NewWithPauseCallbacks(l *slog.Logger, onPause func(), onResume func()) *Scheduler {
	return &Scheduler{
		log:      l,
		next:     make(chan scheduled, 1),
		onPause:  onPause,
		onResume: onResume,
	}
}

// Schedule a new set of OpenTelemetry Components to run.
// Components will only be started when the Scheduler's Run() function has been called.
//
// Schedule() completely overrides the set of previously running components.
// Components which have been removed since the last call to Schedule will be stopped.
//
// updateConsumers is called after the components are paused and before starting the new components.
// It is expected that this function will set the new set of consumers to the wrapping consumer that's assigned to the Alloy component.
//
// Schedule doesn't wait for Run to switch to the new components. If it is
// called again before Run gets to them, only the latest set is started.
func (cs *Scheduler) Schedule(ctx context.Context, updateConsumers func(), h otelcomponent.Host, cc ...otelcomponent.Component) {
	s := scheduled{ctx: ctx, updateConsumers: updateConsumers, host: h, components: cc}
	for {
		select {
		case cs.next <- s:
			return
		case <-cs.next:
			// Drop the set that Run hasn't got to yet. It was never started.
		}
	}
}

// Run starts the Scheduler and stops the components when the context is cancelled.
// While it runs, it switches to the components given to Schedule.
func (cs *Scheduler) Run(ctx context.Context) error {
	var running []otelcomponent.Component

	// Make sure we terminate all of our running components on shutdown.
	defer func() {
		stopComponents(context.Background(), cs.log, running...)
		// this Resume call should not be needed but is added for robustness to ensure that
		// it does not ever exit in "paused" state.
		cs.onResume()
	}()

	for first := true; ; first = false {
		var s scheduled
		select {
		case <-ctx.Done():
			return nil
		case s = <-cs.next:
		}
		if ctx.Err() != nil {
			// select picks at random when both cases are ready. Don't start
			// components that we would stop at once. Keep them for the next Run.
			cs.requeue(s)
			return nil
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
		// The stop can wait on the data path, so Run may take a while to see
		// that ctx is canceled.

		// 1. Pause consumers
		// This prevents them from accepting new data while we're shutting them down.
		cs.onPause()

		// 2. Stop the old components
		stopComponents(s.ctx, cs.log, running...)

		// 3. Change the consumers
		// This can only be done after stopping the previous components and before starting the new ones.
		s.updateConsumers()

		// 4. Start the new components
		cs.log.Debug("scheduling otelcol components", "count", len(s.components))
		var err error
		running, err = startComponents(s.ctx, cs.log, cs, s.host, s.components...)
		//TODO: What if the trace component failed but the metrics one didn't? Should we resume all consumers?

		// 5. Start the consumer
		// The new components will now start accepting telemetry data.
		cs.onResume()

		if err != nil {
			if first && len(running) == 0 {
				// Keep the components, so that the next Run tries to start them
				// again.
				cs.requeue(s)
				return fmt.Errorf("no components started successfully: %w", err)
			}
			cs.log.Error("failed to start some scheduled components", "err", err)
		}
	}
}

// requeue gives s to the next Run, unless Schedule has given newer components.
func (cs *Scheduler) requeue(s scheduled) {
	select {
	case cs.next <- s:
	default:
	}
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
