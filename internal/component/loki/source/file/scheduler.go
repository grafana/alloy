package file

import (
	"context"
	"iter"
	"sync"
	"time"

	"github.com/grafana/dskit/backoff"
)

// Scheduler is used to manage sources.
// So it up to the users to ensure proper synchronization.
type Scheduler[K comparable] struct {
	ctx     context.Context
	cancel  context.CancelFunc
	sources map[K]scheduledSource[K]

	running sync.WaitGroup
}

func NewScheduler[K comparable]() *Scheduler[K] {
	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler[K]{
		ctx:     ctx,
		cancel:  cancel,
		sources: map[K]scheduledSource[K]{},
	}
}

// ApplySource will register and start provided source.
// If a source with the same key already exists it will do nothing.
func (s *Scheduler[K]) ApplySource(source Source[K]) {
	k := source.Key()
	if _, ok := s.sources[k]; ok {
		return
	}

	ctx, cancel := context.WithCancel(s.ctx)
	st := scheduledSource[K]{
		ctx:    ctx,
		cancel: cancel,
		source: source,
	}

	s.sources[k] = st

	s.running.Go(func() {
		// We use a backoff with inifite retries here.
		// This is useful to handle log file rotation when
		// a file might be gone for a very short amount of time.
		// It will only stop retrying when the source is stopped.
		// FIXME: We should be able to configure this per source.
		// e.g. decompressor does not really need it.
		backoff := backoff.New(
			ctx,
			backoff.Config{
				MinBackoff: 1 * time.Second,
				MaxBackoff: 10 * time.Second,
			},
		)

		for backoff.Ongoing() {
			st.source.Run(st.ctx)
			backoff.Wait()
		}
	})
}

// StopSource will unregister provided source and cancel it
// without wating for it to stop.
func (s *Scheduler[K]) StopSource(source Source[K]) {
	k := source.Key()
	scheduledTask, ok := s.sources[k]
	if !ok {
		return
	}
	delete(s.sources, k)
	scheduledTask.cancel()
}

// Sources returns a iterator of all scheduled sources.
func (s *Scheduler[K]) Sources() iter.Seq[Source[K]] {
	return func(yield func(Source[K]) bool) {
		for _, scheduledSource := range s.sources {
			if !yield(scheduledSource.source) {
				return
			}
		}
	}
}

// Contains returns true if a source with provided k already exists.
func (s *Scheduler[K]) Contains(k K) bool {
	_, ok := s.sources[k]
	return ok
}

// Stop will stop all running sources and wait for them to finish.
func (s *Scheduler[K]) Stop() {
	s.cancel()
	s.running.Wait()
}

type Source[K comparable] interface {
	// Run should start the source.
	// It should run until there is no more work or context is canceled.
	Run(ctx context.Context)
	// Key is used to uniquely identity the source.
	Key() K
	IsRunning() bool
}

// scheduledSource is a source that is already scheduled.
// to stop the scheduledSource cancel needs to be called.
type scheduledSource[K comparable] struct {
	ctx    context.Context
	cancel context.CancelFunc
	source Source[K]
}
