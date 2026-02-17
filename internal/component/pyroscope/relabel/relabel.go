// Package relabel provides label manipulation for Pyroscope profiles.
//
// Label Handling:
// The component handles two types of label representations:
// - labels.Labels ([]Label): Used by Pyroscope and relabeling logic
// - model.LabelSet (map[string]string): Used for efficient fingerprinting and cache lookups
//
// Cache Implementation:
// The cache uses model.LabelSet's fingerprinting to store label sets efficiently.
// Each cache entry contains both the original and relabeled labels to handle collisions
// and avoid recomputing relabeling rules.
package relabel

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	debuginfogrpc "buf.build/gen/go/parca-dev/parca/grpc/go/parca/debuginfo/v1alpha1/debuginfov1alpha1grpc"
	"github.com/grafana/alloy/internal/component/pyroscope/write/debuginfo"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.relabel",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	// Where the relabeled metrics should be forwarded to.
	ForwardTo []pyroscope.Appendable `alloy:"forward_to,attr"`
	// The relabelling rules to apply to each log entry before it's forwarded.
	RelabelConfigs []*alloy_relabel.Config `alloy:"rule,block,optional"`
	// The maximum number of items to hold in the component's LRU cache.
	MaxCacheSize int `alloy:"max_cache_size,attr,optional"`
}

// DefaultArguments provides the default arguments for the pyroscope.relabel
// component.
var DefaultArguments = Arguments{
	MaxCacheSize: 10_000,
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = DefaultArguments
}

// Exports holds values which are exported by the pyroscope.relabel component.
type Exports struct {
	Receiver pyroscope.Appendable `alloy:"receiver,attr"`
	Rules    alloy_relabel.Rules  `alloy:"rules,attr"`
}

// Component implements the pyroscope.relabel component.
type Component struct {
	opts         component.Options
	metrics      *metrics
	mut          sync.RWMutex
	rcs          []*relabel.Config
	fanout       *pyroscope.Fanout
	cache        *lru.Cache[model.Fingerprint, []cacheItem]
	maxCacheSize int
	exited       atomic.Bool
}

var (
	_ component.Component = (*Component)(nil)
)

// New creates a new pyroscope.relabel component.
func New(o component.Options, args Arguments) (*Component, error) {
	cache, err := lru.New[model.Fingerprint, []cacheItem](args.MaxCacheSize)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:         o,
		metrics:      newMetrics(o.Registerer),
		cache:        cache,
		maxCacheSize: args.MaxCacheSize,
	}

	c.fanout = pyroscope.NewFanout(args.ForwardTo, o.ID, o.Registerer)

	o.OnStateChange(Exports{
		Receiver: c,
		Rules:    args.RelabelConfigs,
	})

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

func (c *Component) Run(ctx context.Context) error {
	defer c.exited.Store(true)
	<-ctx.Done()
	return nil
}

func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	newRCS := alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelConfigs)

	// If relabeling rules changed, purge the cache
	if relabelingChanged(c.rcs, newRCS) {
		level.Debug(c.opts.Logger).Log("msg", "received new relabel configs, purging cache")
		c.cache.Purge()
		c.metrics.cacheSize.Set(0)
	}

	if newArgs.MaxCacheSize != c.maxCacheSize {
		evicted := c.cache.Resize(newArgs.MaxCacheSize)
		if evicted > 0 {
			level.Debug(c.opts.Logger).Log("msg", "resizing cache led to evicting items", "evicted_count", evicted)
		}
		c.maxCacheSize = newArgs.MaxCacheSize
	}

	c.rcs = newRCS
	c.fanout.UpdateChildren(newArgs.ForwardTo)

	c.opts.OnStateChange(Exports{
		Receiver: c,
		Rules:    newArgs.RelabelConfigs,
	})

	return nil
}

func (c *Component) Append(ctx context.Context, lbls labels.Labels, samples []*pyroscope.RawSample) error {
	if c.exited.Load() {
		return fmt.Errorf("%s has exited", c.opts.ID)
	}

	c.mut.RLock()
	defer c.mut.RUnlock()

	c.metrics.profilesProcessed.Inc()

	if lbls.IsEmpty() {
		c.metrics.profilesOutgoing.Inc()
		return c.fanout.Appender().Append(ctx, lbls, samples)
	}

	newLabels, keep := c.relabel(lbls)
	if !keep {
		c.metrics.profilesDropped.Inc()
		level.Debug(c.opts.Logger).Log("msg", "profile dropped by relabel rules", "labels", lbls.String())
		return nil
	}

	c.metrics.profilesOutgoing.Inc()
	return c.fanout.Appender().Append(ctx, newLabels, samples)
}

