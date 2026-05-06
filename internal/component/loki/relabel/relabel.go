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
	// Where the relabeled entries should be forwarded to.
	ForwardTo []loki.Consumer `alloy:"forward_to,attr"`

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
	Receiver loki.Consumer       `alloy:"receiver,attr"`
	Rules    alloy_relabel.Rules `alloy:"rules,attr"`
}

// Component implements the loki.relabel component.
type Component struct {
	opts    component.Options
	metrics *metrics

	receiver *loki.InterceptorConsumer
	fanout   *loki.FanoutConsumer

	mut     sync.RWMutex
	stopped bool
	rcs     []*relabel.Config

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
		fanout:             loki.NewFanoutConsumer(args.ForwardTo),
	}

	// Create and immediately export the receiver which remains the same for
	// the component's lifetime.
	c.receiver = loki.NewInterceptorConsumer(c.opts.ID, c.fanout, loki.WithConsumeEntryHook(func(ctx context.Context, entry loki.Entry) (loki.Entry, bool, error) {
		c.mut.RLock()
		defer c.mut.RUnlock()

		if c.stopped {
			return entry, false, loki.ErrConsumerStopped
		}

		relabeled, ok := c.relabel(entry)

		count := uint64(1)
		if !ok {
			count = 0
		}
		c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
			livedebugging.ComponentID(c.opts.ID),
			livedebugging.LokiLog,
			count,
			func() string {
				if !ok {
					return fmt.Sprintf("entry: %s, labels: %s => <dropped>", entry.Line, entry.Labels.String())
				}
				return fmt.Sprintf("entry: %s, labels: %s => %s", entry.Line, entry.Labels.String(), relabeled.Labels.String())
			},
		))

		if !ok {
			c.opts.Logger.Debug("dropping entry after relabeling", "labels", entry.Labels.String())
			return loki.Entry{}, false, nil
		}

		c.metrics.entriesOutgoing.Inc()
		return relabeled, true, nil
	}))

	o.OnStateChange(Exports{Receiver: c.receiver, Rules: args.RelabelConfigs})

	// Call to Update() to set the relabelling rules once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()
		c.stopped = true
	}()
	<-ctx.Done()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	newRCS := alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelConfigs)
	if relabelingChanged(c.rcs, newRCS) {
		c.opts.Logger.Debug("received new relabel configs, purging cache")
		c.cache.Purge()
		c.metrics.cacheSize.Set(0)
	}
	if newArgs.MaxCacheSize != c.maxCacheSize {
		evicted := c.cache.Resize(newArgs.MaxCacheSize)
		if evicted > 0 {
			c.opts.Logger.Debug("resizing the cache led to evicting items", "len_items_evicted", evicted)
		}
	}
	c.rcs = newRCS
	c.fanout.Update(newArgs.ForwardTo)

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
func (c *Component) relabel(e loki.Entry) (loki.Entry, bool) {
	c.metrics.entriesProcessed.Inc()

	hash := e.Labels.Fingerprint()

	// Let's look in the cache for the hash of the entry's labels.
	val, found := c.cache.Get(hash)

	// We've seen this hash before; let's see if we've already relabeled this
	// specific entry before and can return early, or if it's a collision.
	if found {
		for _, ci := range val.([]cacheItem) {
			if e.Labels.Equal(ci.original) {
				c.metrics.cacheHits.Inc()
				if len(ci.relabeled) == 0 {
					return loki.Entry{}, false
				}
				e.Labels = ci.relabeled
				return e, true
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

	if len(relabeled) == 0 {
		return loki.Entry{}, false
	}

	e.Labels = relabeled
	return e, true
}

var builderPool = sync.Pool{
	New: func() any {
		return labels.NewBuilder(labels.EmptyLabels())
	},
}

func (c *Component) process(e loki.Entry) model.LabelSet {
	builder := builderPool.Get().(*labels.Builder)
	defer func() {
		builder.Reset(labels.EmptyLabels())
		builderPool.Put(builder)
	}()

	for k, v := range e.Labels {
		builder.Set(string(k), string(v))
	}

	ok := relabel.ProcessBuilder(builder, c.rcs...)
	if !ok {
		return nil
	}

	lbls := builder.Labels()
	relabeled := make(model.LabelSet, lbls.Len())
	lbls.Range(func(lbl labels.Label) {
		relabeled[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})

	return relabeled
}

func (c *Component) LiveDebugging() {}
