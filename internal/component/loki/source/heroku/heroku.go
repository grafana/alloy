package heroku

import (
	"context"
	"fmt"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	fnet "github.com/grafana/alloy/internal/component/common/net"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
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
	metrics       *metrics                 // Metrics about Heroku entries.
	serverMetrics *util.UncheckedCollector // Metircs about the HTTP server managed by the component.

	mut  sync.RWMutex
	args Arguments

	fanout *loki.Fanout
	server *source.Server

	handler loki.LogsBatchReceiver
}

// New creates a new loki.source.heroku component.
func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts:          o,
		metrics:       newMetrics(o.Registerer),
		mut:           sync.RWMutex{},
		args:          Arguments{},
		fanout:        loki.NewFanout(args.ForwardTo),
		handler:       loki.NewLogsBatchReceiver(),
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
		if c.server != nil {
			c.server.ForceShutdown()
		}
	}()

	loki.ConsumeBatch(ctx, c.handler, c.fanout)
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	c.fanout.UpdateChildren(newArgs.ForwardTo)

	if newArgs.Server == nil {
		newArgs.Server = &fnet.ServerConfig{}
	}

	if c.server.NeedsRestart(newArgs.Server) {
		if c.server != nil {
			c.server.Shutdown()
		}

		// [ht.NewHerokuTarget] registers new metrics every time it is called. To
		// avoid issues with re-registering metrics with the same name, we create a
		// new registry for the target every time we create one, and pass it to an
		// unchecked collector to bypass uniqueness checking.
		registry := prometheus.NewRegistry()
		c.serverMetrics.SetCollector(registry)

		server, err := source.NewServer(c.opts.Logger, registry, c.handler, source.ServerConfig{
			Namespace: "loki_source_heroku_drain_target",
			NetConfig: newArgs.Server,
			LogsConfig: &source.LogsConfig{
				FixedLabels:          newArgs.labelSet(),
				RelabelRules:         alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelRules),
				UseIncomingTimestamp: newArgs.UseIncomingTimestamp,
			},
		})

		if err != nil {
			return fmt.Errorf("failed to create heroku server: %w", err)
		}

		if err := server.Run(newRoutes(c.opts.Logger, c.metrics), []source.HandlerRoute{newHealthyHandler()}); err != nil {
			return fmt.Errorf("failed to run heroku server: %w", err)
		}

		c.server = server
		return nil
	}

	if c.server != nil {
		c.server.Update(&source.LogsConfig{
			FixedLabels:          newArgs.labelSet(),
			RelabelRules:         alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelRules),
			UseIncomingTimestamp: newArgs.UseIncomingTimestamp,
		})
	}

	c.args = newArgs

	return nil
}

func (args *Arguments) labelSet() model.LabelSet {
	lbls := make(model.LabelSet, len(args.Labels))
	for k, v := range args.Labels {
		lbls[model.LabelName(k)] = model.LabelValue(v)
	}

	return lbls
}

// DebugInfo returns information about the status of listener.
func (c *Component) DebugInfo() any {
	c.mut.RLock()
	defer c.mut.RUnlock()

	if c.server == nil {
		return readerDebugInfo{}
	}

	return readerDebugInfo{
		Ready:   ready(c.server.HTTPAddr()),
		Address: c.server.HTTPAddr(),
	}
}

type readerDebugInfo struct {
	Ready   bool   `alloy:"ready,attr"`
	Address string `alloy:"address,attr"`
}
