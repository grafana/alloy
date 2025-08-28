package mapping

import (
	"context"
	"fmt"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	prometheus_client "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"go.uber.org/atomic"
)

const name = "prometheus.mapping"

func init() {
	component.Register(component.Registration{
		Name:      name,
		Stability: featuregate.StabilityPublicPreview,
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

	// Labels to use for mapping
	SourceLabel string `alloy:"source_label,attr"`

	// Mapping
	LabelValuesMapping map[string]map[string]string `alloy:"mapping,attr"`
}

// SetToDefault implements syntax.Defaulter.
func (arg *Arguments) SetToDefault() {
}

// Validate implements syntax.Validator.
func (arg *Arguments) Validate() error {
	return nil
}

// Exports holds values which are exported by the prometheus.relabel component.
type Exports struct {
	Receiver storage.Appendable `alloy:"receiver,attr"`
}

// Component implements the prometheus.mapping component.
type Component struct {
	sourceLabel string

	mappings         map[string]map[string]string
	mut              sync.RWMutex
	opts             component.Options
	receiver         *prometheus.Interceptor
	metricsProcessed prometheus_client.Counter
	metricsOutgoing  prometheus_client.Counter
	fanout           *prometheus.Fanout
	exited           atomic.Bool
	ls               labelstore.LabelStore

	debugDataPublisher livedebugging.DebugDataPublisher
}

var (
	_ component.Component     = (*Component)(nil)
	_ component.LiveDebugging = (*Component)(nil)
)

// New creates a new prometheus.mapping component.
func New(o component.Options, args Arguments) (*Component, error) {
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
		ls:                 data.(labelstore.LabelStore),
		sourceLabel:        args.SourceLabel,
		mappings:           args.LabelValuesMapping,
		debugDataPublisher: debugDataPublisher.(livedebugging.DebugDataPublisher),
	}
	c.metricsProcessed = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "alloy_prometheus_mapping_metrics_processed",
		Help: "Total number of metrics processed",
	})
	c.metricsOutgoing = prometheus_client.NewCounter(prometheus_client.CounterOpts{
		Name: "alloy_prometheus_mapping_metrics_written",
		Help: "Total number of metrics written",
	})

	for _, metric := range []prometheus_client.Collector{c.metricsProcessed, c.metricsOutgoing} {
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

			newLbl := c.mapping(l)
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

			newLbl := c.mapping(l)
			if newLbl.IsEmpty() {
				return 0, nil
			}
			return next.AppendExemplar(0, newLbl, e)
		}),
		prometheus.WithMetadataHook(func(_ storage.SeriesRef, l labels.Labels, m metadata.Metadata, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", o.ID)
			}

			newLbl := c.mapping(l)
			if newLbl.IsEmpty() {
				return 0, nil
			}
			return next.UpdateMetadata(0, newLbl, m)
		}),
		prometheus.WithHistogramHook(func(_ storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram, next storage.Appender) (storage.SeriesRef, error) {
			if c.exited.Load() {
				return 0, fmt.Errorf("%s has exited", o.ID)
			}

			newLbl := c.mapping(l)
			if newLbl.IsEmpty() {
				return 0, nil
			}
			return next.AppendHistogram(0, newLbl, t, h, fh)
		}),
	)

	// Immediately export the receiver which remains the same for the component
	// lifetime.
	o.OnStateChange(Exports{Receiver: c.receiver})

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
	c.sourceLabel = newArgs.SourceLabel
	c.mappings = newArgs.LabelValuesMapping
	c.fanout.UpdateChildren(newArgs.ForwardTo)

	c.opts.OnStateChange(Exports{Receiver: c.receiver})

	return nil
}

func (c *Component) mapping(lbls labels.Labels) labels.Labels {
	c.metricsProcessed.Inc()
	// Relabel against a copy of the labels to prevent modifying the original
	// slice.
	lb := labels.NewBuilder(lbls.Copy())
	sourceValue := lb.Get(c.sourceLabel)
	for labelName, labelValue := range c.mappings[sourceValue] {
		lb.Set(labelName, labelValue)
	}
	newLabels := lb.Labels()

	componentID := livedebugging.ComponentID(c.opts.ID)
	if c.debugDataPublisher.IsActive(componentID) {
		c.debugDataPublisher.Publish(componentID, fmt.Sprintf("%s => %s", lbls.String(), newLabels.String()))
	}

	return newLabels
}

func (c *Component) LiveDebugging(_ int) {}
