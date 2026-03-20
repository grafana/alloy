package gelf

import (
	"context"
	"sync"

	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source/gelf/internal/target"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/loki/promtail/scrapeconfig"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "loki.source.gelf",
		Stability: featuregate.StabilityGenerallyAvailable,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

// Arguments are the arguments for the component.
type Arguments struct {
	// ListenAddress only supports UDP.
	ListenAddress        string              `alloy:"listen_address,attr,optional"`
	UseIncomingTimestamp bool                `alloy:"use_incoming_timestamp,attr,optional"`
	RelabelRules         alloy_relabel.Rules `alloy:"relabel_rules,attr,optional"`
	ForwardTo            []loki.LogsReceiver `alloy:"forward_to,attr"`
}

// SetToDefault implements syntax.Defaulter.
func (r *Arguments) SetToDefault() {
	*r = Arguments{
		ListenAddress:        "0.0.0.0:12201",
		UseIncomingTimestamp: false,
	}
}

func convertConfig(a Arguments) *scrapeconfig.GelfTargetConfig {
	return &scrapeconfig.GelfTargetConfig{
		ListenAddress:        a.ListenAddress,
		UseIncomingTimestamp: a.UseIncomingTimestamp,
	}
}

// New creates a new gelf component.
func New(o component.Options, args Arguments) (*Component, error) {
	metrics := target.NewMetrics(o.Registerer)
	c := &Component{
		opts:    o,
		metrics: metrics,
		handler: loki.NewLogsReceiver(),
	}
	// Call to Update() to start readers and set receivers once at the start.
	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

var _ component.Component = (*Component)(nil)

// Component is a receiver for graylog formatted log files.
type Component struct {
	opts    component.Options
	metrics *target.Metrics
	handler loki.LogsReceiver
	fanout  *loki.Fanout

	mut    sync.Mutex
	target *target.Target
}

// Run starts the component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		level.Info(c.opts.Logger).Log("msg", "component shutting down")
		loki.Drain(c.handler, c.fanout, loki.DefaultDrainTimeout, func() {
			c.mut.Lock()
			defer c.mut.Unlock()

			if c.target != nil {
				c.target.Stop()
			}
		})
	}()

	loki.Consume(ctx, c.handler, c.fanout)
	return nil
}

// Update updates the fields of the component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	if c.target != nil {
		c.target.Stop()
	}

	c.fanout.UpdateChildren(newArgs.ForwardTo)

	var rcs []*relabel.Config
	if len(newArgs.RelabelRules) > 0 {
		rcs = alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelRules)
	}

	t, err := target.NewTarget(c.metrics, c.opts.Logger, c.handler, rcs, &scrapeconfig.GelfTargetConfig{
		ListenAddress:        newArgs.ListenAddress,
		UseIncomingTimestamp: newArgs.UseIncomingTimestamp,
	})
	if err != nil {
		return err
	}

	c.target = t
	return nil
}
