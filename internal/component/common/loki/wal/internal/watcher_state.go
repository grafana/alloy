package internal

import (
	"context"
	"log/slog"
	"sync"
)

const (
	// StateRunning is the main functioning state of the watcher. It will keep tailing head segments, consuming closed
	// ones, and checking for new ones.
	StateRunning = iota

	// StateDraining is an intermediary state between running and stopping. The watcher will attempt to consume all the data
	// found in the WAL, omitting errors and assuming all segments found are "closed", that is, no longer being written.
	StateDraining

	// StateStopping means the Watcher is being stopped. It should drop all segment read activity, and exit promptly.
	StateStopping
)

// WatcherState is a holder for the state the Watcher is in. It provides handy methods for checking it it's stopping, getting
// the current state, or blocking until it has stopped.
type WatcherState struct {
	current int
	mut     sync.RWMutex
	logger  *slog.Logger

	// ctx is canceled when the state transitions to StateStopping.
	ctx    context.Context
	cancel context.CancelFunc
}

func NewWatcherState(logger *slog.Logger) *WatcherState {
	ctx, cancel := context.WithCancel(context.Background())

	return &WatcherState{
		current: StateRunning,
		logger:  logger,
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Transition changes the state of WatcherState to next, reacting accordingly.
func (s *WatcherState) Transition(next int) {
	s.mut.Lock()
	defer s.mut.Unlock()

	s.logger.Debug("watcher transitioning state", "currentState", printState(s.current), "nextState", printState(next))

	// only cancel context if the state is not already StateStopping.
	if next == StateStopping && s.current != next {
		s.cancel()
	}

	// update state
	s.current = next
}

// IsDraining evaluates to true if the current state is StateDraining.
func (s *WatcherState) IsDraining() bool {
	s.mut.RLock()
	defer s.mut.RUnlock()
	return s.current == StateDraining
}

// IsStopping evaluates to true if the current state is StateStopping.
func (s *WatcherState) IsStopping() bool {
	s.mut.RLock()
	defer s.mut.RUnlock()
	return s.current == StateStopping
}

// StoppingContext returns a context that will be canceled when state is transitioned to StateStopping.
func (s *WatcherState) StoppingContext() context.Context {
	return s.ctx
}

// printState prints a user-friendly name of the possible Watcher states.
func printState(state int) string {
	switch state {
	case StateRunning:
		return "running"
	case StateDraining:
		return "draining"
	case StateStopping:
		return "stopping"
	default:
		return "unknown"
	}
}
