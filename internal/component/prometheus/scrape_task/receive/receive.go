package receive

import (
	"context"
	"sync"

	promclient "github.com/prometheus/client_golang/prometheus"

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
	opts         component.Options
	tasksCounter promclient.Counter

	mut  sync.RWMutex
	args Arguments
}

func New(opts component.Options, args Arguments) (*Component, error) {
	tasksCounter := promclient.NewCounter(promclient.CounterOpts{
		Name: "scrape_tasks_tasks_processed_total",
		Help: "Number of tasks the prometheus.scrape_task.receiver component has processed"})
	err := opts.Registerer.Register(tasksCounter)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:         opts,
		tasksCounter: tasksCounter,
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

			for _, consumer := range consumers {
				consumer.Consume(tasks)
			}
			c.tasksCounter.Add(float64(len(tasks)))
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
