package receiver

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"golang.org/x/time/rate"
)

const (
	DEFAULT_CLEANUP_INTERVAL = 10 * time.Minute
	DEFAULT_LIMITER_EXPIRY   = 10 * time.Minute
)

// AppRateLimitingConfigKey represents a unique key for an app/environment combination.
// Used for rate limiting purposes.
// Example: "myApp:production"
// Segregates rate limiting configurations per application and environment.
type AppRateLimitingConfigKey string

type AppRateLimiter struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

// String returns the string representation of the AppRateLimitingConfigKey.
func (k AppRateLimitingConfigKey) String() string {
	return string(k)
}

// ParseAppRateLimitingConfigKey creates a key from app and environment values.
func ParseAppRateLimitingConfigKey(app, env string) AppRateLimitingConfigKey {
	return AppRateLimitingConfigKey(fmt.Sprintf("%s:%s", app, env))
}

// AppRateLimitingConfig manages rate limiters per application/environment combination.
type AppRateLimitingConfig struct {
	pool  map[AppRateLimitingConfigKey]*AppRateLimiter
	rate  rate.Limit
	burst int
	mu    sync.RWMutex

	// Metrics
	activeApp          prometheus.Gauge
	rateLimitDecisions *prometheus.CounterVec

	// Self rate limiter to avoid overload
	selfLimiter *rate.Limiter
}

// NewAppRateLimitingConfig creates a new AppRateLimitingConfig with the given rate limit and burst size.
func NewAppRateLimitingConfig(rateLimit float64, burst int, reg prometheus.Registerer) *AppRateLimitingConfig {
	return &AppRateLimitingConfig{
		rate:  rate.Limit(rateLimit),
		burst: burst,
		pool:  make(map[AppRateLimitingConfigKey]*AppRateLimiter),

		activeApp: promauto.With(reg).NewGauge(prometheus.GaugeOpts{
			Name: "faro_receiver_rate_limiter_active_app",
			Help: "Number of active applications with rate limiters. Inactive limiters are cleaned up every 10 minutes.",
		}),
		rateLimitDecisions: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{
				Name: "faro_receiver_rate_limiter_requests_total",
				Help: "Total number of requests processed by the rate limiter per app/environment.",
			},
			[]string{"app", "env", "allowed"},
		),

		selfLimiter: rate.NewLimiter(rate.Limit(rateLimit), burst),
	}
}

// GetPoolLimiter returns the rate limiter for the given key.
// Returns the limiter and a boolean indicating if it exists.
func (r *AppRateLimitingConfig) GetPoolLimiter(key AppRateLimitingConfigKey) (*rate.Limiter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	appLimiter, exists := r.pool[key]
	if !exists {
		return nil, false
	}
	return appLimiter.limiter, true
}

// SetPoolLimiter sets a rate limiter for the given key and returns it.
func (r *AppRateLimitingConfig) SetPoolLimiter(key AppRateLimitingConfigKey, limiter *rate.Limiter) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pool[key] = &AppRateLimiter{
		limiter:  limiter,
		lastUsed: time.Now(),
	}
	r.activeApp.Set(float64(len(r.pool)))
	return limiter
}

// CleanupRoutine starts a goroutine that periodically removes inactive rate limiters.
// It prevents memory leaks by deleting limiters that haven't been used within the expiry duration.
// The routine runs until the context is cancelled.
func (r *AppRateLimitingConfig) CleanupRoutine(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			r.cleanupExpiredLimiters()
		case <-ctx.Done():
			return
		}
	}
}

// cleanupExpiredLimiters removes rate limiters that haven't been used recently.
func (r *AppRateLimitingConfig) cleanupExpiredLimiters() {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	removed := 0
	for key, appLimiter := range r.pool {
		if now.Sub(appLimiter.lastUsed) > DEFAULT_LIMITER_EXPIRY {
			delete(r.pool, key)
			removed++
		}
	}

	// Update pool size metric if limiters were removed
	if removed > 0 {
		r.activeApp.Set(float64(len(r.pool)))
	}
}

// Allow checks if a request is allowed for the given app/environment combination.
// Creates a new rate limiter if one doesn't exist for the app/env key.
func (r *AppRateLimitingConfig) Allow(app, env string) bool {
	key := ParseAppRateLimitingConfigKey(app, env)

	limiter, exists := r.GetPoolLimiter(key)

	if !exists {
		// Self rate limiting to avoid memory overload on app creation
		if !r.selfLimiter.Allow() {
			return false
		}

		// Create new rate limiter with pre-filled bucket (similar to handler.Update)
		t := time.Now().Add(-time.Duration(float64(time.Second) * float64(r.rate) * float64(r.burst)))
		newLimiter := rate.NewLimiter(r.rate, r.burst)
		newLimiter.SetLimitAt(t, r.rate)
		newLimiter.SetBurstAt(t, r.burst)

		limiter = r.SetPoolLimiter(key, newLimiter)
	} else {
		// Update last used time
		r.mu.Lock()
		r.pool[key].lastUsed = time.Now()
		r.mu.Unlock()
	}

	allowed := limiter.Allow()
	r.rateLimitDecisions.WithLabelValues(app, env, fmt.Sprintf("%t", allowed)).Inc()
	return allowed
}
