package file

import (
	"context"
	"iter"
	"sync"

	"github.com/grafana/dskit/backoff"
)

// Scheduler is used to manage sources.
// So it is up to the users to ensure proper synchronization.
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
		sources: make(map[K]scheduledSource[K]),
	}
}

// ApplySource will register and start run provided source.
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
		st.source.Run(st.ctx)
	})
}

// StopSource will unregister provided source and cancel it
// without waiting for it to stop.
func (s *Scheduler[K]) StopSource(source Source[K]) {
	k := source.Key()
	scheduledTask, ok := s.sources[k]
	if !ok {
		return
	}
	delete(s.sources, k)
	scheduledTask.cancel()
}

// Sources returns an iterator of all scheduled sources.
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

// Len returns number of scheduled sources
func (s *Scheduler[K]) Len() int {
	return len(s.sources)
}

// Stop will stop all running sources and wait for them to finish.
func (s *Scheduler[K]) Stop() {
	s.cancel()
	s.running.Wait()
	s.sources = make(map[K]scheduledSource[K])
}

type Source[K comparable] interface {
	// Run should start the source.
	// It should run until there is no more work or context is canceled.
	Run(ctx context.Context)
	// Key is used to uniquely identify the source.
	Key() K
	// IsRunning reports if source is still running.
	IsRunning() bool
}

func NewSourceWithRetry[K comparable](source Source[K], config backoff.Config) *SourceWithRetry[K] {
	return &SourceWithRetry[K]{source, config}
}

// SourceWithRetry is used to wrap another source and apply retries
// when running.
type SourceWithRetry[K comparable] struct {
	source Source[K]
	config backoff.Config
}

func (s *SourceWithRetry[K]) Run(ctx context.Context) {
	backoff := backoff.New(ctx, s.config)

	for backoff.Ongoing() {
		s.source.Run(ctx)
		backoff.Wait()
	}
}

func (s *SourceWithRetry[K]) Key() K {
	return s.source.Key()
}

func (s *SourceWithRetry[K]) IsRunning() bool {
	return s.source.IsRunning()
}

// scheduledSource is a source that is already scheduled.
// to stop the scheduledSource cancel needs to be called.
type scheduledSource[K comparable] struct {
	ctx    context.Context
	cancel context.CancelFunc
	source Source[K]
}
