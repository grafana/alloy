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

	prometheus_client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
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
	// Defaults to "gauge".
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

	seen := make(map[string]struct{}, len(args.Metrics))
	for i, m := range args.Metrics {
		if m.Name == "" {
			return fmt.Errorf("metric[%d]: name must not be empty", i)
		}
		fullName := metricName(args.Prefix, m.Name)
		if !model.IsValidMetricName(model.LabelValue(fullName)) {
			return fmt.Errorf("metric[%d]: %q is not a valid metric name", i, fullName)
		}
		if _, ok := seen[fullName]; ok {
			return fmt.Errorf("metric[%d]: duplicate metric name %q", i, fullName)
		}
		seen[fullName] = struct{}{}

		if _, ok := supportedMetricTypes[m.Type]; !ok {
			return fmt.Errorf("metric[%d]: unsupported type %q; must be one of gauge, counter, info, unknown", i, m.Type)
		}
	}

	return nil
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// Component implements the prometheus.static component.
type Component struct {
	opts   component.Options
	fanout *prometheus.Fanout

	mut      sync.RWMutex
	series   []staticSeries
	interval time.Duration
	reload   chan time.Duration

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

	c := &Component{
		opts:               o,
		fanout:             prometheus.NewFanout(args.ForwardTo, o.ID, o.Registerer, ls),
		reload:             make(chan time.Duration, 1),
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
	c.emit()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			c.emit()
		case newInterval := <-c.reload:
			ticker.Reset(newInterval)
			c.emit()
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
	c.mut.Unlock()

	c.fanout.UpdateChildren(newArgs.ForwardTo)

	// Notify Run of the new interval so it can reset its ticker and re-emit. The
	// buffered channel with a non-blocking send keeps this safe when Run has not
	// started yet (first Update from New) or is mid-emit.
	select {
	case c.reload <- newArgs.ScrapeInterval:
	default:
	}

	return nil
}

// emit appends the configured series to the fanout with the current timestamp.
func (c *Component) emit() {
	c.mut.RLock()
	series := c.series
	c.mut.RUnlock()

	if len(series) == 0 {
		return
	}

	app := c.fanout.Appender(context.Background())
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
