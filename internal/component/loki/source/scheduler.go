package source

import (
	"context"
	"iter"
	"sync"

	"github.com/grafana/dskit/backoff"
)

// Scheduler manages the lifecycle of sources.
// It is not safe for concurrent use: callers must ensure proper synchronization
// when accessing or modifying Scheduler and its sources from multiple goroutines.
type Scheduler[Key comparable] struct {
	ctx     context.Context
	cancel  context.CancelFunc
	sources map[Key]scheduledSource[Key]

	running sync.WaitGroup
}

func NewScheduler[Key comparable]() *Scheduler[Key] {
	ctx, cancel := context.WithCancel(context.Background())

	return &Scheduler[Key]{
		ctx:     ctx,
		cancel:  cancel,
		sources: make(map[Key]scheduledSource[Key]),
	}
}

// ScheduleSource will register and run the provided source in a goroutine.
// If a source with the same key already exists it will do nothing.
func (s *Scheduler[Key]) ScheduleSource(source Source[Key]) {
	k := source.Key()
	if _, ok := s.sources[k]; ok {
		return
	}

	ctx, cancel := context.WithCancel(s.ctx)
	st := scheduledSource[Key]{
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
func (s *Scheduler[Key]) StopSource(source Source[Key]) {
	k := source.Key()
	scheduledTask, ok := s.sources[k]
	if !ok {
		return
	}
	delete(s.sources, k)
	scheduledTask.cancel()
}

// Sources returns an iterator of all scheduled sources.
func (s *Scheduler[Key]) Sources() iter.Seq[Source[Key]] {
	return func(yield func(Source[Key]) bool) {
		for _, scheduledSource := range s.sources {
			if !yield(scheduledSource.source) {
				return
			}
		}
	}
}

// Contains returns true if a source with provided k already exists.
func (s *Scheduler[Key]) Contains(k Key) bool {
	_, ok := s.sources[k]
	return ok
}

// Len returns number of scheduled sources
func (s *Scheduler[Key]) Len() int {
	return len(s.sources)
}

// Stop will stop all running sources and wait for them to finish.
// Scheduler should not be reused after Stop is called.
func (s *Scheduler[Key]) Stop() {
	s.cancel()
	s.running.Wait()
	s.sources = make(map[Key]scheduledSource[Key])
}

// Reset will stop all running sources and wait for them to finish and reset
// Scheduler to a usable state.
func (s *Scheduler[Key]) Reset() {
	s.cancel()
	s.running.Wait()
	s.sources = make(map[Key]scheduledSource[Key])
	s.ctx, s.cancel = context.WithCancel(context.Background())
}

type Source[Key comparable] interface {
	// Run should start the source.
	// It should run until there is no more work or context is canceled.
	Run(ctx context.Context)
	// Key is used to uniquely identify the source.
	Key() Key
}

// DebugSource is an optional interface with debug information.
type DebugSource interface {
	DebugInfo() any
}

func NewSourceWithRetry[Key comparable](source Source[Key], config backoff.Config) *SourceWithRetry[Key] {
	return &SourceWithRetry[Key]{source, config}
}

// SourceWithRetry is used to wrap another source and apply retries
// when running.
type SourceWithRetry[Key comparable] struct {
	source Source[Key]
	config backoff.Config
}

func (s *SourceWithRetry[Key]) Run(ctx context.Context) {
	backoff := backoff.New(ctx, s.config)

	for backoff.Ongoing() {
		s.source.Run(ctx)
		backoff.Wait()
	}
}

func (s *SourceWithRetry[Key]) Key() Key {
	return s.source.Key()
}

func (s *SourceWithRetry[Key]) DebugInfo() any {
	ss, ok := s.source.(DebugSource)
	if !ok {
		return nil
	}
	return ss.DebugInfo()
}

// scheduledSource is a source that is already scheduled.
// to stop the scheduledSource cancel needs to be called.
type scheduledSource[Key comparable] struct {
	ctx    context.Context
	cancel context.CancelFunc
	source Source[Key]
}
