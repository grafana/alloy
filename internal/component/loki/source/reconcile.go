package source

import (
	"errors"
	"iter"

	"github.com/go-kit/log"

	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// ErrSkip is used to indicate that a particular source should not be scheduled.
var ErrSkip = errors.New("skip source")

// SourceKeyFn extracts a comparable key of type T from an input value of type I.
// The key is used to uniquely identify sources in the scheduler.
type SourceKeyFn[T comparable, I any] func(I) T

// SourceFactoryFn creates a Source[T] from a key and input value.
// It returns the created source (or nil if creation failed or should be skipped)
// and an error. Return ErrSkip to indicate that the source should not be scheduled
// without logging an error.
type SourceFactoryFn[T comparable, I any] func(T, I) (Source[T], error)

// Reconcile synchronizes the scheduler's set of running sources with a desired state.
// It iterates over inputs, creates sources for new items, and stops sources that are
// no longer needed.
func Reconcile[T comparable, I any](
	logger log.Logger,
	s *Scheduler[T],
	it iter.Seq[I],
	keyFn SourceKeyFn[T, I],
	sourceFactoryFn SourceFactoryFn[T, I],
) {
	// shouldRun tracks the set of keys that should be active after reconciliation.
	shouldRun := make(map[T]struct{})

	// Process all inputs and create sources for new items.
	for i := range it {
		key := keyFn(i)

		// Skip if we've already processed this key in this iteration.
		if _, ok := shouldRun[key]; ok {
			continue
		}

		shouldRun[key] = struct{}{}

		// Skip if a source with this key is already running.
		if s.Contains(key) {
			continue
		}

		source, err := sourceFactoryFn(key, i)
		if err != nil {
			if !errors.Is(err, ErrSkip) {
				level.Error(logger).Log("msg", "failed to create source", "error", err)
			}
			delete(shouldRun, key)
			continue
		}

		s.ScheduleSource(source)
	}

	// We avoid mutating the scheduler state during iteration by collecting
	// sources to remove and stopping them in a separate loop.
	var toDelete []Source[T]
	for source := range s.Sources() {
		if _, ok := shouldRun[source.Key()]; ok {
			continue
		}
		toDelete = append(toDelete, source)
	}

	// Stop all sources that are no longer needed.
	for _, d := range toDelete {
		s.StopSource(d) // stops without blocking
	}
}
