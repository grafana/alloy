package enrich

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/cespare/xxhash/v2"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/labelstore"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.enrich",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures the prometheus.enrich component.
type Arguments struct {
	// The targets to use for enrichment
	Targets []discovery.Target `alloy:"targets,attr"`

	// Multi-label matching: a map of target_label -> metric_label.
	// Takes precedence over target_match_label / metrics_match_label.
	TargetToMetricMatch map[string]string `alloy:"target_to_metric_match,attr,optional"`

	// Legacy: which label from targets to use for matching (e.g. "hostname", "ip").
	TargetMatchLabel string `alloy:"target_match_label,attr,optional"`

	// Legacy: which label from metrics to match against (e.g. "hostname", "ip").
	// If not specified, TargetMatchLabel will be used.
	MetricsMatchLabel string `alloy:"metrics_match_label,attr,optional"`

	// List of labels to copy from discovered targets to metrics. If empty, all labels will be copied.
	LabelsToCopy []string `alloy:"labels_to_copy,attr,optional"`

	ForwardTo []storage.Appendable `alloy:"forward_to,attr"`
}

// Validate implements syntax.Validator.
func (a Arguments) Validate() error {
	hasLegacy := a.TargetMatchLabel != "" || a.MetricsMatchLabel != ""
	hasNew := len(a.TargetToMetricMatch) > 0

	if !hasLegacy && !hasNew {
		return fmt.Errorf("at least one match mechanism must be specified: set target_match_label or target_to_metric_match")
	}
	// target_to_metric_match takes precedence when set; legacy fields are ignored.
	if hasLegacy && !hasNew && a.TargetMatchLabel == "" {
		return fmt.Errorf("target_match_label must be set when using legacy match fields")
	}
	return nil
}

// Exports holds values which are exported by the prometheus.enrich component.
type Exports struct {
	Receiver storage.Appendable `alloy:"receiver,attr"`
}

var sep = []byte{0xff} // separator to prevent hash collisions across value boundaries

// hashValuesFromLabelSet hashes the values of the given label names (in order)
// from a model.LabelSet. Returns (0, false) if names is empty or any label is
// missing or empty.
func hashValuesFromLabelSet(ls model.LabelSet, names []string) (uint64, bool) {
	if len(names) == 0 {
		return 0, false
	}
	h := xxhash.New()
	for _, name := range names {
		v := string(ls[model.LabelName(name)])
		if v == "" {
			return 0, false
		}
		_, _ = h.WriteString(v)
		_, _ = h.Write(sep)
	}
	return h.Sum64(), true
}

// hashValuesFromLabels hashes the values of the given label names (in order)
// from a labels.Labels. Returns (0, false) if names is empty or any label is
// missing or empty.
func hashValuesFromLabels(lbls labels.Labels, names []string) (uint64, bool) {
	if len(names) == 0 {
		return 0, false
	}
	h := xxhash.New()
	for _, name := range names {
		v := lbls.Get(name)
		if v == "" {
			return 0, false
		}
		_, _ = h.WriteString(v)
		_, _ = h.Write(sep)
	}
	return h.Sum64(), true
}

// matchCache holds the hash-based lookup for a match strategy.
type matchCache struct {
	sortedMetricLabels []string                  // metric label names to hash, sorted by corresponding target label name
	cache              map[uint64]model.LabelSet // hash of values -> target label set
	labelsToCopy       []string                  // snapshot of which target labels to copy (empty means copy all)
}

type Component struct {
	opts component.Options

	mut      sync.RWMutex
	receiver *prometheus.Interceptor
	fanout   *prometheus.Fanout
	exited   atomic.Bool

	mc         *matchCache
	cacheMutex sync.RWMutex

	cacheSize prometheus_client.Gauge
}

