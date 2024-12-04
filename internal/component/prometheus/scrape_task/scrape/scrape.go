package scrape

import (
	"context"
	"fmt"
	"sync"

	"github.com/go-kit/log"
	promclient "github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promadapter"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promstub"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.scrape_task.scrape",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},
		Exports:   Exports{},

		Build: func(opts component.Options, args component.Arguments) (component.Component, error) {
			return New(opts, args.(Arguments))
		},
	})
}

type Arguments struct {
	ForwardTo []scrape_task.MetricsConsumer `alloy:"forward_to,attr"`
	// TODO(thampiotr): we can do RW shards-style auto-tuning of this pool.
	PoolSize int `alloy:"pool_size,attr"`

	// TODO(thampiotr): Proper implementation will have all the configuration options required for scraper too.
	// 					for now we're using stubs, so not needed.
}

type Exports struct {
	Receiver scrape_task.ScrapeTaskConsumer `alloy:"receiver,attr"`
}

type Component struct {
	opts component.Options

	scraper        promadapter.Scraper
	scrapesCounter promclient.Counter

	mut  sync.RWMutex
	args Arguments
	pool *pool
}

var (
	_ component.Component = (*Component)(nil)
)

func New(o component.Options, args Arguments) (*Component, error) {
	counter := promclient.NewCounter(promclient.CounterOpts{
		Name: "scrape_tasks_completed",
		Help: "Number of scraped tasks this component has completed"})
	err := o.Registerer.Register(counter)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts: o,
		// TODO(thampiotr): for now using a stub, but the idea is to use a proper implementation that can scrape
		scraper:        promstub.NewScraper(),
		scrapesCounter: counter,
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
	c.pool.drainAndShutdown()
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	oldPool := c.pool
	c.args = args.(Arguments)
	c.pool = newPool(c.args.PoolSize, log.With(c.opts.Logger, "subsystem", "task_pool"))
	c.mut.Unlock()

	if oldPool != nil {
		oldPool.drainAndShutdown()
	}

	return nil
}

func (c *Component) Consume(tasks []scrape_task.ScrapeTask) {
	level.Debug(c.opts.Logger).Log("msg", "consuming scrape tasks", "count", len(tasks))

	// Create function to collect results
	resultMut := sync.Mutex{}
	scrapeErrs := make(map[int]error)
	metrics := make([]promadapter.Metrics, len(tasks))
	collectResult := func(index int, m promadapter.Metrics, err error) {
		resultMut.Lock()
		defer resultMut.Unlock()
		scrapeErrs[index] = err
		metrics[index] = m
	}

	// Grab the current pool
	c.mut.RLock()
	currentPool := c.pool
	c.mut.RUnlock()

	// Submit all scrape tasks to the pool
	allDone := sync.WaitGroup{}
	for i, task := range tasks {
		ind := i
		allDone.Add(1)
		err := currentPool.submit(func() {
			m, err := c.scrapeWithTimeoutAndRetries(task)
			collectResult(ind, m, err)
			allDone.Done()
		})
		// Pool may reject, in which case the task won't run.
		if err != nil {
			collectResult(ind, promadapter.Metrics{} /* we got nothing to send */, err)
			allDone.Done()
		}
	}

	// Wait for scraping done.
	allDone.Wait()

	// Grab the consumers list
	c.mut.RLock()
	consumers := c.args.ForwardTo
	c.mut.RUnlock()

	// Fan out the metrics to all consumers.
	for _, cons := range consumers {
		cons.Consume(metrics)
	}

	// TODO(thampiotr): surface scrape errors in the UI
}

// scrapeWithTimeoutAndRetries must always complete and return an error.
func (c *Component) scrapeWithTimeoutAndRetries(task scrape_task.ScrapeTask) (promadapter.Metrics, error) {
	level.Debug(c.opts.Logger).Log("msg", "scraping target", "target", task.Target)

	// TODO(thampiotr): better metrics, do a histogram of scraping time etc
	c.scrapesCounter.Inc()

	// Do the scrape
	// TODO(thampiotr): need to make sure here that retries and timeouts work - don't want a task that hangs forever
	return c.scraper.ScrapeTarget(task.Target)
}

// TODO(thampiotr): Maybe some out-of-the-box pool would work too. For now I want to have maximum customisability.
type pool struct {
	tasks      chan func()
	isDraining atomic.Bool
	allDone    sync.WaitGroup
	shutdown   chan struct{}
	logger     log.Logger
}

func newPool(size int, logger log.Logger) *pool {
	p := &pool{
		// TODO(thampiotr): pool queue maybe should be configurable
		tasks:    make(chan func(), size),
		shutdown: make(chan struct{}),
		logger:   logger,
	}
	for i := 0; i < size; i++ {
		p.allDone.Add(1)
		go func() {
			for {
				select {
				case <-p.shutdown:
					p.allDone.Done()
					return
				case task := <-p.tasks:
					task()
				}
			}
		}()
	}
	return p
}

func (p *pool) submit(fn func()) error {
	if p.isDraining.Load() {
		return fmt.Errorf("pool is draining")
	}
	p.tasks <- fn
	return nil
}

func (p *pool) drainAndShutdown() {
	level.Debug(p.logger).Log("msg", "draining tasks pool")
	p.isDraining.Store(true)
	close(p.shutdown)
	p.allDone.Wait()
	level.Debug(p.logger).Log("msg", "pool drained")
}
