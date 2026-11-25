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
// By default, Schedule doesn't wait for a running Scheduler to switch to the
// new components. It hands them to Run, which switches to them in the
// background. Schedule is called from the Update of an Alloy component, and
// Update must not wait on the data path: during a config reload, the
// components that this component sends data to may be paused until every
// Update of the reload has returned. Stopping the old components or swapping
// the consumers can wait for data that is blocked on them, so the reload would
// never finish. See https://github.com/grafana/alloy/issues/7762.
type Scheduler struct {
	log *slog.Logger

	healthMut sync.RWMutex
	health    component.Health

	// schedMut is held while the scheduled components are started, stopped or
	// replaced. This can take a long time, so Schedule only takes it when the
	// Scheduler is synchronous or isn't running.
	schedMut        sync.Mutex
	schedComponents []otelcomponent.Component // Most recently created components
	host            otelcomponent.Host

	// stateMut protects runs, pending and replaced. It is never held for long,
	// and it is taken after schedMut when both are needed.
	stateMut sync.Mutex
	// runs is the number of calls to Run that haven't returned. It is usually 0
	// or 1. The runtime may start a new Run before an old one has returned, if
	// the old one took longer than the task shutdown deadline.
	runs    int
	pending *scheduleRequest
	// replaced holds the components of requests that were replaced before Run
	// took them. They were created but never started, and Run shuts them down.
	replaced []otelcomponent.Component

	// pendingCh tells Run that pending is set.
	pendingCh chan struct{}

	// synchronous makes Schedule switch to the new components before it
	// returns, also when the Scheduler is running.
	synchronous bool

	// onPause is called when scheduler is making changes to running components.
	onPause func()
	// onResume is called when scheduler is done making changes to running components.
	onResume func()
}

