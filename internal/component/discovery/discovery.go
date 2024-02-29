package discovery

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/grafana/agent/internal/component"
	"github.com/grafana/agent/internal/service/cluster"
	"github.com/grafana/ckit/shard"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery"
	"github.com/prometheus/prometheus/discovery/targetgroup"
	"github.com/prometheus/prometheus/model/labels"
)

// Target refers to a singular discovered endpoint found by a discovery
// component.
type Target map[string]string

// DistributedTargets uses the node's Lookup method to distribute discovery
// targets when a Flow component runs in a cluster.
type DistributedTargets struct {
	useClustering bool
	cluster       cluster.Cluster
	targets       []Target
}

// NewDistributedTargets creates the abstraction that allows components to
// dynamically shard targets between components.
func NewDistributedTargets(e bool, n cluster.Cluster, t []Target) DistributedTargets {
	return DistributedTargets{e, n, t}
}

// Get distributes discovery targets a clustered environment.
//
// If a cluster size is 1, then all targets will be returned.
func (t *DistributedTargets) Get() []Target {
	// TODO(@tpaschalis): Make this into a single code-path to simplify logic.
	if !t.useClustering || t.cluster == nil {
		return t.targets
	}

	peerCount := len(t.cluster.Peers())
	resCap := (len(t.targets) + 1)
	if peerCount != 0 {
		resCap = (len(t.targets) + 1) / peerCount
	}

	res := make([]Target, 0, resCap)

	for _, tgt := range t.targets {
		peers, err := t.cluster.Lookup(shard.StringKey(tgt.NonMetaLabels().String()), 1, shard.OpReadWrite)
		if err != nil {
			// This can only fail in case we ask for more owners than the
			// available peers. This will never happen, but in any case we fall
			// back to owning the target ourselves.
			res = append(res, tgt)
		}
		if len(peers) == 0 || peers[0].Self {
			res = append(res, tgt)
		}
	}

	return res
}

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
	Targets []Target `river:"targets,attr"`
}

// Discoverer is an alias for Prometheus' Discoverer interface, so users of this package don't need
// to import github.com/prometheus/prometheus/discover as well.
type Discoverer discovery.Discoverer

// Creator is a function provided by an implementation to create a concrete Discoverer instance.
type Creator func(component.Arguments) (Discoverer, error)

// Component is a reusable component for any discovery implementation.
// it will handle dynamic updates and exporting targets appropriately for a scrape implementation.
type Component struct {
	opts component.Options

	discMut       sync.Mutex
	latestDisc    discovery.Discoverer
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
	var cancel context.CancelFunc
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.newDiscoverer:
			// cancel any previously running discovery
			if cancel != nil {
				cancel()
			}
			// create new context so we can cancel it if we get any future updates
			// since it is derived from the main run context, it only needs to be
			// canceled directly if we receive new updates
			newCtx, cancelFunc := context.WithCancel(ctx)
			cancel = cancelFunc

			// finally run discovery
			c.discMut.Lock()
			disc := c.latestDisc
			c.discMut.Unlock()
			go c.runDiscovery(newCtx, disc)
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	disc, err := c.creator(args)
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
func (c *Component) runDiscovery(ctx context.Context, d Discoverer) {
	// all targets we have seen so far
	cache := map[string]*targetgroup.Group{}

	ch := make(chan []*targetgroup.Group)
	go d.Run(ctx, ch)

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
			send()
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
