package relabel

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/livedebugging"
	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.relabel",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.relabel
// component.
type Arguments struct {
	// Where the relabeled metrics should be forwarded to.
	ForwardTo []loki.LogsReceiver `alloy:"forward_to,attr"`

	// The relabelling rules to apply to each log entry before it's forwarded.
	RelabelConfigs []*alloy_relabel.Config `alloy:"rule,block,optional"`

	// The maximum number of items to hold in the component's LRU cache.
	MaxCacheSize int `alloy:"max_cache_size,attr,optional"`
}

// DefaultArguments provides the default arguments for the loki.relabel
// component.
var DefaultArguments = Arguments{
	MaxCacheSize: 10_000,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Exports holds values which are exported by the loki.relabel component.
type Exports struct {
	Receiver loki.LogsReceiver   `alloy:"receiver,attr"`
	Rules    alloy_relabel.Rules `alloy:"rules,attr"`
}

// Component implements the loki.relabel component.
type Component struct {
	opts    component.Options
	metrics *metrics

	mut      sync.RWMutex
	rcs      []*relabel.Config
	receiver loki.LogsReceiver
	fanout   []loki.LogsReceiver

	cache        *lru.Cache
	maxCacheSize int

	debugDataPublisher livedebugging.DebugDataPublisher
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// New creates a new loki.relabel component.
func New(o component.Options, args Arguments) (*Component, error) {
	cache, err := lru.New(args.MaxCacheSize)
	if err != nil {
		return nil, err
	}

	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:               o,
		metrics:            newMetrics(o.Registerer),
		cache:              cache,
		maxCacheSize:       args.MaxCacheSize,
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	// Create and immediately export the receiver which remains the same for
	// the component's lifetime.
	c.receiver = loki.NewLogsReceiver()
	o.OnStateChange(Exports{Receiver: c.receiver, Rules: args.RelabelConfigs})

	// Call to Update() to set the relabelling rules once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	componentID := livedebugging.ComponentID(c.opts.ID)
	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.receiver.Chan():
			c.metrics.entriesProcessed.Inc()
			lbls := c.relabel(entry)

			if c.debugDataPublisher.IsActive(componentID) {
				c.debugDataPublisher.Publish(componentID, fmt.Sprintf("entry: %s, labels: %s => %s", entry.Line, entry.Labels.String(), lbls.String()))
			}

			if len(lbls) == 0 {
				level.Debug(c.opts.Logger).Log("msg", "dropping entry after relabeling", "labels", entry.Labels.String())
				continue
			}

			c.metrics.entriesOutgoing.Inc()
			entry.Labels = lbls
			for _, f := range c.fanout {
				select {
				case <-ctx.Done():
					return nil
				case f.Chan() <- entry:
				}
			}
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	newRCS := alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelConfigs)
	if relabelingChanged(c.rcs, newRCS) {
		level.Debug(c.opts.Logger).Log("msg", "received new relabel configs, purging cache")
		c.cache.Purge()
		c.metrics.cacheSize.Set(0)
	}
	if newArgs.MaxCacheSize != c.maxCacheSize {
		evicted := c.cache.Resize(newArgs.MaxCacheSize)
		if evicted > 0 {
			level.Debug(c.opts.Logger).Log("msg", "resizing the cache lead to evicting of items", "len_items_evicted", evicted)
		}
	}
	c.rcs = newRCS
	c.fanout = newArgs.ForwardTo

	c.opts.OnStateChange(Exports{Receiver: c.receiver, Rules: newArgs.RelabelConfigs})

	return nil
}

func relabelingChanged(prev, next []*relabel.Config) bool {
	if len(prev) != len(next) {
		return true
	}
	for i := range prev {
		if !reflect.DeepEqual(prev[i], next[i]) {
			return true
		}
	}
	return false
}

type cacheItem struct {
	original  model.LabelSet
	relabeled model.LabelSet
}

// TODO(@tpaschalis) It's unfortunate how we have to cast back and forth
// between model.LabelSet (map) and labels.Labels (slice). Promtail does
// not have this issue as relabel config rules are only applied to targets.
// Do we want to use labels.Labels in loki.Entry instead?
func (c *Component) relabel(e loki.Entry) model.LabelSet {
	hash := e.Labels.Fingerprint()

	// Let's look in the cache for the hash of the entry's labels.
	val, found := c.cache.Get(hash)

	// We've seen this hash before; let's see if we've already relabeled this
	// specific entry before and can return early, or if it's a collision.
	if found {
		for _, ci := range val.([]cacheItem) {
			if e.Labels.Equal(ci.original) {
				c.metrics.cacheHits.Inc()
				return ci.relabeled
			}
		}
	}

	// Seems like it's either a new entry or a hash collision.
	c.metrics.cacheMisses.Inc()
	relabeled := c.process(e)

	// In case it's a new hash, initialize it as a new cacheItem.
	// If it was a collision, append the result to the cached slice.
	if !found {
		val = []cacheItem{{e.Labels, relabeled}}
	} else {
		val = append(val.([]cacheItem), cacheItem{e.Labels, relabeled})
	}

	c.cache.Add(hash, val)
	c.metrics.cacheSize.Set(float64(c.cache.Len()))

	return relabeled
}

func (c *Component) process(e loki.Entry) model.LabelSet {
	var lbls labels.Labels
	for k, v := range e.Labels {
		lbls = append(lbls, labels.Label{
			Name:  string(k),
			Value: string(v),
		})
	}
	lbls, _ = relabel.Process(lbls, c.rcs...)

	relabeled := make(model.LabelSet, len(lbls))
	for i := range lbls {
		relabeled[model.LabelName(lbls[i].Name)] = model.LabelValue(lbls[i].Value)
	}
	return relabeled
}

func (c *Component) LiveDebugging(_ int) {}
