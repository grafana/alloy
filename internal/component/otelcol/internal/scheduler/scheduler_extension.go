package scheduler

import (
	"context"
	"log/slog"
	"sync"

	otelcomponent "go.opentelemetry.io/collector/component"

	"github.com/grafana/alloy/internal/component"
)

// ExtensionScheduler is a specialized scheduler for extensions.
//
// Extensions export handlers in Update, and other components can use them immediately.
// This scheduler starts components in Schedule so exported handlers always point to started extensions.
type ExtensionScheduler struct {
	log *slog.Logger

	healthMut sync.RWMutex
	health    component.Health

	schedMut        sync.Mutex
	schedComponents []otelcomponent.Component
}

// NewExtensionScheduler creates a scheduler for auth extensions.
func NewExtensionScheduler(l *slog.Logger) *ExtensionScheduler {
	return &ExtensionScheduler{
		log: l,
	}
}

// Schedule stops any running components and starts components provided by cc.
func (s *ExtensionScheduler) Schedule(ctx context.Context, h otelcomponent.Host, cc ...otelcomponent.Component) {
	s.schedMut.Lock()
	defer s.schedMut.Unlock()

	stopComponents(ctx, s.log, s.schedComponents...)

	s.log.Debug("scheduling otelcol components", "count", len(cc))
	var err error
	s.schedComponents, err = startComponents(ctx, s.log, s, h, cc...)
	if err != nil {
		s.log.Error("failed to start some scheduled components", "err", err)
	}
}

// Stop stops all running components.
func (s *ExtensionScheduler) Stop() {
	s.schedMut.Lock()
	defer s.schedMut.Unlock()
	stopComponents(context.Background(), s.log, s.schedComponents...)
}

// CurrentHealth reports the most recent component health status.
func (s *ExtensionScheduler) CurrentHealth() component.Health {
	s.healthMut.RLock()
	defer s.healthMut.RUnlock()
	return s.health
}

func (s *ExtensionScheduler) setHealth(h component.Health) {
	s.healthMut.Lock()
	defer s.healthMut.Unlock()
	s.health = h
}
