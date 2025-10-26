package receiver

import (
	"fmt"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// AppRateLimitingConfigKey represents a unique key for an app/environment combination.
// Used for rate limiting purposes.
// Example: "myApp:production"
// Segregates rate limiting configurations per application and environment.
type AppRateLimitingConfigKey string

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
	pool  map[AppRateLimitingConfigKey]*rate.Limiter
	rate  rate.Limit
	burst int
	mu    sync.RWMutex
}

// NewAppRateLimitingConfig creates a new AppRateLimitingConfig with the given rate limit and burst size.
func NewAppRateLimitingConfig(rateLimit float64, burst int) *AppRateLimitingConfig {
	return &AppRateLimitingConfig{
		rate:  rate.Limit(rateLimit),
		burst: burst,
		pool:  make(map[AppRateLimitingConfigKey]*rate.Limiter),
	}
}

// GetPoolLimiter returns the rate limiter for the given key.
// Returns the limiter and a boolean indicating if it exists.
func (r *AppRateLimitingConfig) GetPoolLimiter(key AppRateLimitingConfigKey) (*rate.Limiter, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	limiter, exists := r.pool[key]
	return limiter, exists
}

// SetPoolLimiter sets a rate limiter for the given key and returns it.
func (r *AppRateLimitingConfig) SetPoolLimiter(key AppRateLimitingConfigKey, limiter *rate.Limiter) *rate.Limiter {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.pool[key] = limiter
	return limiter
}

// Allow checks if a request is allowed for the given app/environment combination.
// Creates a new rate limiter if one doesn't exist for the app/env key.
func (r *AppRateLimitingConfig) Allow(app, env string) bool {
	key := ParseAppRateLimitingConfigKey(app, env)

	limiter, exists := r.GetPoolLimiter(key)

	if !exists {
		// Create new rate limiter with pre-filled bucket (similar to handler.Update)
		t := time.Now().Add(-time.Duration(float64(time.Second) * float64(r.rate) * float64(r.burst)))
		newLimiter := rate.NewLimiter(r.rate, r.burst)
		newLimiter.SetLimitAt(t, r.rate)
		newLimiter.SetBurstAt(t, r.burst)

		limiter = r.SetPoolLimiter(key, newLimiter)
	}

	return limiter.Allow()
}
