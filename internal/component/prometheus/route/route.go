// Package route implements the prometheus.route component.
package route

import (
	"context"
	"fmt"
	"sync"

	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/promql/parser"
	"github.com/prometheus/prometheus/storage"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service/labelstore"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.route",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},
		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments configures the destinations for matching and unmatched series.
type Arguments struct {
	DefaultForwardTo []storage.AppendableV2 `alloy:"default_forward_to,attr"`
	Routes           []Route                `alloy:"route,block,optional"`
}

// Route forwards series matching Selector to every entry in ForwardTo.
// TODO: Revisit named route blocks if we add per-route throughput metrics or
// diagnostics. Names aren't needed for routing, and routes are currently unnamed.
type Route struct {
	Selector  string                 `alloy:"selector,attr"`
	ForwardTo []storage.AppendableV2 `alloy:"forward_to,attr"`
}

// Validate implements syntax.Validator.
func (a Arguments) Validate() error {
	_, err := compile(a)
	return err
}

// Exports holds the stable receiver exported by prometheus.route.
type Exports struct {
	Receiver storage.AppendableV2 `alloy:"receiver,attr"`
}

// receiverRange identifies entries in routingConfig.receivers. Entries are not
// deduplicated, even when several routes refer to the same receiver.
type receiverRange struct {
	start, end int
}

type compiledRoute struct {
	selector  labels.Selector
	receivers receiverRange
}

// routingConfig is immutable once published. Each appender retains a snapshot
// so updates cannot change its routing decisions before commit or rollback.
type routingConfig struct {
	routes           []compiledRoute
	defaultReceivers receiverRange
	receivers        []storage.Appendable
}

func compile(args Arguments) (*routingConfig, error) {
	cfg := &routingConfig{}
	p := parser.NewParser(parser.Options{})
	for i, route := range args.Routes {
		if len(route.ForwardTo) == 0 {
			return nil, fmt.Errorf("route %d: forward_to must contain at least one receiver", i+1)
		}
		matchers, err := p.ParseMetricSelector(route.Selector)
		if err != nil {
			return nil, fmt.Errorf("route %d: invalid selector: %w", i+1, err)
		}
		receivers, err := cfg.addReceivers(route.ForwardTo)
		if err != nil {
			return nil, fmt.Errorf("route %d: %w", i+1, err)
		}
		cfg.routes = append(cfg.routes, compiledRoute{selector: matchers, receivers: receivers})
	}
	var err error
	cfg.defaultReceivers, err = cfg.addReceivers(args.DefaultForwardTo)
	if err != nil {
		return nil, fmt.Errorf("default_forward_to: %w", err)
	}
	return cfg, nil
}

func (cfg *routingConfig) addReceivers(receivers []storage.AppendableV2) (receiverRange, error) {
	r := receiverRange{start: len(cfg.receivers)}
	for _, receiver := range receivers {
		// Like prometheus.Fanout, tolerate receivers which aren't available yet.
		if receiver == nil {
			continue
		}
		// NOTE(appenderv2 migration): production receivers still use the V1
		// append path. Keep this aligned with prometheus.Fanout until the V2
		// migration lands. See https://github.com/grafana/alloy/issues/6896.
		v1, ok := receiver.(storage.Appendable)
		if !ok {
			return receiverRange{}, fmt.Errorf("receiver %T does not implement storage.Appendable", receiver)
		}
		cfg.receivers = append(cfg.receivers, v1)
	}
	r.end = len(cfg.receivers)
	return r, nil
}

// Component implements prometheus.route.
type Component struct {
	opts component.Options

	mut sync.RWMutex
	cfg *routingConfig

	receiver      *prometheus.Interceptor
	fanout        *prometheus.Fanout
	useLabelStore bool
	exited        atomic.Bool

	// nextRef issues router-local refs when the label store is disabled. These
	// let upstream scrapers track staleness without exposing downstream refs.
	// It isn't reset on updates, and doesn't require a per-series lookup table.
	nextRef atomic.Uint64

	processed prom.Counter
	forwarded prom.Counter
	dropped   prom.Counter
}

var (
	_ component.Component  = (*Component)(nil)
	_ storage.Appendable   = (*Component)(nil)
	_ storage.AppendableV2 = (*Component)(nil)
)

// New creates a prometheus.route component.
func New(opts component.Options, args Arguments) (*Component, error) {
	cfg, err := compile(args)
	if err != nil {
		return nil, err
	}
	data, err := opts.GetServiceData(labelstore.ServiceName)
	if err != nil {
		return nil, err
	}
	ls := data.(labelstore.LabelStore)
	c := &Component{
		opts:          opts,
		cfg:           cfg,
		useLabelStore: ls.Enabled(),
		processed: prom.NewCounter(prom.CounterOpts{
			Name: "alloy_prometheus_route_samples_processed_total",
			Help: "Total number of float and histogram samples processed.",
		}),
		forwarded: prom.NewCounter(prom.CounterOpts{
			Name: "alloy_prometheus_route_samples_forwarded_total",
			Help: "Total number of successful float and histogram sample appends to downstream receivers, including duplicates.",
		}),
		dropped: prom.NewCounter(prom.CounterOpts{
			Name: "alloy_prometheus_route_samples_dropped_total",
			Help: "Total number of float and histogram samples with no available destination.",
		}),
	}
	for _, collector := range []prom.Collector{c.processed, c.forwarded, c.dropped} {
		if err := opts.Registerer.Register(collector); err != nil {
			return nil, err
		}
	}

	// Keep the shared context, global-reference and staleness handling in
	// Fanout. Its single child selects destinations rather than broadcasting.
	c.fanout = prometheus.NewFanout([]storage.AppendableV2{c}, opts.ID, opts.Registerer, ls)
	c.receiver = prometheus.NewInterceptor(c.fanout, prometheus.WithComponentID(opts.ID))
	opts.OnStateChange(Exports{Receiver: c.receiver})
	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer c.exited.Store(true)
	defer c.fanout.Clear()
	<-ctx.Done()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	cfg, err := compile(args.(Arguments))
	if err != nil {
		return err
	}
	c.mut.Lock()
	c.cfg = cfg
	c.mut.Unlock()
	return nil
}

// Appender implements storage.Appendable.
func (c *Component) Appender(ctx context.Context) storage.Appender {
	c.mut.RLock()
	cfg := c.cfg
	c.mut.RUnlock()
	return &appender{
		component: c,
		ctx:       ctx,
		cfg:       cfg,
		children:  make([]storage.Appender, len(cfg.receivers)),
	}
}

// AppenderV2 implements storage.AppendableV2.
// TODO(appenderv2 migration): implement alongside prometheus.Fanout and
// prometheus.Interceptor. All production appends still use V1; see #6896.
func (c *Component) AppenderV2(context.Context) storage.AppenderV2 {
	panic("AppenderV2 not yet implemented for prometheus.route")
}
