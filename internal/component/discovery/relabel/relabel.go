package relabel

import (
	"context"
	"fmt"
	"sync"

	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

func init() {
	component.Register(component.Registration{
		Name:      "discovery.relabel",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the discovery.relabel component.
type Arguments struct {
	// Targets contains the input 'targets' passed by a service discovery component.
	Targets []discovery.Target `alloy:"targets,attr"`

	// The relabelling rules to apply to each target's label set.
	RelabelConfigs []*alloy_relabel.Config `alloy:"rule,block,optional"`
}

// Exports holds values which are exported by the discovery.relabel component.
type Exports struct {
	Output []discovery.Target  `alloy:"output,attr"`
	Rules  alloy_relabel.Rules `alloy:"rules,attr"`
}

// Component implements the discovery.relabel component.
type Component struct {
	opts component.Options

	mut sync.RWMutex

	// cache memoises the relabeling result per target, keyed on the target's
	// packed labels. Guarded by mut.
	cache *targetCache
	// rules is a snapshot of the rules the cached results were produced with.
	rules []ruleSnapshot

	metrics *metrics

	debugDataPublisher livedebugging.DebugDataPublisher
}

var _ component.Component = (*Component)(nil)
var _ component.LiveDebugging = (*Component)(nil)

// New creates a new discovery.relabel component.
func New(o component.Options, args Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}
	c := &Component{
		opts:               o,
		cache:              newTargetCache(),
		metrics:            newMetrics(o.Registerer),
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	// Call to Update() to set the output once at the start
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	componentID := livedebugging.ComponentID(c.opts.ID)

	// With no rules every target passes through unchanged, so there is nothing
	// worth caching. Drop anything cached previously so that a config that removes
	// its rules does not keep the memory.
	if len(newArgs.RelabelConfigs) == 0 {
		if c.cache.len() > 0 {
			c.cache.clear()
		}
		c.rules = nil
		c.metrics.cacheSize.Set(0)

		targets := make([]discovery.Target, len(newArgs.Targets))
		copy(targets, newArgs.Targets)
		for _, t := range targets {
			c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
				componentID,
				livedebugging.Target,
				1,
				func() string { return fmt.Sprintf("%s => %s", t, t) },
			))
		}
		c.opts.OnStateChange(Exports{Output: targets, Rules: newArgs.RelabelConfigs})
		return nil
	}

	// Cached results are only valid for the rules that produced them.
	rules := snapshotRules(newArgs.RelabelConfigs)
	if rulesChanged(c.rules, rules) {
		c.cache.clear()
		c.rules = rules
	}

	targets := make([]discovery.Target, 0, len(newArgs.Targets))

	c.cache.begin()
	var hits, misses int
	for _, t := range newArgs.Targets {
		key := t.CacheKey()
		entry, cached := c.cache.lookup(key)
		if cached {
			hits++
		} else {
			misses++
			var (
				relabelled discovery.Target
				builder    = discovery.NewTargetBuilderFrom(t)
				keep       = alloy_relabel.ProcessBuilder(builder, newArgs.RelabelConfigs...)
			)
			if keep {
				relabelled = builder.Target()
			}
			entry = c.cache.insert(key, relabelled, keep)
		}

		if entry.keep {
			targets = append(targets, entry.output)
		}

		// Capture the entry rather than the loop variable so that the message is
		// built from this target's result if live debugging is active.
		e := entry
		c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
			componentID,
			livedebugging.Target,
			1,
			func() string {
				var relabelled discovery.Target
				if e.keep {
					relabelled = e.output
				}
				return fmt.Sprintf("%s => %s", t, relabelled)
			},
		))
	}
	c.cache.end()

	c.metrics.cacheHits.Add(float64(hits))
	c.metrics.cacheMisses.Add(float64(misses))
	c.metrics.cacheSize.Set(float64(c.cache.len()))

	c.opts.OnStateChange(Exports{
		Output: targets,
		Rules:  newArgs.RelabelConfigs,
	})

	return nil
}

func (c *Component) LiveDebugging() {}
