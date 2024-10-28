package relabel

import (
	"context"
	"fmt"
	"sync"

	"github.com/grafana/alloy/internal/component"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	lru "github.com/hashicorp/golang-lru/v2"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/model/relabel"
	"github.com/prometheus/prometheus/model/value"
	"github.com/prometheus/prometheus/storage"
	"go.uber.org/atomic"
)

const name = "prometheus.relabel"

func init() {
	component.Register(component.Registration{
		Name:      name,
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the prometheus.relabel
// component.
type Arguments struct {
	// Where the relabelled metrics should be forwarded to.
	ForwardTo []storage.Appendable `alloy:"forward_to,attr"`

	// The relabelling rules to apply to each metric before it's forwarded.
	MetricRelabelConfigs []*alloy_relabel.Config `alloy:"rule,block,optional"`

	// Cache size to use for LRU cache.
	CacheSize int `alloy:"max_cache_size,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (arg *Arguments) SetToDefault() {
	*arg = Arguments{
		CacheSize: 100_000,
	}
}

// Validate implements syntax.Validator.
func (arg *Arguments) Validate() error {
	if arg.CacheSize <= 0 {
		return fmt.Errorf("max_cache_size must be greater than 0 and is %d", arg.CacheSize)
	}
	return nil
}

// Exports holds values which are exported by the prometheus.relabel component.
type Exports struct {
	Receiver storage.Appendable  `alloy:"receiver,attr"`
	Rules    alloy_relabel.Rules `alloy:"rules,attr"`
}

// Component implements the prometheus.relabel component.
type Component struct {
	mut              sync.RWMutex
	opts             component.Options
	mrc              []*relabel.Config
	receiver         *prometheus.Interceptor
	metricsProcessed prometheus_client.Counter
	metricsOutgoing  prometheus_client.Counter
	cacheHits        prometheus_client.Counter
	cacheMisses      prometheus_client.Counter
	cacheSize        prometheus_client.Gauge
	cacheDeletes     prometheus_client.Counter
	fanout           *prometheus.Fanout
	exited           atomic.Bool
	ls               labelstore.LabelStore

	debugDataPublisher livedebugging.DebugDataPublisher

	cacheMut sync.RWMutex
	cache    *lru.Cache[uint64, *labelAndID]
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// New creates a new prometheus.relabel component.
func New(o component.Options, args Arguments) (*Component, error) {
	cache, err := lru.New[uint64, *labelAndID](args.CacheSize)
	if err != nil {
		return nil, err
	}

	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	data, err := o.GetServiceData(labelstore.ServiceName)
	if err != nil {
		return nil, err
	}
	c := &Component{
		opts:               o,
		cache:              cache,
		ls:                 data.(labelstore.LabelStore),
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}
	c.metricsProcessed = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "alloy_prometheus_relabel_metrics_processed",
		Help: "Total number of metrics processed",
	})
	c.metricsOutgoing = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "alloy_prometheus_relabel_metrics_written",
		Help: "Total number of metrics written",
	})
	c.cacheMisses = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "alloy_prometheus_relabel_cache_misses",
		Help: "Total number of cache misses",
	})
	c.cacheHits = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "alloy_prometheus_relabel_cache_hits",
		Help: "Total number of cache hits",
	})
	c.cacheSize = prometheus_client.NewGauge(prometheus_client.GaugeOpts{
		Name: "alloy_prometheus_relabel_cache_size",
		Help: "Total size of relabel cache",
	})
	c.cacheDeletes = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "alloy_prometheus_relabel_cache_deletes",
		Help: "Total number of cache deletes",
	})

	for _, metric := range []prometheus_client.Collector{c.metricsProcessed, c.metricsOutgoing, c.cacheMisses, c.cacheHits, c.cacheSize, c.cacheDeletes} {
		err = o.Registerer.Register(metric)
		if err != nil {
			return nil, err
		}
	}

	c.fanout = prometheus.NewFanout(args.ForwardTo, o.ID, o.Registerer, c.ls)
	c.receiver = prometheus.NewInterceptor(
		c.fanout,
		c.ls,
		prometheus.WithAppendHook(func(_ storage.SeriesRef, l labels.Labels, t int64, v float64, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", o.ID)
			}

			newLbl := c.relabel(v, l)
			if newLbl.IsEmpty() {
				return 0, nil
			}
			c.metricsOutgoing.Inc()
			return next.Append(0, newLbl, t, v)
		}),
		prometheus.WithExemplarHook(func(_ storage.SeriesRef, l labels.Labels, e exemplar.Exemplar, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", o.ID)
			}

			newLbl := c.relabel(0, l)
			if newLbl.IsEmpty() {
				return 0, nil
			}
			return next.AppendExemplar(0, newLbl, e)
		}),
		prometheus.WithMetadataHook(func(_ storage.SeriesRef, l labels.Labels, m metadata.Metadata, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", o.ID)
			}

			newLbl := c.relabel(0, l)
			if newLbl.IsEmpty() {
				return 0, nil
			}
			return next.UpdateMetadata(0, newLbl, m)
		}),
		prometheus.WithHistogramHook(func(_ storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", o.ID)
			}

			newLbl := c.relabel(0, l)
			if newLbl.IsEmpty() {
				return 0, nil
			}
			return next.AppendHistogram(0, newLbl, t, h, fh)
		}),
	)

	// Immediately export the receiver which remains the same for the component
	// lifetime.
	o.OnStateChange(Exports{Receiver: c.receiver, Rules: args.MetricRelabelConfigs})

	// Call to Update() to set the relabelling rules once at the start.
	if err = c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer c.exited.Store(true)

	<-ctx.Done()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	c.clearCache(newArgs.CacheSize)
	c.mrc = alloy_relabel.ComponentToPromRelabelConfigs(newArgs.MetricRelabelConfigs)
	c.fanout.UpdateChildren(newArgs.ForwardTo)

	c.opts.OnStateChange(Exports{Receiver: c.receiver, Rules: newArgs.MetricRelabelConfigs})

	return nil
}

func (c *Component) relabel(val float64, lbls labels.Labels) labels.Labels {
	c.mut.RLock()
	defer c.mut.RUnlock()

	globalRef := c.ls.GetOrAddGlobalRefID(lbls)
	var (
		relabelled labels.Labels
		keep       bool
	)
	newLbls, found := c.getFromCache(globalRef)
	if found {
		c.cacheHits.Inc()
		// If newLbls is nil but cache entry was found then we want to keep the value nil, if it's not we want to reuse the labels
		if newLbls != nil {
			relabelled = newLbls.labels
		}
	} else {
		// Relabel against a copy of the labels to prevent modifying the original
		// slice.
		relabelled, keep = relabel.Process(lbls.Copy(), c.mrc...)
		c.cacheMisses.Inc()
		c.addToCache(globalRef, relabelled, keep)
	}

	// If stale remove from the cache, the reason we don't exit early is so the stale value can propagate.
	// TODO: (@mattdurham) This caching can leak and likely needs a timed eviction at some point, but this is simple.
	// In the future the global ref cache may have some hooks to allow notification of when caches should be evicted.
	if value.IsStaleNaN(val) {
		c.deleteFromCache(globalRef)
	}
	// Set the cache size to the cache.len
	// TODO(@mattdurham): Instead of setting this each time could collect on demand for better performance.
	c.cacheSize.Set(float64(c.cache.Len()))

	componentID := livedebugging.ComponentID(c.opts.ID)
	if c.debugDataPublisher.IsActive(componentID) {
		c.debugDataPublisher.Publish(componentID, fmt.Sprintf("%s => %s", lbls.String(), relabelled.String()))
	}

	return relabelled
}

func (c *Component) getFromCache(id uint64) (*labelAndID, bool) {
	c.cacheMut.RLock()
	defer c.cacheMut.RUnlock()

	fm, found := c.cache.Get(id)
	return fm, found
}

func (c *Component) deleteFromCache(id uint64) {
	c.cacheMut.Lock()
	defer c.cacheMut.Unlock()
	c.cacheDeletes.Inc()
	c.cache.Remove(id)
}

func (c *Component) clearCache(cacheSize int) {
	c.cacheMut.Lock()
	defer c.cacheMut.Unlock()
	cache, _ := lru.New[uint64, *labelAndID](cacheSize)
	c.cache = cache
}

func (c *Component) addToCache(originalID uint64, lbls labels.Labels, keep bool) {
	c.cacheMut.Lock()
	defer c.cacheMut.Unlock()

	if !keep {
		c.cache.Add(originalID, nil)
		return
	}
	newGlobal := c.ls.GetOrAddGlobalRefID(lbls)
	c.cache.Add(originalID, &labelAndID{
		labels: lbls,
		id:     newGlobal,
	})
}

func (c *Component) LiveDebugging(_ int) {}

// labelAndID stores both the globalrefid for the label and the id itself. We store the id so that it doesn't have
// to be recalculated again.
type labelAndID struct {
	labels labels.Labels
	id     uint64
}
