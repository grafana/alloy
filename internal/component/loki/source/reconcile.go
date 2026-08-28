package source

import (
	"errors"
	"iter"
	"log/slog"
)

// ErrSkip is used to indicate that a particular source should not be scheduled.
var ErrSkip = errors.New("skip source")

// KeyFn extracts a comparable key of type Key from an input value of type Input.
// The key is used to uniquely identify sources in the scheduler.
type KeyFn[Key comparable, Input any] func(Input) Key

// DedupFn extracts a comparable key of type Dedup from an input value of type Input.
// Inputs that share a key are treated as the same source.
type DedupFn[Dedup comparable, Input any] func(Input) Dedup

// SourceFactoryFn creates a Source[Key] from a key and input value.
// It returns the created source (or nil if creation failed or should be skipped)
// and an error. Return ErrSkip to indicate that the source should not be scheduled
// without logging an error.
type SourceFactoryFn[Key comparable, Input any] func(Key, Input) (Source[Key], error)

// Reconcile synchronizes the scheduler's set of running sources with a desired state.
// It iterates over inputs, creates sources for new items, and stops sources that are
// no longer needed.
func Reconcile[Key comparable, Input any](
	l *slog.Logger,
	s *Scheduler[Key],
	it iter.Seq[Input],
	keyFn KeyFn[Key, Input],
	sourceFactoryFn SourceFactoryFn[Key, Input],
) {

	reconcile(l, s, it, func(i Input) (Key, Key) { key := keyFn(i); return key, key }, sourceFactoryFn)
}

// ReconcileWithDedup behaves like Reconcile, but deduplicates inputs on a key that
// is independent of the one used by the scheduler. When several inputs share a dedup
// key only the first one is used, so callers control which one wins through the order
// of inputs.
func ReconcileWithDedup[Key comparable, Dedup comparable, Input any](
	l *slog.Logger,
	s *Scheduler[Key],
	it iter.Seq[Input],
	keyFn KeyFn[Key, Input],
	dedupFn DedupFn[Dedup, Input],
	sourceFactoryFn SourceFactoryFn[Key, Input],
) {

	reconcile(l, s, it, func(i Input) (Key, Dedup) { return keyFn(i), dedupFn(i) }, sourceFactoryFn)
}

func reconcile[Key, Dedup comparable, Input any](
	l *slog.Logger,
	s *Scheduler[Key],
	it iter.Seq[Input],
	keysFn func(Input) (Key, Dedup),
	sourceFactoryFn SourceFactoryFn[Key, Input],
) {

	var (
		// seen is used to deduplicate targets.
		seen = make(map[Dedup]struct{})
		// shouldRun tracks the set of keys that should be active after reconciliation.
		shouldRun = make(map[Key]struct{})
	)

	// Process all inputs and create sources for new items.
	for i := range it {
		key, dedupKey := keysFn(i)
		// Skip if we've already processed this input in this iteration.
		if _, ok := seen[dedupKey]; ok {
			continue
		}

		seen[dedupKey] = struct{}{}
		shouldRun[key] = struct{}{}

		// Skip if a source with this key is already running.
		if s.Contains(key) {
			continue
		}

		source, err := sourceFactoryFn(key, i)
		if err != nil {
			if !errors.Is(err, ErrSkip) {
				l.Error("failed to create source, skipping", "error", err, "key", key)
			}
			delete(shouldRun, key)
			continue
		}

		s.ScheduleSource(source)
	}

	// We avoid mutating the scheduler state during iteration by collecting
	// sources to remove and stopping them in a separate loop.
	var toDelete []Source[Key]
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
