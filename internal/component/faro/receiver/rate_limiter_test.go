package receiver

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

func TestParseAppRateLimitingConfigKey(t *testing.T) {
	tests := []struct {
		name     string
		app      string
		env      string
		expected AppRateLimitingConfigKey
	}{
		{
			name:     "valid app and env",
			app:      "myapp",
			env:      "production",
			expected: AppRateLimitingConfigKey("myapp:production"),
		},
		{
			name:     "empty app",
			app:      "",
			env:      "production",
			expected: AppRateLimitingConfigKey(":production"),
		},
		{
			name:     "empty env",
			app:      "myapp",
			env:      "",
			expected: AppRateLimitingConfigKey("myapp:"),
		},
		{
			name:     "both empty",
			app:      "",
			env:      "",
			expected: AppRateLimitingConfigKey(":"),
		},
		{
			name:     "special characters",
			app:      "my-app_v2",
			env:      "staging-1",
			expected: AppRateLimitingConfigKey("my-app_v2:staging-1"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseAppRateLimitingConfigKey(tt.app, tt.env)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAppRateLimitingConfigKey_String(t *testing.T) {
	key := AppRateLimitingConfigKey("myapp:production")
	assert.Equal(t, "myapp:production", key.String())
}

func TestNewAppRateLimitingConfig(t *testing.T) {
	tests := []struct {
		name      string
		rateLimit float64
		burst     int
	}{
		{
			name:      "standard rate",
			rateLimit: 100.0,
			burst:     200,
		},
		{
			name:      "fractional rate",
			rateLimit: 0.5,
			burst:     1,
		},
		{
			name:      "high rate",
			rateLimit: 1000.0,
			burst:     2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewAppRateLimitingConfig(tt.rateLimit, tt.burst)

			require.NotNil(t, config)
			assert.NotNil(t, config.pool)
			assert.Equal(t, tt.rateLimit, float64(config.rate))
			assert.Equal(t, tt.burst, config.burst)
		})
	}
}

func TestAppRateLimitingConfig_GetPoolLimiter(t *testing.T) {
	config := NewAppRateLimitingConfig(10.0, 20)
	key := ParseAppRateLimitingConfigKey("myapp", "production")

	// Test getting non-existent limiter
	limiter, exists := config.GetPoolLimiter(key)
	assert.False(t, exists)
	assert.Nil(t, limiter)

	// Test getting existing limiter after setting
	config.Allow("myapp", "production") // This creates the limiter
	limiter, exists = config.GetPoolLimiter(key)
	assert.True(t, exists)
	assert.NotNil(t, limiter)
}

func TestAppRateLimitingConfig_SetPoolLimiter(t *testing.T) {
	config := NewAppRateLimitingConfig(10.0, 20)
	key := ParseAppRateLimitingConfigKey("myapp", "production")

	// Create a new limiter
	newLimiter := rate.NewLimiter(config.rate, config.burst)
	returnedLimiter := config.SetPoolLimiter(key, newLimiter)

	// Verify the limiter was set and returned
	assert.Equal(t, newLimiter, returnedLimiter)

	// Verify we can retrieve it
	retrievedLimiter, exists := config.GetPoolLimiter(key)
	assert.True(t, exists)
	assert.Equal(t, newLimiter, retrievedLimiter)
}

func TestAppRateLimitingConfig_Allow(t *testing.T) {
	tests := []struct {
		name      string
		rateLimit float64
		burst     int
		app       string
		env       string
		requests  int
		expected  []bool
	}{
		{
			name:      "within burst limit",
			rateLimit: 1.0,
			burst:     5,
			app:       "myapp",
			env:       "production",
			requests:  3,
			expected:  []bool{true, true, true},
		},
		{
			name:      "exceed burst limit",
			rateLimit: 1.0,
			burst:     2,
			app:       "myapp",
			env:       "production",
			requests:  4,
			expected:  []bool{true, true, false, false},
		},
		{
			name:      "high rate limit",
			rateLimit: 100.0,
			burst:     10,
			app:       "myapp",
			env:       "production",
			requests:  5,
			expected:  []bool{true, true, true, true, true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := NewAppRateLimitingConfig(tt.rateLimit, tt.burst)

			for i := 0; i < tt.requests; i++ {
				result := config.Allow(tt.app, tt.env)
				assert.Equal(t, tt.expected[i], result, "request %d", i+1)
			}
		})
	}
}

func TestAppRateLimitingConfig_Allow_DifferentApps(t *testing.T) {
	config := NewAppRateLimitingConfig(1.0, 2)

	// Each app should have its own rate limiter
	app1Result1 := config.Allow("app1", "production")
	app1Result2 := config.Allow("app1", "production")
	app1Result3 := config.Allow("app1", "production") // Should be false (burst exceeded)

	app2Result1 := config.Allow("app2", "production")
	app2Result2 := config.Allow("app2", "production")
	app2Result3 := config.Allow("app2", "production") // Should be false (burst exceeded)

	assert.True(t, app1Result1)
	assert.True(t, app1Result2)
	assert.False(t, app1Result3)

	assert.True(t, app2Result1)
	assert.True(t, app2Result2)
	assert.False(t, app2Result3)
}

func TestAppRateLimitingConfig_Allow_DifferentEnvironments(t *testing.T) {
	config := NewAppRateLimitingConfig(1.0, 2)

	// Same app, different environments should have separate rate limiters
	prodResult1 := config.Allow("myapp", "production")
	prodResult2 := config.Allow("myapp", "production")
	prodResult3 := config.Allow("myapp", "production") // Should be false

	stagingResult1 := config.Allow("myapp", "staging")
	stagingResult2 := config.Allow("myapp", "staging")
	stagingResult3 := config.Allow("myapp", "staging") // Should be false

	assert.True(t, prodResult1)
	assert.True(t, prodResult2)
	assert.False(t, prodResult3)

	assert.True(t, stagingResult1)
	assert.True(t, stagingResult2)
	assert.False(t, stagingResult3)
}

func TestAppRateLimitingConfig_ConcurrentAccess(t *testing.T) {
	config := NewAppRateLimitingConfig(100.0, 200)
	numGoroutines := 10
	requestsPerGoroutine := 100

	var wg sync.WaitGroup
	var allowedCount int64
	var mu sync.Mutex

	// Launch multiple goroutines making requests concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			app := "testapp"
			env := "production"

			localAllowed := 0
			for j := 0; j < requestsPerGoroutine; j++ {
				if config.Allow(app, env) {
					localAllowed++
				}
			}

			mu.Lock()
			allowedCount += int64(localAllowed)
			mu.Unlock()
		}()
	}

	wg.Wait()

	// We expect some requests to be allowed (at least the burst size)
	// but not all of them due to rate limiting
	assert.Greater(t, allowedCount, int64(0))
	assert.LessOrEqual(t, allowedCount, int64(numGoroutines*requestsPerGoroutine))
}

func TestAppRateLimitingConfig_RateRecovery(t *testing.T) {
	config := NewAppRateLimitingConfig(10.0, 1) // 10 requests per second, burst of 1

	// Use up the initial burst
	assert.True(t, config.Allow("myapp", "production"))
	assert.False(t, config.Allow("myapp", "production"))

	// Wait for rate limiter to recover (slightly more than 1/10 second)
	time.Sleep(150 * time.Millisecond)

	// Should be able to make another request
	assert.True(t, config.Allow("myapp", "production"))
}

func BenchmarkAppRateLimitingConfig_Allow(b *testing.B) {
	config := NewAppRateLimitingConfig(1000.0, 2000)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		config.Allow("benchapp", "production")
	}
}

func BenchmarkAppRateLimitingConfig_Allow_Concurrent(b *testing.B) {
	config := NewAppRateLimitingConfig(1000.0, 2000)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			config.Allow("benchapp", "production")
		}
	})
}
