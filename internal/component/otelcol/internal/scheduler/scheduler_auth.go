package scheduler

import (
	"context"
	"sync"

	"github.com/go-kit/log"
	otelcomponent "go.opentelemetry.io/collector/component"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

// AuthExtentionScheduler is a specialized scheduler for auth extensions.
//
// Auth handlers are exported in Update and can be consumed immediately.
// This scheduler starts components in Schedule so exported handlers always point to started extensions.
type AuthExtentionScheduler struct {
	log log.Logger

	healthMut sync.RWMutex
	health    component.Health

	schedMut        sync.Mutex
	schedComponents []otelcomponent.Component
	host            otelcomponent.Host
}

// NewAuthExtentionScheduler creates a scheduler for auth extensions.
func NewAuthExtentionScheduler(l log.Logger) *AuthExtentionScheduler {
	return &AuthExtentionScheduler{
		log: l,
	}
}

// Schedule stops any running components and starts components provided by cc.
func (s *AuthExtentionScheduler) Schedule(ctx context.Context, h otelcomponent.Host, cc ...otelcomponent.Component) {
	s.schedMut.Lock()
	defer s.schedMut.Unlock()

	stopComponents(ctx, s.log, s.schedComponents...)

	level.Debug(s.log).Log("msg", "scheduling otelcol components", "count", len(cc))
	var err error
	s.schedComponents, err = startComponents(ctx, s.log, s, h, cc...)
	if err != nil {
		level.Error(s.log).Log("msg", "failed to start some scheduled components", "err", err)
	}
	s.host = h
}

// Stop stops all running components.
func (s *AuthExtentionScheduler) Stop() {
	s.schedMut.Lock()
	defer s.schedMut.Unlock()
	stopComponents(context.Background(), s.log, s.schedComponents...)
}

// CurrentHealth reports the most recent component health status.
func (s *AuthExtentionScheduler) CurrentHealth() component.Health {
	s.healthMut.RLock()
	defer s.healthMut.RUnlock()
	return s.health
}

func (s *AuthExtentionScheduler) setHealth(h component.Health) {
	s.healthMut.Lock()
	defer s.healthMut.Unlock()
	s.health = h
}
