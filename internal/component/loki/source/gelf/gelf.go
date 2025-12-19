package gelf

import (
	"context"
	"sync"

	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/relabel"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/common/loki"
	alloy_relabel "github.com/grafana/alloy/internal/component/common/relabel"
	"github.com/grafana/alloy/internal/component/loki/source"
	"github.com/grafana/alloy/internal/component/loki/source/gelf/internal/target"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/loki/promtail/scrapeconfig"
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

// New creates a new gelf component.
func New(o component.Options, args Arguments) (*Component, error) {
	metrics := target.NewMetrics(o.Registerer)
	c := &Component{
		o:       o,
		metrics: metrics,
		handler: loki.NewLogsReceiver(),
		fanout:  loki.NewFanout(args.ForwardTo),
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
	o       component.Options
	metrics *target.Metrics
	handler loki.LogsReceiver

	mut    sync.RWMutex
	target *target.Target

	fanout *loki.Fanout
}

// Run starts the component.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		source.Drain(c.handler, func() {
			c.mut.Lock()
			defer c.mut.Unlock()
			if c.target != nil {
				c.target.Stop()
			}
		})

	}()

	source.ConsumeAndProccess(ctx, c.handler, c.fanout, func(e loki.Entry) loki.Entry {
		if e.Labels["job"] == "" {
			e.Labels["job"] = model.LabelValue(c.o.ID)
		}
		return e
	})

	return nil
}

// Update updates the fields of the component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	newArgs := args.(Arguments)

	c.fanout.UpdateChildren(newArgs.ForwardTo)

	if c.target != nil {
		c.target.Stop()
	}

	var rcs []*relabel.Config
	if len(newArgs.RelabelRules) > 0 {
		rcs = alloy_relabel.ComponentToPromRelabelConfigs(newArgs.RelabelRules)
	}

	t, err := target.NewTarget(c.metrics, c.o.Logger, c.handler, rcs, convertConfig(newArgs))
	if err != nil {
		return err
	}
	c.target = t

	return nil
}

// Arguments are the arguments for the component.
type Arguments struct {
	// ListenAddress only supports UDP.
	ListenAddress        string              `alloy:"listen_address,attr,optional"`
	UseIncomingTimestamp bool                `alloy:"use_incoming_timestamp,attr,optional"`
	RelabelRules         alloy_relabel.Rules `alloy:"relabel_rules,attr,optional"`
	ForwardTo            []loki.LogsReceiver `alloy:"forward_to,attr"`
}

func defaultArgs() Arguments {
	return Arguments{
		ListenAddress:        "0.0.0.0:12201",
		UseIncomingTimestamp: false,
	}
}

// SetToDefault implements syntax.Defaulter.
func (r *Arguments) SetToDefault() {
	*r = defaultArgs()
}

func convertConfig(a Arguments) *scrapeconfig.GelfTargetConfig {
	return &scrapeconfig.GelfTargetConfig{
		ListenAddress:        a.ListenAddress,
		Labels:               nil,
		UseIncomingTimestamp: a.UseIncomingTimestamp,
	}
}
