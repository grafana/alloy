package receive

import (
	"context"
	"fmt"
	"sync"

	"github.com/grafana/ckit/shard"
	promclient "github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/queuestub"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
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
	ForwardTo                []scrape_task.ScrapeTaskConsumer `alloy:"forward_to,attr"`
	BatchSize                int                              `alloy:"batch_size,attr,optional"`
	SimulateStableAssignment bool                             `alloy:"simulate_stable_assignment,attr,optional"`
}

func (a *Arguments) SetToDefault() {
	a.BatchSize = 100
}

type Component struct {
	opts         component.Options
	tasksCounter promclient.Counter
	cluster      cluster.Cluster

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

	clusterSvc, err := opts.GetServiceData(cluster.ServiceName)
	if err != nil {
		return nil, fmt.Errorf("failed to get information about cluster: %w", err)
	}
	c := &Component{
		opts:         opts,
		tasksCounter: tasksCounter,
		cluster:      clusterSvc.(cluster.Cluster),
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
			stableAssignment := c.args.SimulateStableAssignment
			c.mut.RUnlock()

			tasks := queuestub.PopTasks(batchSize)
			if stableAssignment {
				// If we want to simulate stable assignment, we will only process tasks that belong to this instance
				// as determined by consistent hashing - this is similar to current clustering where targets are
				// distributed between instances.
				tasks = c.filterTasksWeOwn(tasks)
			}

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

func (c *Component) filterTasksWeOwn(tasks []scrape_task.ScrapeTask) []scrape_task.ScrapeTask {
	var newTasks []scrape_task.ScrapeTask
	for _, task := range tasks {
		// Extract host label from target
		sl := task.Target.SpecificLabels([]string{"host"})
		if len(sl) != 1 {
			fmt.Println("missing host label")
			return nil
		}
		host := sl[0].Value

		peers, err := c.cluster.Lookup(shard.StringKey(host), 1, shard.OpReadWrite)
		belongsToLocal := err != nil || len(peers) == 0 || peers[0].Self

		if belongsToLocal {
			newTasks = append(newTasks, task)
		}
	}
	return newTasks
}

// Update satisfies the Component interface.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.args = args.(Arguments)
	return nil
}