func (c *Component) AppendIngest(ctx context.Context, profile *pyroscope.IncomingProfile) error {
	if c.exited.Load() {
		return fmt.Errorf("%s has exited", c.opts.ID)
	}

	c.mut.RLock()
	defer c.mut.RUnlock()

	c.metrics.profilesProcessed.Inc()

	if profile.Labels.IsEmpty() {
		c.metrics.profilesOutgoing.Inc()
		return c.fanout.Appender().AppendIngest(ctx, profile)
	}

	newLabels, keep := c.relabel(profile.Labels)
	if !keep {
		c.metrics.profilesDropped.Inc()
		level.Debug(c.opts.Logger).Log("msg", "profile dropped by relabel rules")
		return nil
	}

	profile.Labels = newLabels
	c.metrics.profilesOutgoing.Inc()
	return c.fanout.Appender().AppendIngest(ctx, profile)
}

func (c *Component) Appender() pyroscope.Appender {
	return c
}

type cacheItem struct {
	original  model.LabelSet
	relabeled model.LabelSet
}

// relabel applies the configured relabeling rules to the input labels
// Returns the new labels, whether to keep the profile, and any error
func (c *Component) relabel(lbls labels.Labels) (labels.Labels, bool) {
	labelSet := toModelLabelSet(lbls)
	hash := labelSet.Fingerprint()

	// Check cache
	if result, keep, found := c.getCacheEntry(hash, labelSet); found {
		return result, keep
	}

	// Apply relabeling
	builder := labels.NewBuilder(lbls)
	keep := relabel.ProcessBuilder(builder, c.rcs...)
	if !keep {
		c.addToCache(hash, labelSet, labels.EmptyLabels())
		return labels.EmptyLabels(), false
	}

	newLabels := builder.Labels()

	// Cache result
	c.addToCache(hash, labelSet, newLabels)

	return newLabels, true
}

func (c *Component) getCacheEntry(hash model.Fingerprint, labelSet model.LabelSet) (labels.Labels, bool, bool) {
	if val, ok := c.cache.Get(hash); ok {
		for _, item := range val {
			if labelSet.Equal(item.original) {
				c.metrics.cacheHits.Inc()
				if len(item.relabeled) == 0 {
					return labels.Labels{}, false, true
				}
				return toLabelsLabels(item.relabeled), true, true
			}
		}
	}
	c.metrics.cacheMisses.Inc()
	return labels.Labels{}, false, false
}

func (c *Component) addToCache(hash model.Fingerprint, original model.LabelSet, relabeled labels.Labels) {
	var cacheValue []cacheItem
	if val, exists := c.cache.Get(hash); exists {
		cacheValue = val
	}
	cacheValue = append(cacheValue, cacheItem{
		original:  original,
		relabeled: toModelLabelSet(relabeled),
	})
	c.cache.Add(hash, cacheValue)
	c.metrics.cacheSize.Set(float64(c.cache.Len()))
}

// Helper function to detect relabel config changes
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

// toModelLabelSet converts labels.Labels to model.LabelSet
func toModelLabelSet(lbls labels.Labels) model.LabelSet {
	labelSet := make(model.LabelSet, lbls.Len())
	lbls.Range(func(l labels.Label) {
		labelSet[model.LabelName(l.Name)] = model.LabelValue(l.Value)
	})
	return labelSet
}

// toLabelsLabels converts model.LabelSet to labels.Labels
func toLabelsLabels(ls model.LabelSet) labels.Labels {
	result := labels.NewScratchBuilder(len(ls))
	for name, value := range ls {
		result.Add(string(name), string(value))
	}
	// Labels need to be sorted
	result.Sort()
	return result.Labels()
}

func (c *Component) Upload(j debuginfo.UploadJob) {
	c.fanout.Upload(j)
}

func (c *Component) Client() debuginfogrpc.DebuginfoServiceClient {
	return c.fanout.Client()
}
