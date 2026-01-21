package heroku

import (
	"context"
	"reflect"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	ht "github.com/grafana/alloy/internal/component/loki/source/heroku/internal/herokutarget"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.heroku",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments holds values which are used to configure the loki.source.heroku
// component.
type Arguments struct {
	Server               *fnet.ServerConfig  `alloy:",squash"`
	Labels               map[string]string   `alloy:"labels,attr,optional"`
	UseIncomingTimestamp bool                `alloy:"use_incoming_timestamp,attr,optional"`
	ForwardTo            []loki.LogsReceiver `alloy:"forward_to,attr"`
	RelabelRules         alloy_relabel.Rules `alloy:"relabel_rules,attr,optional"`
}

// SetToDefault implements syntax.Defaulter.
func (a *Arguments) SetToDefault() {
	*a = Arguments{
		Server: fnet.DefaultServerConfig(),
	}
}

// Component implements the loki.source.heroku component.
type Component struct {
	opts          component.Options
	metrics       *ht.Metrics              // Metrics about Heroku entries.
	serverMetrics *util.UncheckedCollector // Metircs about the HTTP server managed by the component.

	mut    sync.RWMutex
	args   Arguments
	fanout []loki.LogsReceiver
	target *ht.HerokuTarget

	handler loki.LogsReceiver
}

// New creates a new loki.source.heroku component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:          o,
		metrics:       ht.NewMetrics(o.Registerer),
		mut:           sync.RWMutex{},
		args:          Arguments{},
		fanout:        args.ForwardTo,
		target:        nil,
		handler:       loki.NewLogsReceiver(),
		serverMetrics: util.NewUncheckedCollector(nil),
	}

	o.Registerer.MustRegister(c.serverMetrics)

	// Call to Update() to start readers and set receivers once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.Lock()
		defer c.mut.Unlock()

		level.Info(c.opts.Logger).Log("msg", "loki.source.heroku component shutting down, stopping listener")
		if c.target != nil {
			err := c.target.Stop()
			if err != nil {
				level.Error(c.opts.Logger).Log("msg", "error while stopping heroku listener", "err", err)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return nil
		case entry := <-c.handler.Chan():
			c.mut.RLock()
			for _, receiver := range c.fanout {
				receiver.Chan() <- entry
			}
			c.mut.RUnlock()
		}
	}
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)
	c.fanout = newArgs.ForwardTo

	var rcs []*relabel.Config
	if len(newArgs.RelabelRules) > 0 {
		rcs = alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelRules)
	}

	restartRequired := changed(c.args.Server, newArgs.Server) ||
		changed(c.args.RelabelRules, newArgs.RelabelRules) ||
		changed(c.args.Labels, newArgs.Labels) ||
		c.args.UseIncomingTimestamp != newArgs.UseIncomingTimestamp
	if restartRequired {
		if c.target != nil {
			err := c.target.Stop()
			if err != nil {
				level.Error(c.opts.Logger).Log("msg", "error while stopping heroku listener", "err", err)
			}
		}

		// [ht.NewHerokuTarget] registers new metrics every time it is called. To
		// avoid issues with re-registering metrics with the same name, we create a
		// new registry for the target every time we create one, and pass it to an
		// unchecked collector to bypass uniqueness checking.
		registry := prometheus.NewRegistry()
		c.serverMetrics.SetCollector(registry)

		entryHandler := loki.NewEntryHandler(c.handler.Chan(), func() {})
		t, err := ht.NewHerokuTarget(c.metrics, c.opts.Logger, entryHandler, rcs, newArgs.Convert(), registry)
		if err != nil {
			level.Error(c.opts.Logger).Log("msg", "failed to create heroku listener with provided config", "err", err)
			return err
		}

		c.target = t
		c.args = newArgs
	}

	return nil
}

// Convert is used to bridge between the Alloy and Promtail types.
func (args *Arguments) Convert() *ht.HerokuDrainTargetConfig {
	lbls := make(model.LabelSet, len(args.Labels))
	for k, v := range args.Labels {
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}

	return &ht.HerokuDrainTargetConfig{
		Server:               args.Server,
		Labels:               lbls,
		UseIncomingTimestamp: args.UseIncomingTimestamp,
	}
}

// DebugInfo returns information about the status of listener.
func (c *Component) DebugInfo() any {
	c.mut.RLock()
	defer c.mut.RUnlock()

	var res = readerDebugInfo{
		Ready:   c.target.Ready(),
		Address: c.target.HTTPListenAddress(),
	}

	return res
}

type readerDebugInfo struct {
	Ready   bool   `alloy:"ready,attr"`
	Address string `alloy:"address,attr"`
}

func changed(prev, next any) bool {
	return !reflect.DeepEqual(prev, next)
}