func New(opts component.Options, args Arguments) (*Component, error) {
	service, err := opts.GetServiceData(labelstore.ServiceName)
	if err != nil {
		return nil, err
	}
	ls := service.(labelstore.LabelStore)

	c := &Component{
		opts: opts,
	}

	c.cacheSize = prometheus_client.NewGauge(prometheus_client.GaugeOpts{
		Name: "alloy_prometheus_target_cache_size",
		Help: "Total size of targets cache",
	})

	for _, metric := range []prometheus_client.Collector{
		c.cacheSize,
	} {
		err = opts.Registerer.Register(metric)
		if err != nil {
			return nil, err
		}
	}

	c.fanout = prometheus.NewFanout(args.ForwardTo, opts.ID, opts.Registerer, ls)
	c.receiver = prometheus.NewInterceptor(
		c.fanout,
		prometheus.WithComponentID(c.opts.ID),
		prometheus.WithAppendHook(func(_ storage.SeriesRef, l labels.Labels, t int64, v float64, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", opts.ID)
			}

			newLabels := c.enrich(l)

			// Since SeriesRefs are tied to the labels, we send zero to indicate the seriesRef should be recalculated downstream.
			return next.Append(0, newLabels, t, v)
		}),
		prometheus.WithExemplarHook(func(_ storage.SeriesRef, l labels.Labels, e exemplar.Exemplar, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", opts.ID)
			}

			newLabels := c.enrich(l)

			// Since SeriesRefs are tied to the labels, we send zero to indicate the seriesRef should be recalculated downstream.
			return next.AppendExemplar(0, newLabels, e)
		}),
		prometheus.WithMetadataHook(func(_ storage.SeriesRef, l labels.Labels, m metadata.Metadata, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", opts.ID)
			}

			newLabels := c.enrich(l)

			// Since SeriesRefs are tied to the labels, we send zero to indicate the seriesRef should be recalculated downstream.
			return next.UpdateMetadata(0, newLabels, m)
		}),
		prometheus.WithHistogramHook(func(_ storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", opts.ID)
			}

			newLabels := c.enrich(l)

			return next.AppendHistogram(0, newLabels, t, h, fh)
		}),
	)

	if err = c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer c.exited.Store(true)
	defer c.fanout.Clear()

	<-ctx.Done()

	return nil
}

func (c *Component) Name() string {
	return "prometheus.enrich"
}

func (c *Component) Ready() bool {
	return true
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	c.fanout.UpdateChildren(newArgs.ForwardTo)

	c.refreshCacheFromTargets(newArgs)

	c.opts.OnStateChange(Exports{Receiver: c.receiver})

	return nil
}

func (c *Component) enrich(lbls labels.Labels) labels.Labels {
	c.cacheMutex.RLock()
	mc := c.mc
	c.cacheMutex.RUnlock()

	if mc == nil {
		return lbls
	}

	h, ok := hashValuesFromLabels(lbls, mc.sortedMetricLabels)
	if !ok {
		return lbls
	}
	targetSet, found := mc.cache[h]
	if !found {
		return lbls
	}

	newLabels := labels.NewBuilder(lbls.Copy())
	if len(mc.labelsToCopy) == 0 {
		for k, v := range targetSet {
			newLabels.Set(string(k), string(v))
		}
	} else {
		for _, label := range mc.labelsToCopy {
			if value, ok := targetSet[model.LabelName(label)]; ok {
				newLabels.Set(label, string(value))
			}
		}
	}
	return newLabels.Labels()
}

// sortStrategyMap converts a target_label->metric_label map into sorted
// parallel slices for deterministic hashing.
func sortStrategyMap(m map[string]string) (targetLabels, metricLabels []string) {
	targetLabels = make([]string, 0, len(m))
	for k := range m {
		targetLabels = append(targetLabels, k)
	}
	sort.Strings(targetLabels)

	metricLabels = make([]string, 0, len(targetLabels))
	for _, k := range targetLabels {
		metricLabels = append(metricLabels, m[k])
	}
	return targetLabels, metricLabels
}

func (c *Component) refreshCacheFromTargets(args Arguments) {
	// Build the strategy map: target_label -> metric_label.
	strategyMap := args.TargetToMetricMatch
	if len(strategyMap) == 0 && args.TargetMatchLabel != "" {
		// Legacy single-label mode: synthesize a strategy map.
		metricsLabel := args.MetricsMatchLabel
		if metricsLabel == "" {
			metricsLabel = args.TargetMatchLabel
		}
		strategyMap = map[string]string{args.TargetMatchLabel: metricsLabel}
	}

	sortedTargetLabels, sortedMetricLabels := sortStrategyMap(strategyMap)
	cache := make(map[uint64]model.LabelSet)
	for _, target := range args.Targets {
		labelSet := make(model.LabelSet)
		target.ForEachLabel(func(k, v string) bool {
			labelSet[model.LabelName(k)] = model.LabelValue(v)
			return true
		})
		h, ok := hashValuesFromLabelSet(labelSet, sortedTargetLabels)
		if !ok {
			continue
		}
		cache[h] = labelSet
	}

	c.cacheMutex.Lock()
	c.mc = &matchCache{
		sortedMetricLabels: sortedMetricLabels,
		cache:              cache,
		labelsToCopy:       args.LabelsToCopy,
	}
	c.cacheMutex.Unlock()

	c.cacheSize.Set(float64(len(cache)))
}
