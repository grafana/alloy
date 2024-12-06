package redis

import (
	"context"
	"encoding/json"
	"slices"
	"sync"
	"time"

	"github.com/hibiken/asynq"
	promclient "github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/faketasks"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.scrape_task.produce.redis",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	TargetsCount   int           `alloy:"targets_count,attr,optional"`
	ScrapeInterval time.Duration `alloy:"scrape_interval,attr,optional"`
	RedisAddress   string        `alloy:"redis_address,attr,optional"`
	BatchSize      int           `alloy:"batch_size,attr,optional"`
}

func (a *Arguments) SetToDefault() {
	a.TargetsCount = 100
	a.BatchSize = 10
	a.ScrapeInterval = time.Minute
	a.RedisAddress = "localhost:6379"
}

type Component struct {
	opts             component.Options
	fakeTaskProvider scrape_task.ScrapeTaskProvider
	taskLag          promclient.Histogram

	mut  sync.RWMutex
	args Arguments
}

func New(opts component.Options, args Arguments) (*Component, error) {
	taskLag := promclient.NewHistogram(promclient.HistogramOpts{
		Name:    "scrape_tasks_redis_tasks_write_lag_seconds",
		Help:    "The time a task spends waiting to be written to the queue",
		Buckets: []float64{0.5, 1, 5, 10, 30, 60, 90, 180, 300, 600},
	})
	err := opts.Registerer.Register(taskLag)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:    opts,
		taskLag: taskLag,
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
			taskProvider := c.fakeTaskProvider
			redisAddr := c.args.RedisAddress
			batchSize := c.args.BatchSize
			scrapeInterval := c.args.ScrapeInterval
			c.mut.RUnlock()

			// This will block for scrape interval until tasks are available.
			tasks := taskProvider.Get()

			// TODO(thampiotr): we should ideally keep the same client around until address changes, but for this demo
			//                  I'm avoiding the complexity of managing client updates.
			client := asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr})

			// Send in batches
			for batch := range slices.Chunk(tasks, batchSize) {
				payload, err := newScrapeTasksPayload(batch)
				if err != nil {
					level.Error(c.opts.Logger).Log("msg", "error creating scrape tasks payload", "err", err)
				}

				info, err := client.Enqueue(
					payload,
					asynq.MaxRetry(3),
					asynq.Queue("scrape_tasks"),
					asynq.Timeout(scrapeInterval),
					asynq.Unique(scrapeInterval*2),
				)
				if err != nil {
					level.Error(c.opts.Logger).Log("msg", "error enqueue scrape tasks", "err", err, "info", info)
				} else {
					level.Debug(c.opts.Logger).Log("msg", "scrape tasks enqueued", "count", len(batch), "id", info.ID, "queue", info.Queue)
				}
			}

			if err := client.Close(); err != nil {
				level.Warn(c.opts.Logger).Log("msg", "failed to close redis client", "err", err)
			}
		}
	}
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

func newScrapeTasksPayload(tasks []scrape_task.ScrapeTask) (*asynq.Task, error) {
	payload, err := json.Marshal(tasks)
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(scrape_task.TaskTypeScrapeTaskV1, payload), nil
}
