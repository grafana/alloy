package remotewrite

import (
	"fmt"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component"
)

type statusWatcher struct {
	logger log.Logger

	lastChange    time.Time
	lastMessage   string
	currentHealth component.HealthType

	mut sync.Mutex
}

// Log intercepts log messages to the remote write to check component status
func (s *statusWatcher) Log(keyvals ...interface{}) error {

	msg, err := "", ""
	// look specifically for msg = "non-recoverable error"
	for i := 0; i < len(keyvals)-1; i += 2 {
		k := fmt.Sprint(keyvals[i])
		v := fmt.Sprint(keyvals[i+1])
		switch k {
		case "msg":
			msg = v
		case "err":
			err = v
		}
	}
	if msg == "non-recoverable error" {
		s.mut.Lock()
		s.lastChange = time.Now()
		s.lastMessage = err
		s.currentHealth = component.HealthTypeUnhealthy
		s.mut.Unlock()
	}
	// pass through to real logger no matter what
	return s.logger.Log(keyvals...)
}

func (s *statusWatcher) CurrentHealth() component.Health {
	// time after wich we assume we are healthy if no more errors have happened
	// its hard to get a clear signal things are fully working, so this is an ok substitute.
	// TODO: perhaps this could be heuristically inferred from the frequency of failures, or by observing various metrics
	// as they pass through the remote write component's append hook
	const resetTime = 2 * time.Minute
	// time on startup we take to go from unknown to healthy
	const initTime = 10 * time.Second
	s.mut.Lock()
	defer s.mut.Unlock()
	if (s.currentHealth == component.HealthTypeUnhealthy && time.Since(s.lastChange) > resetTime) ||
		(s.currentHealth == component.HealthTypeUnknown && time.Since(s.lastChange) > initTime) {
		s.currentHealth = component.HealthTypeHealthy
		s.lastChange = time.Now()
		s.lastMessage = "Healthy"
	}
	fmt.Println(s.currentHealth, s.lastChange)
	return component.Health{
		Health:     s.currentHealth,
		Message:    s.lastMessage,
		UpdateTime: s.lastChange,
	}
}

func (s *statusWatcher) reset() {
	s.mut.Lock()
	defer s.mut.Unlock()
	s.currentHealth = component.HealthTypeUnknown
	s.lastChange = time.Now()
	s.lastMessage = "reloaded"
}
