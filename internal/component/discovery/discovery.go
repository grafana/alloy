package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

// Exports holds values which are exported by all discovery components.
type Exports struct {
	Targets []Target `alloy:"targets,attr"`
}

// DiscovererConfig is an alias for Prometheus' DiscovererConfig interface, so users of this package don't need
// to import github.com/prometheus/prometheus/discover as well.
type DiscovererConfig discovery.Config

// Creator is a function provided by an implementation to create a concrete DiscovererConfig instance.
type Creator func(component.Arguments) (DiscovererConfig, error)

// Component is a reusable component for any discovery implementation.
// it will handle dynamic updates and exporting targets appropriately for a scrape implementation.
type Component struct {
	opts component.Options

	discMut       sync.Mutex
	latestDisc    DiscovererWithMetrics
	newDiscoverer chan struct{}

	creator            Creator
	debugDataPublisher livedebugging.DebugDataPublisher
}

var _ component.Component = (*Component)(nil)
var _ component.LiveDebugging = (*Component)(nil)

// New creates a discovery component given arguments and a concrete Discovery implementation function.
func New(o component.Options, args component.Arguments, creator Creator) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:    o,
		creator: creator,
		// buffered to avoid deadlock from the first immediate update
		newDiscoverer:      make(chan struct{}, 1),
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}
	return c, c.Update(args)
}

// ConvertibleConfig is used to more conveniently convert a configuration struct into a DiscovererConfig.
type ConvertibleConfig interface {
	// Convert converts the struct into a DiscovererConfig.
	Convert() DiscovererConfig
}

// NewFromConvertibleConfig creates a discovery component given a ConvertibleConfig. Convenience function for New.
func NewFromConvertibleConfig[T ConvertibleConfig](opts component.Options, conf T) (component.Component, error) {
	return New(opts, conf, func(args component.Arguments) (DiscovererConfig, error) {
		return args.(T).Convert(), nil
	})
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	var (
		runFn                func()
		stopFn               func() = nil
		stopCurrentIfRunning        = func() {
			if stopFn != nil {
				stopFn()
			}
		}
	)
	defer stopCurrentIfRunning()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.newDiscoverer:
			// stop the current discovery (blocking, unregisters metrics etc.)
			stopCurrentIfRunning()

			// grab the latest discovery
			c.discMut.Lock()
			disc := c.latestDisc
			c.discMut.Unlock()

			// run the new discovery and grab the new stop function
			runFn, stopFn = c.newRunAndStopForDisc(ctx, disc)
			go runFn()
		}
	}
}

// newRunAndStopForDisc creates a new runFn and stopFn functions for a given DiscovererWithMetrics. The run function will
// register the metrics and run discoverer until stopFn is called. After that it will unregister the metrics. The stop
// function will block until run function is finished cleaning up.
func (c *Component) newRunAndStopForDisc(ctx context.Context, disc DiscovererWithMetrics) (runFn func(), stopFn func()) {
	// create new context, so we can cancel it if we get any future updates
	// since it is derived from the main run context, it only needs to be
	// canceled directly if we receive new updates
	newCtx, cancelCtx := context.WithCancel(ctx)

	doneRunning := make(chan struct{})
	runFn = func() {
		// DiscovererWithMetrics needs to have its metrics registered before running.
		err := disc.Register()
		if err != nil {
			_ = level.Warn(c.opts.Logger).Log("msg", "failed to register discoverer metrics", "err", err)
		}

		// Run the discoverer.
		c.runDiscovery(newCtx, disc)

		// DiscovererWithMetrics needs to have its metrics unregistered after running.
		disc.Unregister()
		doneRunning <- struct{}{}
	}

	stopFn = func() {
		cancelCtx()
		// Wait till the runFn is done and cleaned up / unregistered the metrics.
		<-doneRunning
	}

	return runFn, stopFn
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	discConfig, err := c.creator(args)
	if err != nil {
		return err
	}
	disc, err := NewDiscovererWithMetrics(discConfig, c.opts.Registerer, c.opts.Logger)
	if err != nil {
		return err
	}
	c.discMut.Lock()
	c.latestDisc = disc
	c.discMut.Unlock()

	select {
	case c.newDiscoverer <- struct{}{}:
	default:
	}

	return nil
}

// MaxUpdateFrequency is the minimum time to wait between updating targets.
// Prometheus uses a static threshold. Do not recommend changing this, except for tests.
var MaxUpdateFrequency = 5 * time.Second

// runDiscovery is a utility for consuming and forwarding target groups from a discoverer.
// It will handle collating targets (and clearing), as well as time based throttling of updates.
func (c *Component) runDiscovery(ctx context.Context, d DiscovererWithMetrics) {
	// all targets we have seen so far
	cache := map[string]*targetgroup.Group{}

	ch := make(chan []*targetgroup.Group)
	runExited := make(chan struct{})
	go func() {
		d.Run(ctx, ch)
		runExited <- struct{}{}
	}()

	// function to convert and send targets in format scraper expects
	send := func() {
		allTargets := toAlloyTargets(cache)
		componentID := livedebugging.ComponentID(c.opts.ID)
		c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
			componentID,
			livedebugging.Target,
			uint64(len(allTargets)),
			func() string { return fmt.Sprintf("%s", allTargets) },
		))
		c.opts.OnStateChange(Exports{Targets: allTargets})
	}

	ticker := time.NewTicker(MaxUpdateFrequency)
	// true if we have received new targets and need to send. Initially set it to true to send empty targets in case
	// the discoverer never sends any targets.
	haveUpdates := true
	for {
		select {
		case <-ticker.C:
			if haveUpdates {
				send()
				haveUpdates = false
			}
		case <-ctx.Done():
			// shut down the discoverer - send latest targets and wait for discoverer goroutine to exit
			send()
			<-runExited
			return
		case groups := <-ch:
			for _, group := range groups {
				// Discoverer will send an empty target set to indicate the group (keyed by Source field)
				// should be removed
				if len(group.Targets) == 0 {
					delete(cache, group.Source)
				} else {
					cache[group.Source] = group
				}
			}
			haveUpdates = true
		}
	}
}

func toAlloyTargets(cache map[string]*targetgroup.Group) []Target {
	targetsCount := 0
	for _, group := range cache {
		targetsCount += len(group.Targets)
	}
	allTargets := make([]Target, 0, targetsCount)

	for _, group := range cache {
		for _, target := range group.Targets {
			allTargets = append(allTargets, NewTargetFromSpecificAndBaseLabelSet(target, group.Labels))
		}
	}
	return allTargets
}

func (c *Component) LiveDebugging() {}
