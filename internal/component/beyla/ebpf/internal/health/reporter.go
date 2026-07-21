package health

import (
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component"
)

type Reporter struct {
	mu     sync.RWMutex
	health component.Health
}

func New() *Reporter {
	return &Reporter{}
}

func (r *Reporter) Current() component.Health {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.health
}

func (r *Reporter) SetHealthy() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.health = component.Health{
		Health:     component.HealthTypeHealthy,
		UpdateTime: time.Now(),
	}
}

func (r *Reporter) SetUnhealthy(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.health = component.Health{
		Health:     component.HealthTypeUnhealthy,
		Message:    err.Error(),
		UpdateTime: time.Now(),
	}
}
