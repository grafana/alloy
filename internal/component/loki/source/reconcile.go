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

// SourceFactoryFn creates a Source[Key] from a key and input value.
// It returns the created source (or nil if creation failed or should be skipped)
// and an error. Return ErrSkip to indicate that the source should not be scheduled
// without logging an error.
type SourceFactoryFn[Key comparable, Input any] func(Key, Input) (Source[Key], error)

// SourceUpdateFn updates a source which is already scheduled for an input.
//
// If the source can be updated in place, SourceUpdateFn should update it and
// return nil. If the source must be restarted, SourceUpdateFn should return a
// replacement source. Returning ErrSkip stops the existing source without
// scheduling a replacement.
type SourceUpdateFn[Key comparable, Input any] func(Source[Key], Input) (Source[Key], error)

// Reconcile synchronizes the scheduler's set of running sources with a desired state.
// It iterates over inputs, creates sources for new items, updates or replaces existing
// sources, and stops sources that are no longer needed.
func Reconcile[Key comparable, Input any](
	logger *slog.Logger,
	s *Scheduler[Key],
	it iter.Seq[Input],
	keyFn KeyFn[Key, Input],
	sourceFactoryFn SourceFactoryFn[Key, Input],
	sourceUpdateFn SourceUpdateFn[Key, Input],
) {
	// shouldRun tracks the set of keys that should be active after reconciliation.
	shouldRun := make(map[Key]struct{})

	// Process all inputs and create sources for new items.
	for i := range it {
		key := keyFn(i)

		// Skip if we've already processed this key in this iteration.
		if _, ok := shouldRun[key]; ok {
			continue
		}

		shouldRun[key] = struct{}{}

		// Update or replace a source with this key if one is already running.
		if existing, ok := s.GetSource(key); ok {
			if sourceUpdateFn == nil {
				continue
			}

			replacement, err := sourceUpdateFn(existing, i)
			if err != nil {
				if errors.Is(err, ErrSkip) {
					s.StopSource(existing)
					delete(shouldRun, key)
				} else {
					logger.Error("failed to update source, keeping existing source", "error", err, "key", key)
				}
				continue
			}

			if replacement != nil {
				if replacement.Key() != key {
					logger.Error("failed to replace source, replacement has a different key", "key", key, "replacement_key", replacement.Key())
					continue
				}
				s.ReplaceSource(existing, replacement)
			}
			continue
		}

		source, err := sourceFactoryFn(key, i)
		if err != nil {
			if !errors.Is(err, ErrSkip) {
				logger.Error("failed to create source, skipping", "error", err, "key", key)
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
