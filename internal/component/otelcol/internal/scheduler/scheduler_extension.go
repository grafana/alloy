package scheduler

import (
	"context"
	"log/slog"
	"sync"

	otelcomponent "go.opentelemetry.io/collector/component"

	"github.com/grafana/alloy/internal/component"
)

// ExtensionScheduler is the scheduler for all extensions: otelcol.auth and
// otelcol.extension components use it.
//
// Extensions export handlers in Update, and other components can use them as
// soon as Update returns. So unlike Scheduler, ExtensionScheduler starts the
// components in Schedule, before it returns. Callers must call Schedule before
// they export the handler, so that exported handlers always point to started
// extensions.
//
// This is safe because extensions are not on the data path: Shutdown of an
// extension does not wait for data to flow downstream. See
// https://github.com/grafana/alloy/issues/7762 for why Scheduler cannot do this.
type ExtensionScheduler struct {
	log *slog.Logger

	healthMut sync.RWMutex
	health    component.Health

	schedMut        sync.Mutex
	schedComponents []otelcomponent.Component
}

// NewExtensionScheduler creates a scheduler for extensions.
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
