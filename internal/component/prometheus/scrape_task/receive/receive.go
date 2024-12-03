package receive

import (
	"context"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.scrape_task.receive",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	ForwardTo []scrape_task.ScrapeTaskConsumer `alloy:"forward_to,attr"`
}

type Component struct {
	opts component.Options

	updateMut sync.RWMutex
	args      Arguments
}

func New(opts component.Options, args Arguments) (*Component, error) {
	c := &Component{
		opts: opts,
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run satisfies the Component interface.
func (c *Component) Run(ctx context.Context) error {
	// TODO(thampiotr): consume tasks from somewhere, e.g.
	<-ctx.Done()
	level.Info(c.opts.Logger).Log("msg", "terminating due to context done")
	return nil
}

// Update satisfies the Component interface.
func (c *Component) Update(args component.Arguments) error {
	newArgs := args.(Arguments)

	c.updateMut.Lock()
	defer c.updateMut.Unlock()

	c.args = newArgs
	return nil
}
