package discovery

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/labels"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// Target refers to a singular discovered endpoint found by a discovery
// component.
type Target map[string]string

// Labels converts Target into a set of sorted labels.
func (t Target) Labels() labels.Labels {
	var lset labels.Labels
	for k, v := range t {
		lset = append(lset, labels.Label{Name: k, Value: v})
	}
	sort.Sort(lset)
	return lset
}

func (t Target) NonMetaLabels() labels.Labels {
	var lset labels.Labels
	for k, v := range t {
		if !strings.HasPrefix(k, model.MetaLabelPrefix) {
			lset = append(lset, labels.Label{Name: k, Value: v})
		}
	}
	sort.Sort(lset)
	return lset
}

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

	creator Creator
}

// New creates a discovery component given arguments and a concrete Discovery implementation function.
func New(o component.Options, args component.Arguments, creator Creator) (*Component, error) {
	c := &Component{
		opts:    o,
		creator: creator,
		// buffered to avoid deadlock from the first immediate update
		newDiscoverer: make(chan struct{}, 1),
	}
	return c, c.Update(args)
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	var stopFn func() = nil
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.newDiscoverer:
			// stop any previously running discovery
			if stopFn != nil {
				stopFn()
			}

			// grab the latest discovery
			c.discMut.Lock()
			disc := c.latestDisc
			c.discMut.Unlock()

			// run the new discovery
			var runFn func()
			runFn, stopFn = c.newRunAndStopForDisc(ctx, disc)
			go runFn()
		}
	}
}

func (c *Component) newRunAndStopForDisc(ctx context.Context, disc DiscovererWithMetrics) (func(), func()) {
	// create new context, so we can cancel it if we get any future updates
	// since it is derived from the main run context, it only needs to be
	// canceled directly if we receive new updates
	newCtx, cancelCtx := context.WithCancel(ctx)

	doneRunning := make(chan struct{})
	runFn := func() {
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

	stopCurrent := func() {
		cancelCtx()
		<-doneRunning
	}

	return runFn, stopCurrent
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
		allTargets := []Target{}
		for _, group := range cache {
			for _, target := range group.Targets {
				labels := map[string]string{}
				// first add the group labels, and then the
				// target labels, so that target labels take precedence.
				for k, v := range group.Labels {
					labels[string(k)] = string(v)
				}
				for k, v := range target {
					labels[string(k)] = string(v)
				}
				allTargets = append(allTargets, labels)
			}
		}
		c.opts.OnStateChange(Exports{Targets: allTargets})
	}

	ticker := time.NewTicker(MaxUpdateFrequency)
	// true if we have received new targets and need to send.
	haveUpdates := false
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
				// DiscovererConfig will send an empty target set to indicate the group (keyed by Source field)
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
