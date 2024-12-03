package scrape

import (
	"context"
	"errors"
	"fmt"
	"sync"

	promclient "github.com/prometheus/client_golang/prometheus"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promadapter"
	"github.com/grafana/alloy/internal/component/prometheus/scrape_task/internal/promstub"
	"github.com/grafana/alloy/internal/featuregate"
)

func init() {
	component.Register(component.Registration{
		Name:      "prometheus.scrape_task.scrape",
		Stability: featuregate.StabilityExperimental,
		Args:      Arguments{},

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
	return nil
}

// Update implements component.Component.
func (c *Component) Update(args component.Arguments) error {
	c.mut.Lock()
	oldPool := c.pool
	c.args = args.(Arguments)
	c.pool = newPool(c.args.PoolSize)
	c.mut.Unlock()

	if oldPool != nil {
		oldPool.drainAndShutdown()
	}

	return nil
}

func (c *Component) Consume(tasks []scrape_task.ScrapeTask) map[int]error {
	// Create function to collect errors
	resultMut := sync.Mutex{}
	results := make(map[int]error, len(tasks))
	reportResult := func(index int, err error) {
		resultMut.Lock()
		defer resultMut.Unlock()
		results[index] = err
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
			err := c.scrapeWithTimeoutAndRetries(task)
			reportResult(ind, err)
			allDone.Done()
		})
		// Pool may reject, in which case the task won't run.
		if err != nil {
			reportResult(ind, err)
			allDone.Done()
		}
	}

	// Wait for done and return.
	allDone.Wait()
	return results
}

// scrapeWithTimeoutAndRetries must always complete and return an error.
func (c *Component) scrapeWithTimeoutAndRetries(task scrape_task.ScrapeTask) error {
	var err error

	// Do the scrape
	// TODO(thampiotr): need to make sure here that retries and timeouts work - don't want a task that hangs forever
	metrics, err := c.scraper.ScrapeTarget(task.Target)
	if err != nil {
		return err
	}

	// Fan out the same metrics to all consumers.
	for ind, cons := range c.args.ForwardTo {
		consErr := cons.Consume(metrics)
		if consErr != nil {
			err = errors.Join(err, fmt.Errorf("consumer at index %d failed to consume metrics: %w", ind, consErr))
		}
	}
	return err
}

// TODO(thampiotr): Maybe some out-of-the-box pool would work too. For now I want to have maximum customisability.
type pool struct {
	tasks      chan func()
	isDraining atomic.Bool
	allDone    sync.WaitGroup
	shutdown   chan struct{}
}

func newPool(size int) *pool {
	p := &pool{
		tasks: make(chan func(), size),
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
	p.isDraining.Store(true)
	close(p.shutdown)
	p.allDone.Wait()
}
