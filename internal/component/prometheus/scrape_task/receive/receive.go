package receive

import (
	"context"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/discovery"
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

	mut  sync.RWMutex
	args Arguments
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
	// TODO(thampiotr): consume tasks from somewhere, e.g. redis queue and send them to the consumer

	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			level.Info(c.opts.Logger).Log("msg", "terminating due to context done")
			return nil
		case <-tick.C:
			level.Debug(c.opts.Logger).Log("msg", "generating fake scrape tasks and forwarding to consumers")

			c.mut.RLock()
			consumers := c.args.ForwardTo
			c.mut.RUnlock()

			// TODO(thampiotr): add metrics here

			for _, consumer := range consumers {
				consumer.Consume(fakeScrapeTasks())
			}
		}
	}
}

func fakeScrapeTasks() []scrape_task.ScrapeTask {
	return []scrape_task.ScrapeTask{
		{
			IssueTime: time.Now(),
			Target:    discovery.Target{"host": "test_host_1", "cluster": "home"},
		},
		{
			IssueTime: time.Now(),
			Target:    discovery.Target{"host": "test_host_2", "cluster": "home"},
		},
	}
}

// Update satisfies the Component interface.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.args = args.(Arguments)
	return nil
}
