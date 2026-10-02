package enrich

import (
	"context"
	"fmt"
	"sync"

	prometheus_client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	commonenrich "github.com/grafana/alloy/internal/component/common/enrich"
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

// matchCache holds the hash-based lookup for a match strategy.
type matchCache struct {
	matcher      *commonenrich.Matcher
	labelsToCopy []string // snapshot of which target labels to copy (empty means copy all)
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

	targetSet := mc.matcher.Match(lbls.Get)
	if targetSet == nil {
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

	matcher := commonenrich.NewMatcher(args.Targets, strategyMap)

	c.cacheMutex.Lock()
	c.mc = &matchCache{
		matcher:      matcher,
		labelsToCopy: args.LabelsToCopy,
	}
	c.cacheMutex.Unlock()

	c.cacheSize.Set(float64(matcher.Len()))
}
