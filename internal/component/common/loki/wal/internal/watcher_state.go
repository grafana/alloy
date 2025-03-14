package internal

import (
	"sync"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/service/logging/level"
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
	current        int
	mut            sync.RWMutex
	stoppingSignal chan struct{}
	logger         log.Logger
}

func NewWatcherState(l log.Logger) *WatcherState {
	return &WatcherState{
		current:        StateRunning,
		stoppingSignal: make(chan struct{}),
		logger:         l,
	}
}

// Transition changes the state of WatcherState to next, reacting accordingly.
func (s *WatcherState) Transition(next int) {
	s.mut.Lock()
	defer s.mut.Unlock()

	level.Debug(s.logger).Log("msg", "watcher transitioning state", "currentState", printState(s.current), "nextState", printState(next))

	// only perform channel close if the state is not already stopping
	// expect s.s to be either draining ro running to perform a close
	if next == StateStopping && s.current != next {
		close(s.stoppingSignal)
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

// WaitForStopping returns a channel in which the called can read, effectively waiting until the state changes to stopping.
func (s *WatcherState) WaitForStopping() <-chan struct{} {
	return s.stoppingSignal
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
