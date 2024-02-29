package integrations

import (
	"context"
	"sync"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

type workerPool struct {
	log       log.Logger
	parentCtx context.Context

	mut     sync.Mutex
	workers map[*controlledIntegration]worker

	runningWorkers sync.WaitGroup
}

type worker struct {
	ci     *controlledIntegration
	stop   context.CancelFunc
	exited chan struct{}
}

func newWorkerPool(ctx context.Context, l log.Logger) *workerPool {
	return &workerPool{
		log:       l,
		parentCtx: ctx,

		workers: make(map[*controlledIntegration]worker),
	}
}

func (p *workerPool) Reload(newIntegrations []*controlledIntegration) {
	p.mut.Lock()
	defer p.mut.Unlock()

	level.Debug(p.log).Log("msg", "updating running integrations", "prev_count", len(p.workers), "new_count", len(newIntegrations))

	// Shut down workers whose integrations have gone away.
	var stopped []worker
	for ci, w := range p.workers {
		var found bool
		for _, current := range newIntegrations {
			if ci == current {
				found = true
				break
			}
		}
		if !found {
			w.stop()
			stopped = append(stopped, w)
		}
	}
	for _, w := range stopped {
		// Wait for stopped integrations to fully exit. We do this in a separate
		// loop so context cancellations can be handled simultaneously, allowing
		// the wait to complete faster.
		<-w.exited
	}

	// Spawn new workers for integrations that don't have them.
	for _, current := range newIntegrations {
		if _, workerExists := p.workers[current]; workerExists {
			continue
		}
		// This integration doesn't have an existing worker; schedule a new one.
		p.scheduleWorker(current)
	}
}

func (p *workerPool) Close() {
	p.mut.Lock()
	defer p.mut.Unlock()

	level.Debug(p.log).Log("msg", "stopping all integrations")

	defer p.runningWorkers.Wait()
	for _, w := range p.workers {
		w.stop()
	}
}

func (p *workerPool) scheduleWorker(ci *controlledIntegration) {
	p.runningWorkers.Add(1)

	ctx, cancel := context.WithCancel(p.parentCtx)

	w := worker{
		ci:     ci,
		stop:   cancel,
		exited: make(chan struct{}),
	}
	p.workers[ci] = w

	go func() {
		ci.running.Store(true)

		// When the integration stops running, we want to free any of our
		// resources that will notify watchers waiting for the worker to stop.
		//
		// Afterwards, we'll block until we remove ourselves from the map; having
		// a worker remove itself on shutdown allows exited integrations to
		// re-start when the config is reloaded.
		defer func() {
			ci.running.Store(false)
			close(w.exited)
			p.runningWorkers.Done()

			p.mut.Lock()
			defer p.mut.Unlock()
			delete(p.workers, ci)
		}()

		err := ci.i.RunIntegration(ctx)
		if err != nil {
			level.Error(p.log).Log("msg", "integration exited with error", "id", ci.id, "err", err)
		}
	}()
}
