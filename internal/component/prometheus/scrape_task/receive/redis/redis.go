package receive

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/hibiken/asynq"
	promclient "github.com/prometheus/client_golang/prometheus"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.scrape_task.receive.redis",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	ForwardTo    []scrape_task.ScrapeTaskConsumer `alloy:"forward_to,attr"`
	Concurrency  int                              `alloy:"concurrency,attr,optional"`
	RedisAddress string                           `alloy:"redis_address,attr,optional"`
}

func (a *Arguments) SetToDefault() {
	a.Concurrency = 5
	a.RedisAddress = "localhost:6379"
}

type Component struct {
	opts          component.Options
	tasksCounter  promclient.Counter
	server        *asynq.Server
	serverUpdated chan struct{}

	mut  sync.RWMutex
	args Arguments
}

func New(opts component.Options, args Arguments) (*Component, error) {
	tasksCounter := promclient.NewCounter(promclient.CounterOpts{
		Name: "scrape_tasks_redis_tasks_processed_total",
		Help: "Number of tasks the prometheus.scrape_task.receiver.redis component has processed"})
	err := opts.Registerer.Register(tasksCounter)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts:          opts,
		tasksCounter:  tasksCounter,
		serverUpdated: make(chan struct{}, 1),
	}

	if err := c.Update(args); err != nil {
		return nil, err
	}
	return c, nil
}

// Run satisfies the Component interface.
func (c *Component) Run(ctx context.Context) error {
	defer func() {
		c.mut.RLock()
		defer c.mut.RUnlock()
		if c.server != nil {
			c.server.Shutdown()
		}
	}()

	oneServerAtATime := sync.Mutex{}
	runServer := func() {
		c.mut.RLock()
		server := c.server
		c.mut.RUnlock()

		// Only allow one server running at a time
		oneServerAtATime.Lock()
		defer oneServerAtATime.Unlock()
		if err := server.Run(c.serverMux()); err != nil {
			level.Warn(c.opts.Logger).Log("msg", "server exit with error", "err", err)
		}
	}

	for {
		select {
		case <-ctx.Done():
			level.Info(c.opts.Logger).Log("msg", "terminating due to context done")
			return nil
		case <-c.serverUpdated:
			level.Info(c.opts.Logger).Log("msg", "starting new server after update")
			go runServer()
		}
	}
}

// Update satisfies the Component interface.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	defer c.mut.Unlock()
	newArgs := args.(Arguments)

	if c.serverNeedsUpdate(newArgs) {
		if c.server != nil {
			c.server.Shutdown()
		}

		srv := asynq.NewServer(
			asynq.RedisClientOpt{Addr: c.args.RedisAddress},
			asynq.Config{
				// Specify how many concurrent workers to use
				Concurrency: c.args.Concurrency,
				// Optionally specify multiple queues with different priority.
				Queues: map[string]int{
					"scrape_tasks": 10,
				},
			},
		)
		c.server = srv

		// Signal server is updated.
		select {
		case c.serverUpdated <- struct{}{}:
		default:
		}
	}

	c.args = newArgs

	return nil
}

func (c *Component) serverMux() *asynq.ServeMux {
	mux := asynq.NewServeMux()
	mux.Handle(scrape_task.TaskTypeScrapeTaskV1, c)
	return mux
}

func (c *Component) ProcessTask(ctx context.Context, t *asynq.Task) error {
	var tasks []scrape_task.ScrapeTask
	if err := json.Unmarshal(t.Payload(), &tasks); err != nil {
		return fmt.Errorf("json.Unmarshal failed: %v: %w", err, asynq.SkipRetry)
	}

	level.Info(c.opts.Logger).Log("msg", "=======> received scrape tasks", "count", len(tasks))

	c.mut.RLock()
	consumers := c.args.ForwardTo
	c.mut.RUnlock()

	// TODO(thampiotr): should handle ctx.Done() here!
	for _, consumer := range consumers {
		consumer.Consume(tasks)
	}
	c.tasksCounter.Add(float64(len(tasks)))

	return nil
}

func (c *Component) serverNeedsUpdate(newArgs Arguments) bool {
	return c.server == nil || c.args.RedisAddress != newArgs.RedisAddress || c.args.Concurrency != newArgs.Concurrency
}
