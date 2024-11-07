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

	// newComponentsCh is written to when schedComponents gets updated.
	newComponentsCh chan struct{}

	// onPause is called when scheduler is making changes to running components.
	onPause func()
	// onResume is called when scheduler is done making changes to running components.
	onResume func()
}

// New creates a new unstarted Scheduler. Call Run to start it, and call
// Schedule to schedule components to run.
func New(l log.Logger) *Scheduler {
	return &Scheduler{
		log:             l,
		newComponentsCh: make(chan struct{}, 1),
		onPause:         func() {},
		onResume:        func() {},
	}
}

// NewWithPauseCallbacks is like New, but allows to specify onPause and onResume callbacks. The scheduler is assumed to
// start paused and only when its components are scheduled, it will call onResume. From then on, each update to running
// components via Schedule method will trigger a call to onPause and then onResume.
func NewWithPauseCallbacks(l log.Logger, onPause func(), onResume func()) *Scheduler {
	return &Scheduler{
		log:             l,
		newComponentsCh: make(chan struct{}, 1),
		onPause:         onPause,
		onResume:        onResume,
	}
}

// Schedule schedules a new set of OpenTelemetry Components to run. Components
// will only be scheduled when the Scheduler is running.
//
// Schedule completely overrides the set of previously running components;
// components which have been removed since the last call to Schedule will be
// stopped.
func (cs *Scheduler) Schedule(h otelcomponent.Host, cc ...otelcomponent.Component) {
	cs.schedMut.Lock()
	defer cs.schedMut.Unlock()

	cs.schedComponents = cc
	cs.host = h

	select {
	case cs.newComponentsCh <- struct{}{}:
		// Queued new message.
	default:
		// A message is already queued for refreshing running components so we
		// don't have to do anything here.
	}
}

// Run starts the Scheduler. Run will watch for schedule components to appear
// and run them, terminating previously running components if they exist.
func (cs *Scheduler) Run(ctx context.Context) error {
	firstRun := true
	var components []otelcomponent.Component

	// Make sure we terminate all of our running components on shutdown.
	defer func() {
		if !firstRun { // always handle the callbacks correctly
			cs.onPause()
		}
		cs.stopComponents(context.Background(), components...)
		// We don't resume, as the scheduler is exiting.
	}()

	// Wait for a write to cs.newComponentsCh. The initial list of components is
	// always empty so there's nothing to do until cs.newComponentsCh is written
	// to.
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-cs.newComponentsCh:
			if !firstRun {
				cs.onPause() // do not pause on first run
			} else {
				firstRun = false
			}
			// Stop the old components before running new scheduled ones.
			cs.stopComponents(ctx, components...)

			cs.schedMut.Lock()
			components = cs.schedComponents
			host := cs.host
			cs.schedMut.Unlock()

			level.Debug(cs.log).Log("msg", "scheduling components", "count", len(components))
			components = cs.startComponents(ctx, host, components...)
			cs.onResume()
		}
	}
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
