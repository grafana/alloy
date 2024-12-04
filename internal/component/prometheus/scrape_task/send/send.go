package send

import (
	"context"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promadapter"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promstub"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.scrape_task.send",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	// TODO(thampiotr): Proper implementation will have all the configuration options required for the endpoint to
	// 					send the data to.
}

type Exports struct {
	Receiver scrape_task.MetricsConsumer `alloy:"receiver,attr"`
}

type Component struct {
	opts component.Options

	sender promadapter.Sender

	mut  sync.RWMutex
	args Arguments
}

var (
	_ component.Component = (*Component)(nil)
)

func New(o component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts: o,
		// TODO(thampiotr): for now using a stub, but the idea is to use a proper implementation that can remote write
		sender: promstub.NewSender(),
	}

	o.OnStateChange(Exports{Receiver: c})

	if err := c.Update(args); err != nil {
		return nil, err
	}

	return c, nil
}

// Run implements component.Component.
func (c *Component) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.args = args.(Arguments)
	return nil
}

func (c *Component) Consume(metrics []promadapter.Metrics) {
	// TODO(thampiotr): batch metrics differently here? Or is this responsibility of the sender? TBD
	err := c.sender.Send(metrics)
	if err != nil {
		level.Error(c.opts.Logger).Log("msg", "sending metrics failed", "err", err)
	}
	totalSeries := 0
	for _, m := range metrics {
		totalSeries += m.SeriesCount()
	}
	level.Debug(c.opts.Logger).Log("msg", "done sending metrics", "count", len(metrics), "total_series", totalSeries)
}
