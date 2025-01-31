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
	"sort"
	"sync"
	"sync/atomic"

	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/pyroscope"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/pyroscope/api/model/labelset"

	lru "github.com/hashicorp/golang-lru"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/relabel"
)

func init() {
	component.Register(component.Registration{
		Name:      "pyroscope.relabel",
		Stability: featuregate.StabilityPublicPreview,
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
	cache        *lru.Cache
	maxCacheSize int
	exited       atomic.Bool
}

var (
	_ component.Component = (*Component)(nil)
)

// New creates a new pyroscope.relabel component.
func New(o component.Options, args Arguments) (*Component, error) {
	cache, err := lru.New(args.MaxCacheSize)
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

	newLabels, keep, err := c.relabel(lbls)
	if err != nil {
		return err
	}
	if !keep {
		level.Debug(c.opts.Logger).Log("msg", "profile dropped by relabel rules", "labels", lbls.String())
		return nil
	}

	return c.fanout.Appender().Append(ctx, newLabels, samples)
}

func (c *Component) AppendIngest(ctx context.Context, profile *pyroscope.IncomingProfile) error {
	if c.exited.Load() {
		return fmt.Errorf("%s has exited", c.opts.ID)
	}

	c.mut.RLock()
	defer c.mut.RUnlock()

	if profile.Labels.IsEmpty() {
		return c.fanout.Appender().AppendIngest(ctx, profile)
	}

	newLabels, keep, err := c.relabel(profile.Labels)
	if err != nil {
		return fmt.Errorf("processing labels: %w", err)
	}
	if !keep {
		c.metrics.profilesDropped.Inc()
		level.Debug(c.opts.Logger).Log("msg", "profile dropped by relabel rules")
		return nil
	}

	profile.Labels = newLabels
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
func (c *Component) relabel(lbls labels.Labels) (labels.Labels, bool, error) {
	labelSet := toModelLabelSet(lbls)
	hash := labelSet.Fingerprint()

	// Check cache
	if result, keep, found := c.getCacheEntry(hash, labelSet); found {
		return result, keep, nil
	}

	// Apply relabeling
	// builder := labels.NewBuilder(lbls)
	builder := labels.NewBuilder(lbls)
	keep := relabel.ProcessBuilder(builder, c.rcs...)
	if !keep {
		c.metrics.profilesDropped.Inc()
		return labels.Labels{}, false, nil
	}

	newLabels := builder.Labels()

	// Cache result
	c.addToCache(hash, labelSet, newLabels)

	return newLabels, true, nil
}

func (c *Component) getCacheEntry(hash model.Fingerprint, labelSet model.LabelSet) (labels.Labels, bool, bool) {
	if val, ok := c.cache.Get(hash); ok {
		for _, item := range val.([]cacheItem) {
			if labelSet.Equal(item.original) {
				c.metrics.cacheHits.Inc()
				if len(item.relabeled) == 0 {
					c.metrics.profilesDropped.Inc()
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
		cacheValue = val.([]cacheItem)
	}
	cacheValue = append(cacheValue, cacheItem{
		original:  original,
		relabeled: toModelLabelSet(relabeled),
	})
	c.cache.Add(hash, cacheValue)
}

// extractLabelsFromIncomingProfile converts profile labels to Prometheus labels
func extractLabelsFromIncomingProfile(profile *pyroscope.IncomingProfile) (labels.Labels, error) {
	nameParam := profile.URL.Query().Get("name")
	if nameParam == "" {
		return labels.Labels{}, nil
	}

	ls, err := labelset.Parse(nameParam)
	if err != nil {
		return labels.Labels{}, fmt.Errorf("parsing labels from name parameter: %w", err)
	}

	var lbls []labels.Label
	for k, v := range ls.Labels() {
		lbls = append(lbls, labels.Label{Name: k, Value: v})
	}
	return labels.New(lbls...), nil
}

// updateProfileLabels converts Prometheus labels back to pyroscope format
func updateProfileLabels(profile *pyroscope.IncomingProfile, lbls labels.Labels) {
	newLS := labelset.New(make(map[string]string))
	for _, l := range lbls {
		newLS.Add(l.Name, l.Value)
	}

	query := profile.URL.Query()
	query.Set("name", newLS.Normalized())
	profile.URL.RawQuery = query.Encode()
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
	result := make(labels.Labels, 0, len(ls))
	for name, value := range ls {
		result = append(result, labels.Label{
			Name:  string(name),
			Value: string(value),
		})
	}
	// Labels need to be sorted
	sort.Sort(result)
	return result
}
