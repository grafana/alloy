package scrape

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	PoolSize      int `alloy:"pool_size,attr,optional"`
	PoolQueueSize int `alloy:"pool_queue_size,attr,optional"`

	// TODO(thampiotr): Proper implementation will have all the configuration options required for scraper too.
	// 					for now we're using stubs, so not needed.
}

func (a *Arguments) SetToDefault() {
	a.PoolSize = 10
	a.PoolQueueSize = 20
}

type Exports struct {
	Receiver scrape_task.ScrapeTaskConsumer `alloy:"receiver,attr"`
}

type Component struct {
	opts component.Options

	scraper         promadapter.Scraper
	scrapesCounter  promclient.Counter
	scrapesDuration promclient.Histogram
	seriesPerTarget promclient.Histogram

	mut  sync.RWMutex
	args Arguments
	pool *pool
}

var (
	_ component.Component = (*Component)(nil)
)

func New(o component.Options, args Arguments) (*Component, error) {
	scrapesCounter := promclient.NewCounter(promclient.CounterOpts{
		Name: "scrape_tasks_scrapes_total",
		Help: "Number of scrapes the prometheus.scrape_task.scrape component has completed"})
	err := o.Registerer.Register(scrapesCounter)
	if err != nil {
		return nil, err
	}

	scrapesDuration := promclient.NewHistogram(promclient.HistogramOpts{
		Name: "scrape_tasks_scrapes_duration_seconds",
		Help: "The time it takes to scrape",
	})
	err = o.Registerer.Register(scrapesDuration)
	if err != nil {
		return nil, err
	}

	seriesPerTarget := promclient.NewHistogram(promclient.HistogramOpts{
		Name:    "scrape_tasks_series_per_target",
		Help:    "Number of series per target observed",
		Buckets: []float64{10, 30, 100, 300, 1_000, 3_000, 10_000, 30_000, 100_000, 300_000},
	})
	err = o.Registerer.Register(seriesPerTarget)
	if err != nil {
		return nil, err
	}

	c := &Component{
		opts: o,
		// TODO(thampiotr): for now using a stub, but the idea is to use a proper implementation that can scrape
		scraper:         promstub.NewScraper(),
		scrapesCounter:  scrapesCounter,
		scrapesDuration: scrapesDuration,
		seriesPerTarget: seriesPerTarget,
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
	c.pool = newPool(
		c.args.PoolSize,
		c.args.PoolQueueSize,
		log.With(c.opts.Logger, "subsystem", "task_pool"),
	)
	c.mut.Unlock()

	if oldPool != nil {
		oldPool.drainAndShutdown()
	}

	return nil
}

func (c *Component) Consume(tasks []scrape_task.ScrapeTask) {
	level.Debug(c.opts.Logger).Log("msg", "consuming scrape tasks", "count", len(tasks))

	// Create function to collect errors
	resultMut := sync.Mutex{}
	scrapeErrs := make(map[int]error)
	collectResult := func(index int, m promadapter.Metrics, err error) {
		resultMut.Lock()
		defer resultMut.Unlock()
		scrapeErrs[index] = err
	}

	// Grab the current pool and consumers list
	c.mut.RLock()
	currentPool := c.pool
	consumers := c.args.ForwardTo
	c.mut.RUnlock()

	// Submit all scrape tasks to the pool
	for i, task := range tasks {
		ind := i
		err := currentPool.submit(func() {
			m, err := c.scrapeWithTimeoutAndRetries(task)
			// Fan out the metrics to all consumers.
			for _, cons := range consumers {
				// TODO(thampiotr): change the type here as we don't need the slice?
				// TODO(thampiotr): this must never hang forever - needs timeout
				cons.Consume([]promadapter.Metrics{m})
			}
			collectResult(ind, m, err)
		})
		// Pool may reject, in which case the task won't run.
		if err != nil {
			collectResult(ind, promadapter.Metrics{} /* we got nothing to send */, err)
		}
	}

	// TODO(thampiotr): surface scrape errors in the UI
}

// scrapeWithTimeoutAndRetries must always complete and return an error.
func (c *Component) scrapeWithTimeoutAndRetries(task scrape_task.ScrapeTask) (promadapter.Metrics, error) {
	start := time.Now()

	// Do the scrape
	// TODO(thampiotr): need to make sure here that retries and timeouts work - don't want a task that hangs forever
	metrics, err := c.scraper.ScrapeTarget(task.Target)
	level.Debug(c.opts.Logger).Log("msg", "scraped target", "target", task.Target, "duration", time.Since(start))
	c.scrapesCounter.Inc()
	c.scrapesDuration.Observe(time.Since(start).Seconds())
	c.seriesPerTarget.Observe(float64(metrics.SeriesCount()))

	return metrics, err
}

// TODO(thampiotr): Maybe some out-of-the-box pool would work too. For now I want to have maximum customisability.
type pool struct {
	tasks      chan func()
	isDraining atomic.Bool
	allDone    sync.WaitGroup
	shutdown   chan struct{}
	logger     log.Logger
}

func newPool(size int, queueSize int, logger log.Logger) *pool {
	p := &pool{
		tasks:    make(chan func(), queueSize),
		shutdown: make(chan struct{}),
		logger:   logger,
	}
	for i := 0; i < size; i++ {
		p.allDone.Add(1)
		go func() {
			for {
				select {
				case <-p.shutdown:
					// drain the queue first and return
					for {
						select {
						case task := <-p.tasks:
							task()
						default:
							p.allDone.Done()
							return
						}
					}
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
