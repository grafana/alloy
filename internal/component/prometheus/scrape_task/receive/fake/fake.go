package receive

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/grafana/ckit/shard"
	promclient "github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/faketasks"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
	"github.com/grafana/alloy/internal/service/cluster"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.scrape_task.receive.fake",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	ForwardTo      []scrape_task.ScrapeTaskConsumer `alloy:"forward_to,attr"`
	TargetsCount   int                              `alloy:"targets_count,attr,optional"`
	ScrapeInterval time.Duration                    `alloy:"scrape_interval,attr,optional"`
}

func (a *Arguments) SetToDefault() {
	a.TargetsCount = 100
	a.ScrapeInterval = time.Minute
}

type Component struct {
	opts             component.Options
	tasksCounter     promclient.Counter
	taskLag          promclient.Histogram
	cluster          cluster.Cluster
	fakeTaskProvider scrape_task.ScrapeTaskProvider

	mut  sync.RWMutex
	args Arguments
}

func New(opts component.Options, args Arguments) (*Component, error) {
	tasksCounter := promclient.NewCounter(promclient.CounterOpts{
		Name: "scrape_tasks_fake_tasks_processed_total",
		Help: "Number of tasks the prometheus.scrape_task.receiver.fake component has processed"})
	err := opts.Registerer.Register(tasksCounter)
	if err != nil {
		return nil, err
	}

	taskLag := promclient.NewHistogram(promclient.HistogramOpts{
		Name:    "scrape_tasks_fake_tasks_lag_seconds",
		Help:    "The time a task spends waiting to be picked up",
		Buckets: []float64{0.5, 1, 5, 10, 30, 60, 90, 180, 300, 600},
	})
	err = opts.Registerer.Register(taskLag)
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
		taskLag:      taskLag,
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
			consumers := c.args.ForwardTo
			taskProvider := c.fakeTaskProvider
			c.mut.RUnlock()

			// This will block for scrape interval until tasks are available.
			tasks := taskProvider.Get()
			// Like our current stable cluster sharding - pick only targets that belong to local instance.
			tasks = c.filterTasksWeOwn(tasks)

			// Fan out the tasks
			level.Debug(c.opts.Logger).Log("msg", "forwarding scrape tasks to consumers", "count", len(tasks))
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
		// Extract host label from target. This is a hack for demo purposes.
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

	c.fakeTaskProvider = faketasks.NewProvider(
		c.args.ScrapeInterval,
		c.args.TargetsCount,
		func(duration time.Duration) {
			c.taskLag.Observe(duration.Seconds())
		},
	)

	return nil
}