// scheduleRequest is a set of components that Schedule handed to Run.
type scheduleRequest struct {
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

// NewSynchronous is like New, but Schedule switches to the new components
// before it returns, also when the Scheduler is running. Use it only when
// other components may use the new components as soon as Schedule returns, for
// example an extension whose exports point to them. Its Schedule can wait on
// the data path.
func NewSynchronous(l *slog.Logger) *Scheduler {
	s := New(l)
	s.synchronous = true
	return s
}

// NewWithPauseCallbacks is like New, but allows to specify onPause() and onResume() callbacks.
// The callbacks are a useful way of pausing and resuming the ingestion of data by the components:
// * onPause() is called before the scheduler stops the components.
// * onResume() is called after the scheduler starts the components.
// The callbacks are used by the Schedule() and Run() functions.
// The scheduler is assumed to start paused; Schedule() won't call onPause() if Run() was never ran.
func NewWithPauseCallbacks(l *slog.Logger, onPause func(), onResume func()) *Scheduler {
	return &Scheduler{
		log:       l,
		pendingCh: make(chan struct{}, 1),
		onPause:   onPause,
		onResume:  onResume,
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
// When the Scheduler is running, Schedule returns before the switch to the
// new components is done, unless the Scheduler was created with
// NewSynchronous. Run does the switch in the background. If Schedule is
// called again before Run gets to a set of components, only the latest set is
// started. The skipped sets are shut down without being started.
func (cs *Scheduler) Schedule(ctx context.Context, updateConsumers func(), h otelcomponent.Host, cc ...otelcomponent.Component) {
	req := &scheduleRequest{ctx: ctx, updateConsumers: updateConsumers, host: h, components: cc}

	// Most calls to a running Scheduler return here, without waiting for
	// schedMut, which Run may hold for a long time.
	if cs.handToRun(req) {
		return
	}

	cs.schedMut.Lock()
	defer cs.schedMut.Unlock()

	// Run may have started while this call waited for schedMut. Run can't start
	// or stop while schedMut is held, so running doesn't change after this.
	if cs.handToRun(req) {
		return
	}

	cs.stateMut.Lock()
	running := cs.runs > 0
	cs.stateMut.Unlock()

	// If the scheduler isn't running yet, just update the state.
	// That way the Run function is ready to go.
	if !running {
		cs.schedComponents = cc
		cs.host = h
		updateConsumers()
		return
	}

	// A synchronous Scheduler switches to the new components here.
	cs.switchComponents(req)
}

// handToRun gives req to Run if the Scheduler is running and isn't
// synchronous. It returns false if it didn't. A request that Run hasn't taken
// yet is replaced, so its components are never started. Run shuts them down.
func (cs *Scheduler) handToRun(req *scheduleRequest) bool {
	if cs.synchronous {
		return false
	}

	cs.stateMut.Lock()
	defer cs.stateMut.Unlock()
	if cs.runs == 0 {
		return false
	}

	if cs.pending != nil {
		cs.replaced = append(cs.replaced, cs.pending.components...)
	}
	cs.pending = req
	select {
	case cs.pendingCh <- struct{}{}:
	default: // Run has already been told.
	}
	return true
}

// switchComponents stops the running components and starts the components of
// req. schedMut must be held.
func (cs *Scheduler) switchComponents(req *scheduleRequest) {
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

	// 2. Stop the old components
	stopComponents(req.ctx, cs.log, cs.schedComponents...)

	// 3. Change the consumers
	// This can only be done after stopping the previous components and before starting the new ones.
	req.updateConsumers()

	// 4. Start the new components
	cs.log.Debug("scheduling otelcol components", "count", len(req.components))
	var err error
	cs.schedComponents, err = startComponents(req.ctx, cs.log, cs, req.host, req.components...)
	if err != nil {
		cs.log.Error("failed to start some scheduled components", "err", err)
	}
	cs.host = req.host
	//TODO: What if the trace component failed but the metrics one didn't? Should we resume all consumers?

	// 5. Start the consumer
	// The new components will now start accepting telemetry data.
	cs.onResume()
}

// takePending returns the request that Schedule handed to Run, if any, and
// the components of the requests that it replaced.
func (cs *Scheduler) takePending() (req *scheduleRequest, replaced []otelcomponent.Component) {
	cs.stateMut.Lock()
	defer cs.stateMut.Unlock()
	req, replaced = cs.pending, cs.replaced
	cs.pending, cs.replaced = nil, nil
	return req, replaced
}

// Run starts the Scheduler and stops the components when the context is cancelled.
// While it runs, it switches to the components that Schedule hands to it.
func (cs *Scheduler) Run(ctx context.Context) error {
	cs.schedMut.Lock()
	cs.stateMut.Lock()
	cs.runs++
	cs.stateMut.Unlock()

	cs.onPause()
	started, err := startComponents(ctx, cs.log, cs, cs.host, cs.schedComponents...)
	cs.onResume()

	if len(started) == 0 && err != nil {
		// Mark the Scheduler as stopped before schedMut is released, so that a
		// synchronous Schedule doesn't start components that nobody stops.
		cs.markRunExited()
		cs.schedMut.Unlock()
		return fmt.Errorf("no components started successfully: %w", err)
	}
	cs.schedMut.Unlock()

	// Make sure we terminate all of our running components on shutdown.
	defer func() {
		cs.schedMut.Lock()
		defer cs.schedMut.Unlock()
		stopComponents(context.Background(), cs.log, cs.schedComponents...)
		// this Resume call should not be needed but is added for robustness to ensure that
		// it does not ever exit in "paused" state.
		cs.onResume()
		cs.markRunExited()
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-cs.pendingCh:
			req, replaced := cs.takePending()
			// The replaced components were never started.
			stopComponents(context.Background(), cs.log, replaced...)
			if req == nil {
				continue
			}
			// The switch can wait on the data path, for example while the old
			// components flush data to a downstream component that is paused
			// during a config reload. Run doesn't return until the switch is
			// done, even if ctx is canceled.
			cs.schedMut.Lock()
			cs.switchComponents(req)
			cs.schedMut.Unlock()
		}
	}
}

// markRunExited is called when Run exits. When no other Run is active, a
// request that Run hasn't taken yet is kept as the state of the stopped
// Scheduler, so that the next Run starts its components. schedMut must be held.
func (cs *Scheduler) markRunExited() {
	cs.stateMut.Lock()
	cs.runs--
	if cs.runs > 0 {
		cs.stateMut.Unlock()
		return
	}
	req, replaced := cs.pending, cs.replaced
	cs.pending, cs.replaced = nil, nil
	cs.stateMut.Unlock()

	// The replaced components were never started.
	stopComponents(context.Background(), cs.log, replaced...)
	if req != nil {
		cs.schedComponents = req.components
		cs.host = req.host
		req.updateConsumers()
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
