// Package scheduler exposes utilities for scheduling and running OpenTelemetry
// Collector components.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	otelcomponent "go.opentelemetry.io/collector/component"
	"go.uber.org/multierr"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/logging/level"
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
type Scheduler struct {
	log log.Logger

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
}

// TODO: Delete this function? I don't think it's used anywhere.
// New creates a new unstarted Scheduler. Call Run to start it, and call
// Schedule to schedule components to run.
func New(l log.Logger) *Scheduler {
	return &Scheduler{
		log:      l,
		onPause:  func() {},
		onResume: func() {},
	}
}

// TODO: Rename to "New"?
// TODO: Write a new comment to explain what this method does.
func NewWithPauseCallbacks(l log.Logger, onPause func(), onResume func()) *Scheduler {
	//TODO: Instead of assuming that the scheduler is paused, just call onPause() here.
	return &Scheduler{
		log:      l,
		onPause:  onPause,
		onResume: onResume,
	}
}

// Schedule schedules a new set of OpenTelemetry Components to run. Components
// will only be scheduled when the Scheduler is running.
//
// Schedule completely overrides the set of previously running components;
// components which have been removed since the last call to Schedule will be
// stopped.
func (cs *Scheduler) Schedule(ctx context.Context, h otelcomponent.Host, cc ...otelcomponent.Component) {
	cs.schedMut.Lock()
	defer cs.schedMut.Unlock()

	cs.schedComponents = cc
	cs.host = h

	if !cs.running {
		return
	}

	cs.runScheduled(ctx)
}

// Run starts the Scheduler and stops the components when the context is cancelled.
func (cs *Scheduler) Run(ctx context.Context) error {
	cs.schedMut.Lock()
	cs.running = true
	cs.runScheduled(ctx)
	cs.schedMut.Unlock()

	// Make sure we terminate all of our running components on shutdown.
	defer func() {
		cs.schedMut.Lock()
		defer cs.schedMut.Unlock()
		cs.stopComponents(context.Background(), cs.schedComponents...)
	}()

	<-ctx.Done()
	return nil
}

func (cs *Scheduler) runScheduled(ctx context.Context) {
	cs.onPause()

	// Stop the old components before running new scheduled ones.
	cs.stopComponents(ctx, cs.schedComponents...)

	level.Debug(cs.log).Log("msg", "scheduling components", "count", len(cs.schedComponents))
	cs.schedComponents = cs.startComponents(ctx, cs.host, cs.schedComponents...)
	//TODO: Check if there were errors? What if the trace component failed but the metrics one didn't? Should we resume all consumers?

	cs.onResume()
}

func (cs *Scheduler) stopComponents(ctx context.Context, cc ...otelcomponent.Component) {
	for _, c := range cc {
		if err := c.Shutdown(ctx); err != nil {
			level.Error(cs.log).Log("msg", "failed to stop scheduled component; future updates may fail", "err", err)
		}
	}
}

// startComponent schedules the provided components from cc. It then returns
// the list of components which started successfully.
func (cs *Scheduler) startComponents(ctx context.Context, h otelcomponent.Host, cc ...otelcomponent.Component) (started []otelcomponent.Component) {
	var errs error

	for _, c := range cc {
		if err := c.Start(ctx, h); err != nil {
			level.Error(cs.log).Log("msg", "failed to start scheduled component", "err", err)
			errs = multierr.Append(errs, err)
		} else {
			started = append(started, c)
		}
	}

	if errs != nil {
		cs.setHealth(component.Health{
			Health:     component.HealthTypeUnhealthy,
			Message:    fmt.Sprintf("failed to create components: %s", errs),
			UpdateTime: time.Now(),
		})
	} else {
		cs.setHealth(component.Health{
			Health:     component.HealthTypeHealthy,
			Message:    "started scheduled components",
			UpdateTime: time.Now(),
		})
	}

	return started
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
