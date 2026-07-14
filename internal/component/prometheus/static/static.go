// Package static implements the prometheus.static component, which emits a
// fixed set of user-defined metrics to downstream Prometheus-compatible
// components.
package static

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/grafana/ckit/shard"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/cluster"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
)

const name = "prometheus.static"

func init() {
	component.Register(component.Registration{
		Name:      name,
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures the prometheus.static component.
type Arguments struct {
	// Prefix is prepended to every metric name, joined with an underscore.
	Prefix string `alloy:"prefix,attr,optional"`

	// Metrics is the set of static metrics to emit.
	Metrics []MetricConfig `alloy:"metric,block"`

	// Labels are attached to every emitted metric. A metric's own labels take
	// precedence when the two overlap.
	Labels map[string]string `alloy:"labels,block,optional"`

	// ScrapeInterval controls how often the metrics are emitted to the
	// forward_to receivers so that they remain fresh in downstream storage.
	ScrapeInterval time.Duration `alloy:"scrape_interval,attr,optional"`

	// Clustering, when enabled, makes only a single node in the cluster emit the
	// metrics, preventing duplicates across replicas.
	Clustering cluster.ComponentBlock `alloy:"clustering,block,optional"`

	// ForwardTo is the list of receivers the metrics are forwarded to.
	ForwardTo []storage.Appendable `alloy:"forward_to,attr"`
}

// MetricConfig defines a single static metric.
type MetricConfig struct {
	// Name is the metric name, before the component prefix is applied.
	Name string `alloy:"name,attr"`

	// Value is the sample value emitted for the metric. Defaults to 1, which is
	// the convention for info-style metrics.
	Value float64 `alloy:"value,attr,optional"`

	// Type is the metric type reported as metadata to downstream components.
	// Defaults to "unknown".
	Type string `alloy:"type,attr,optional"`

	// Help is an optional description reported as metadata to downstream
	// components.
	Help string `alloy:"help,attr,optional"`

	// Labels are attached to this metric only, taking precedence over the
	// component-level labels.
	Labels map[string]string `alloy:"labels,block,optional"`
}

// supportedMetricTypes are the metric types that can be reported as metadata.
// Only types that make sense for a single user-supplied float value are allowed.
var supportedMetricTypes = map[string]model.MetricType{
	"gauge":   model.MetricTypeGauge,
	"counter": model.MetricTypeCounter,
	"info":    model.MetricTypeInfo,
	"unknown": model.MetricTypeUnknown,
}

// DefaultArguments holds the default settings for the prometheus.static
// component.
var DefaultArguments = Arguments{
	ScrapeInterval: time.Minute,
}

// SetToDefault implements syntax.Defaulter.
func (args *Arguments) SetToDefault() {
	*args = DefaultArguments
}

// SetToDefault implements syntax.Defaulter.
func (m *MetricConfig) SetToDefault() {
	*m = MetricConfig{
		Value: 1,
		Type:  "unknown",
	}
}

// Validate implements syntax.Validator.
func (args *Arguments) Validate() error {
	if args.ScrapeInterval <= 0 {
		return fmt.Errorf("scrape_interval must be greater than 0; got %s", args.ScrapeInterval)
	}

	if len(args.Metrics) == 0 {
		return fmt.Errorf("at least one metric block must be defined")
	}

	if err := validateLabels(args.Labels); err != nil {
		return fmt.Errorf("labels: %w", err)
	}

	seen := make(map[string]struct{}, len(args.Metrics))
	for i, m := range args.Metrics {
		if m.Name == "" {
			return fmt.Errorf("metric[%d]: name must not be empty", i)
		}
		fullName := metricName(args.Prefix, m.Name)
		if !model.UTF8Validation.IsValidMetricName(fullName) {
			return fmt.Errorf("metric[%d]: %q is not a valid metric name", i, fullName)
		}
		if _, ok := seen[fullName]; ok {
			return fmt.Errorf("metric[%d]: duplicate metric name %q", i, fullName)
		}
		seen[fullName] = struct{}{}

		if _, ok := supportedMetricTypes[m.Type]; !ok {
			return fmt.Errorf("metric[%d]: unsupported type %q; must be one of gauge, counter, info, unknown", i, m.Type)
		}

		if err := validateLabels(m.Labels); err != nil {
			return fmt.Errorf("metric[%d]: %w", i, err)
		}
	}

	return nil
}

// validateLabels rejects label sets with invalid or reserved label names so
// that misconfigurations fail fast at load time instead of erroring on every
// emit.
func validateLabels(lbls map[string]string) error {
	for k := range lbls {
		if k == model.MetricNameLabel {
			return fmt.Errorf("label %q is reserved and can't be set; use the metric's name and prefix instead", model.MetricNameLabel)
		}
		if !model.UTF8Validation.IsValidLabelName(k) {
			return fmt.Errorf("%q is not a valid label name", k)
		}
	}
	return nil
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
	_ cluster.Component       = (*Component)(nil)
)

// Component implements the prometheus.static component.
type Component struct {
	opts    component.Options
	fanout  *prometheus.Fanout
	cluster cluster.Cluster

	mut               sync.RWMutex
	series            []staticSeries
	interval          time.Duration
	clusteringEnabled bool
	// reload signals Run that the config changed. It carries no value: Run reads
	// the current interval from c.interval under the mutex, so a coalesced
	// (dropped) signal never leaves Run ticking at a stale interval.
	reload chan struct{}
	// clusterChanged signals Run that cluster membership changed so it can
	// re-evaluate ownership and emit promptly, without resetting the ticker.
	clusterChanged chan struct{}

	metricsEmitted prometheus_client.Counter

	debugDataPublisher livedebugging.DebugDataPublisher
}

// staticSeries is a precomputed series ready to be appended.
type staticSeries struct {
	labels   labels.Labels
	value    float64
	metadata metadata.Metadata
}

// New creates a new prometheus.static component.
func New(o component.Options, args Arguments) (*Component, error) {
	debugDataPublisher, err := o.GetServiceData(livedebugging.ServiceName)
	if err != nil {
		return nil, err
	}

	data, err := o.GetServiceData(labelstore.ServiceName)
	if err != nil {
		return nil, err
	}
	ls := data.(labelstore.LabelStore)

	clusterData, err := o.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:               o,
		fanout:             prometheus.NewFanout(args.ForwardTo, o.ID, o.Registerer, ls),
		cluster:            clusterData.(cluster.Cluster),
		reload:             make(chan struct{}, 1),
		clusterChanged:     make(chan struct{}, 1),
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}

	c.metricsEmitted = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "alloy_prometheus_static_metrics_emitted_total",
		Help: "Total number of static metrics emitted to downstream components",
	})
	if err := o.Registerer.Register(c.metricsEmitted); err != nil {
		return nil, err
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer c.fanout.Clear()

	c.mut.RLock()
	interval := c.interval
	c.mut.RUnlock()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Emit immediately on startup so downstream sees the metrics without waiting
	// for the first tick.
	c.emit(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.emit(ctx)
		case <-c.reload:
			// Read the latest interval from shared state rather than the channel,
			// so a coalesced signal still applies the newest config.
			c.mut.RLock()
			interval = c.interval
			c.mut.RUnlock()
			ticker.Reset(interval)
			c.emit(ctx)
		case <-c.clusterChanged:
			// Cluster membership changed: re-evaluate ownership and emit promptly
			// so a newly-elected owner doesn't wait a full interval. The ticker is
			// left untouched to keep the regular cadence.
			c.emit(ctx)
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	series := buildSeries(newArgs)

	c.mut.Lock()
	c.series = series
	c.interval = newArgs.ScrapeInterval
	c.clusteringEnabled = newArgs.Clustering.Enabled
	c.mut.Unlock()

	c.fanout.UpdateChildren(newArgs.ForwardTo)

	// Notify Run that the config changed so it can reset its ticker and re-emit.
	// The buffered channel with a non-blocking send keeps this safe when Run has
	// not started yet (first Update from New) or is mid-emit. A dropped signal is
	// harmless: Run reads the latest interval from c.interval, and any pending
	// signal already guarantees a reset.
	select {
	case c.reload <- struct{}{}:
	default:
	}

	return nil
}

// NotifyClusterChange implements cluster.Component. It wakes Run so ownership is
// re-evaluated promptly after cluster membership changes.
func (c *Component) NotifyClusterChange() {
	c.mut.RLock()
	enabled := c.clusteringEnabled
	c.mut.RUnlock()
	if !enabled {
		return
	}

	select {
	case c.clusterChanged <- struct{}{}:
	default:
	}
}

// shouldEmit reports whether this node is responsible for emitting the metrics.
// When clustering is enabled, exactly one node in the cluster owns the
// component's key, so only that node emits and duplicates are avoided.
func (c *Component) shouldEmit() bool {
	c.mut.RLock()
	enabled := c.clusteringEnabled
	c.mut.RUnlock()
	if !enabled {
		return true
	}

	if !c.cluster.Ready() {
		// Avoid emitting from every node while the cluster is forming.
		// NotifyClusterChange re-triggers an emit once the cluster is ready.
		return false
	}

	peers, err := c.cluster.Lookup(shard.StringKey(c.opts.ID), 1, shard.OpReadWrite)
	if err != nil {
		// On lookup failure, bias toward availability and emit rather than risk a
		// gap in the metrics.
		c.opts.Logger.Warn("failed to determine cluster ownership, emitting anyway", "err", err)
		return true
	}
	return len(peers) > 0 && peers[0].Self
}

// emit appends the configured series to the fanout with the current timestamp.
// The context is threaded from Run so an in-flight emit is canceled on shutdown
// or config reload rather than blocking on a stalled downstream appender.
func (c *Component) emit(ctx context.Context) {
	if !c.shouldEmit() {
		return
	}

	c.mut.RLock()
	series := c.series
	c.mut.RUnlock()

	if len(series) == 0 {
		return
	}

	app := c.fanout.Appender(ctx)
	ts := time.Now().UnixMilli()
	for _, s := range series {
		ref, err := app.Append(0, s.labels, ts, s.value)
		if err != nil {
			_ = app.Rollback()
			c.opts.Logger.Error("failed to append static metric", "err", err)
			return
		}
		if _, err := app.UpdateMetadata(ref, s.labels, s.metadata); err != nil {
			_ = app.Rollback()
			c.opts.Logger.Error("failed to update static metric metadata", "err", err)
			return
		}
	}
	if err := app.Commit(); err != nil {
		c.opts.Logger.Error("failed to commit static metrics", "err", err)
		return
	}

	c.metricsEmitted.Add(float64(len(series)))

	componentID := livedebugging.ComponentID(c.opts.ID)
	c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
		componentID,
		livedebugging.PrometheusMetric,
		uint64(len(series)),
		func() string {
			var sb strings.Builder
			for i, s := range series {
				if i > 0 {
					sb.WriteByte('\n')
				}
				fmt.Fprintf(&sb, "%s => %g", s.labels.String(), s.value)
			}
			return sb.String()
		},
	))
}

// buildSeries precomputes the label sets for each configured metric.
func buildSeries(args Arguments) []staticSeries {
	series := make([]staticSeries, 0, len(args.Metrics))
	for _, m := range args.Metrics {
		b := labels.NewBuilder(labels.EmptyLabels())

		// Component-level labels first, then metric-level labels so the latter
		// win on conflict.
		for k, v := range args.Labels {
			b.Set(k, v)
		}
		for k, v := range m.Labels {
			b.Set(k, v)
		}
		b.Set(model.MetricNameLabel, metricName(args.Prefix, m.Name))

		series = append(series, staticSeries{
			labels: b.Labels(),
			value:  m.Value,
			metadata: metadata.Metadata{
				Type: supportedMetricTypes[m.Type],
				Help: m.Help,
			},
		})
	}
	return series
}

// metricName joins the prefix and name with an underscore when a prefix is set.
func metricName(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "_" + name
}

// LiveDebugging implements component.LiveDebugging.
func (c *Component) LiveDebugging() {}
