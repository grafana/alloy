package expose

import (
	"context"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"

	"github.com/grafana/alloy/internal/component"
	alloy_prometheus "github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
)

const (
	name = "prometheus.expose"
)

func init() {
	component.Register(component.Registration{
		Name:      name,
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds the arguments for the prometheus.expose component
type Arguments struct {
	// Optional: namespace prefix for metrics
	Namespace string `alloy:"namespace,attr,optional"`

	// Optional: subsystem prefix for metrics
	Subsystem string `alloy:"subsystem,attr,optional"`

	// Optional: global labels to add to all metrics
	Labels map[string]string `alloy:"labels,attr,optional"`
}

// Exports holds the exports of the prometheus.expose component
type Exports struct {
	// Receiver is a Prometheus Appendable that can be used as a forward_to target
	Receiver storage.Appendable `alloy:"receiver,attr"`
}

// Component implements the prometheus.expose component
type Component struct {
	opts component.Options
	mut  sync.RWMutex
	args Arguments

	// collector wraps metrics for exposition
	collector *MetricsCollector

	// registerer for registering the collector
	registerer prometheus.Registerer
}

// New creates a new prometheus.expose component
func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:       opts,
		args:       args,
		collector:  NewMetricsCollector(args.Namespace, args.Subsystem, args.Labels),
		registerer: prometheus.DefaultRegisterer, // must use global registry to appear at /metrics
	}

	// Register the collector with the component's registerer
	err := c.registerer.Register(c.collector)
	if err != nil {
		return nil, err
	}

	// Create an interceptor that passes metrics through and stores them
	receiver := alloy_prometheus.NewInterceptor(
		noop{}, // Use a no-op appendable since we're just collecting
		alloy_prometheus.WithAppendHook(func(ref storage.SeriesRef, l labels.Labels, t int64, v float64, next storage.Appender) (storage.SeriesRef, error) {
			c.collector.AppendMetric(l, t, v)
			return ref, nil
		}),
	)

	opts.OnStateChange(Exports{Receiver: receiver})
	return c, nil
}

// Update implements component.Component
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	// If namespace, subsystem, or labels changed, recreate collector
	if newArgs.Namespace != c.args.Namespace ||
		newArgs.Subsystem != c.args.Subsystem ||
		!mapsEqual(newArgs.Labels, c.args.Labels) {

		// Unregister old collector
		c.registerer.Unregister(c.collector)

		// Create new collector
		c.collector = NewMetricsCollector(newArgs.Namespace, newArgs.Subsystem, newArgs.Labels)

		// Register new collector
		err := c.registerer.Register(c.collector)
		if err != nil {
			// Try to restore old collector; ignore error since we're already handling one.
			c.collector = NewMetricsCollector(c.args.Namespace, c.args.Subsystem, c.args.Labels)
			_ = c.registerer.Register(c.collector)
			return err
		}
	}

	c.args = newArgs
	return nil
}

// Run implements component.Component
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()

	// Unregister the collector when component stops
	c.registerer.Unregister(c.collector)
	return nil
}

// mapsEqual checks if two maps are equal
func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// noop is a no-op Appendable
type noop struct{}

func (n noop) Appender(ctx context.Context) storage.Appender {
	return noopAppender{}
}

type noopAppender struct{}

func (a noopAppender) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	return ref, nil
}

func (a noopAppender) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	return ref, nil
}

func (a noopAppender) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return ref, nil
}

func (a noopAppender) AppendSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64) (storage.SeriesRef, error) {
	return ref, nil
}

func (a noopAppender) AppendHistogramSTZeroSample(ref storage.SeriesRef, l labels.Labels, t, st int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return ref, nil
}

func (a noopAppender) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	return ref, nil
}

func (a noopAppender) SetOptions(_ *storage.AppendOptions) { /* no-op */ }

func (a noopAppender) Commit() error {
	return nil
}

func (a noopAppender) Rollback() error {
	return nil
}

// _ ensures noop implements storage.Appendable
var _ storage.Appendable = noop{}
