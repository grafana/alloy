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
	// DEFAULT_CLEANUP_INTERVAL is the interval at which the cleanup routine runs to remove inactive rate limiters.
	DEFAULT_CLEANUP_INTERVAL = 10 * time.Minute
	// DEFAULT_LIMITER_EXPIRY is the duration after which an inactive rate limiter is considered expired and removed.
	DEFAULT_LIMITER_EXPIRY = 10 * time.Minute
)

// AppRateLimitingConfigKey represents a unique key for an app/environment combination.
// Used for rate limiting purposes.
// Example: "myApp:production"
// Segregates rate limiting configurations per application and environment.
type AppRateLimitingConfigKey string

// AppRateLimiter wraps a rate limiter with metadata for cleanup purposes.
// The lastUsed field tracks when the limiter was last accessed, allowing
// the cleanup routine to remove limiters that haven't been used within
// DEFAULT_LIMITER_EXPIRY duration (10 minutes by default).
// This prevents memory leaks from applications that stop sending requests.
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
// Each unique app/env pair gets its own isolated rate limiter to prevent one application
// from affecting others. Inactive limiters are automatically cleaned up every 10 minutes
// to prevent unbounded memory growth.
type AppRateLimitingConfig struct {
	pool  map[AppRateLimitingConfigKey]*AppRateLimiter
	rate  rate.Limit
	burst int
	mu    sync.RWMutex

	// Metrics
	activeApp          prometheus.Gauge
	rateLimitDecisions *prometheus.CounterVec
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
	}
}

// GetPoolLimiter returns the rate limiter for the given key.
// Returns the limiter and a boolean indicating if it exists.
func (r *AppRateLimitingConfig) GetPoolLimiter(key AppRateLimitingConfigKey) (*rate.Limiter, bool) {
	appLimiter, exists := r.pool[key]
	if !exists {
		return nil, false
	}
	return appLimiter.limiter, true
}

// SetPoolLimiter sets a rate limiter for the given key and returns it.
func (r *AppRateLimitingConfig) SetPoolLimiter(key AppRateLimitingConfigKey, limiter *rate.Limiter) *rate.Limiter {
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
// Updates the lastUsed timestamp on each call to prevent the cleanup routine
// from removing active limiters.
func (r *AppRateLimitingConfig) Allow(app, env string) bool {
	key := ParseAppRateLimitingConfigKey(app, env)

	r.mu.Lock()
	limiter, exists := r.GetPoolLimiter(key)
	if !exists {
		newLimiter := r.getNewLimiter()

		limiter = r.SetPoolLimiter(key, newLimiter)
	} else {
		// Limiter exists, update its last used timestamp to prevent cleanup
		appLimiter := r.pool[key]
		appLimiter.lastUsed = time.Now()
	}

	r.mu.Unlock()

	allowed := limiter.Allow()
	r.rateLimitDecisions.WithLabelValues(app, env, fmt.Sprintf("%t", allowed)).Inc()
	return allowed
}

func (r *AppRateLimitingConfig) getNewLimiter() *rate.Limiter {
	t := time.Now().Add(-time.Duration(float64(time.Second) * float64(r.rate) * float64(r.burst)))
	newLimiter := rate.NewLimiter(r.rate, r.burst)
	newLimiter.SetLimitAt(t, r.rate)
	newLimiter.SetBurstAt(t, r.burst)

	return newLimiter
}
