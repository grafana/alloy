package receive

import (
	"context"
	"sync"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/queuestub"
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
	BatchSize int                              `alloy:"batch_size,attr,optional"`
}

func (a *Arguments) SetToDefault() {
	a.BatchSize = 100
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
	for {
		select {
		case <-ctx.Done():
			level.Info(c.opts.Logger).Log("msg", "terminating due to context done")
			return nil
		default:
			c.mut.RLock()
			batchSize := c.args.BatchSize
			c.mut.RUnlock()

			tasks := queuestub.PopTasks(batchSize)
			level.Debug(c.opts.Logger).Log("msg", "forwarding scrape tasks to consumers", "count", len(tasks))

			c.mut.RLock()
			consumers := c.args.ForwardTo
			c.mut.RUnlock()

			// TODO(thampiotr): add metrics here

			for _, consumer := range consumers {
				consumer.Consume(tasks)
			}
		}
	}
}

// Update satisfies the Component interface.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.args = args.(Arguments)
	return nil
}
