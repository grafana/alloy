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

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// Component implements the loki.relabel component.
type Component struct {
	opts    component.Options
	metrics *metrics
	cache   *lru.Cache

	receiver    loki.LogsReceiver
	fanout      *loki.Fanout
	interceptor *loki.InterceptorConsumer

	mut          sync.RWMutex
	stopped      bool
	rcs          []*relabel.Config
	maxCacheSize int

	debugDataPublisher livedebugging.DebugDataPublisher
}

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
		fanout:             loki.NewFanout(args.ForwardTo),
		receiver:           loki.NewLogsReceiver(loki.WithComponentID(o.ID)),
	}

	c.interceptor = loki.NewInterceptorConsumer(
		o.ID,
		// FIXME(kalleep): Forward entries through consumer interface once we have migrated at pipeline level.
		// See https://github.com/grafana/alloy/issues/4953
		loki.NewNopConsumer(),
		func(ctx context.Context, batch loki.Batch) (loki.Batch, error) {
			c.mut.RLock()
			defer c.mut.RUnlock()

			if c.stopped {
				return loki.Batch{}, loki.ErrConsumerStopped
			}

			c.metrics.entriesProcessed.Add(float64(batch.EntryLen()))

			batch.FilterMapStreams(func(stream *loki.Stream) bool {
				relabeled, ok := c.relabel(stream.Labels)

				var count uint64
				if ok {
					count = uint64(len(stream.Entries))
				}

				c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
					livedebugging.ComponentID(c.opts.ID),
					livedebugging.LokiLog,
					count,
					func() string {
						if !ok {
							return fmt.Sprintf("stream: %s => <dropped>", stream.Labels.String())
						}
						return fmt.Sprintf("stream: %s => %s", stream.Labels.String(), relabeled.String())
					},
				))

				stream.Labels = relabeled
				return ok
			})

			c.metrics.entriesOutgoing.Add(float64(batch.EntryLen()))
			return batch, nil
		},
	)

	// Call to Update() to set the relabelling rules once at the start and export rules and receiver.
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

	loki.ConsumeAndProcess(ctx, c.receiver, c.fanout, func(entry loki.Entry) (loki.Entry, bool) {
		c.mut.RLock()
		defer c.mut.RUnlock()
		c.metrics.entriesProcessed.Inc()

		relabeled, ok := c.relabel(entry.Labels)
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
				return fmt.Sprintf("entry: %s, labels: %s => %s", entry.Line, entry.Labels.String(), relabeled.String())
			},
		))

		if !ok {
			c.opts.Logger.Debug("dropping entry after relabeling", "labels", entry.Labels.String())
			return loki.Entry{}, false
		}

		c.metrics.entriesOutgoing.Inc()
		entry.Labels = relabeled
		return entry, true
	})
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
		c.maxCacheSize = newArgs.MaxCacheSize
		evicted := c.cache.Resize(c.maxCacheSize)
		if evicted > 0 {
			c.opts.Logger.Debug("resizing the cache led to evicting items", "len_items_evicted", evicted)
		}
	}
	c.rcs = newRCS
	c.fanout.UpdateChildren(newArgs.ForwardTo)
	c.opts.OnStateChange(Exports{Receiver: c.receiver, Rules: newArgs.RelabelConfigs})

	return nil
}

type cacheItem struct {
	original  model.LabelSet
	relabeled model.LabelSet
}

// TODO(@tpaschalis) It's unfortunate how we have to cast back and forth
// between model.LabelSet (map) and labels.Labels (slice). Promtail does
// not have this issue as relabel config rules are only applied to targets.
// Do we want to use labels.Labels in loki.Entry instead?
func (c *Component) relabel(lset model.LabelSet) (model.LabelSet, bool) {
	hash := lset.Fingerprint()

	// Let's look in the cache for the hash of the entry's labels.
	val, found := c.cache.Get(hash)

	// We've seen this hash before; let's see if we've already relabeled this
	// specific entry before and can return early, or if it's a collision.
	if found {
		for _, ci := range val.([]cacheItem) {
			if lset.Equal(ci.original) {
				c.metrics.cacheHits.Inc()
				if len(ci.relabeled) == 0 {
					return nil, false
				}
				return ci.relabeled, true
			}
		}
	}

	// Seems like it's either a new entry or a hash collision.
	c.metrics.cacheMisses.Inc()
	relabeled := c.process(lset)

	// In case it's a new hash, initialize it as a new cacheItem.
	// If it was a collision, append the result to the cached slice.
	if !found {
		val = []cacheItem{{lset, relabeled}}
	} else {
		val = append(val.([]cacheItem), cacheItem{lset, relabeled})
	}

	c.cache.Add(hash, val)
	c.metrics.cacheSize.Set(float64(c.cache.Len()))

	if len(relabeled) == 0 {
		return nil, false
	}

	return relabeled, true
}

func (c *Component) process(lset model.LabelSet) model.LabelSet {
	br := labels.NewBuilder(labels.EmptyLabels())
	for k, v := range lset {
		br.Set(string(k), string(v))
	}

	if !relabel.ProcessBuilder(br, c.rcs...) {
		return nil
	}

	relabeled := make(model.LabelSet, len(lset))
	br.Range(func(lbl labels.Label) {
		relabeled[model.LabelName(lbl.Name)] = model.LabelValue(lbl.Value)
	})
	return relabeled
}

func (c *Component) LiveDebugging() {}

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
