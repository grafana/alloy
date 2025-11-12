package receiver

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/alecthomas/units"
	"github.com/grafana/alloy/internal/component/faro/receiver/internal/payload"
	"github.com/grafana/alloy/internal/util"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const emptyPayload = `{
	"traces": {
		"resourceSpans": []
	},
	"logs": [],
	"exceptions": [],
	"measurements": [],
	"meta": {}
}`

func TestMultipleExportersAllSucceed(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 1)
	require.Len(t, exporter2.payloads, 1)
}

func TestMultipleExportersOneFails(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", true, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 0)
	require.Len(t, exporter2.payloads, 1)
}

func TestMultipleExportersAllFail(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", true, nil}
		exporter2 = &testExporter{"exporter2", true, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 0)
	require.Len(t, exporter2.payloads, 0)
}

func TestPayloadWithinLimit(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		MaxAllowedPayloadSize: units.Base2Bytes(len(emptyPayload)),
	})

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 1)
	require.Len(t, exporter2.payloads, 1)
}

func TestPayloadTooLarge(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		MaxAllowedPayloadSize: units.Base2Bytes(len(emptyPayload) - 1),
	})

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusRequestEntityTooLarge, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 0)
	require.Len(t, exporter2.payloads, 0)
}

func TestMissingAPIKey(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		APIKey: "fakekey",
	})

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 0)
	require.Len(t, exporter2.payloads, 0)
}

func TestInvalidAPIKey(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		APIKey: "fakekey",
	})

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)
	req.Header.Set("x-api-key", "badkey")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusUnauthorized, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 0)
	require.Len(t, exporter2.payloads, 0)
}

func TestValidAPIKey(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		APIKey: "fakekey",
	})

	req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
	require.NoError(t, err)
	req.Header.Set("x-api-key", "fakekey")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	require.Equal(t, http.StatusAccepted, rr.Result().StatusCode)
	require.Len(t, exporter1.payloads, 1)
	require.Len(t, exporter2.payloads, 1)
}

func TestCORSPreflightWithDisallowedHeader(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		CORSAllowedOrigins: []string{
			"https://example.com",
		},
	})

	// Test preflight request with disallowed header
	req, err := http.NewRequest(http.MethodOptions, "/collect", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")
	req.Header.Set("Access-Control-Request-Headers", "x-custom-header")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// The preflight should succeed (204), but x-custom-header should NOT be in allowed headers
	require.Equal(t, http.StatusNoContent, rr.Result().StatusCode)
	allowedHeaders := rr.Header().Get("Access-Control-Allow-Headers")

	// When requesting a disallowed header, CORS returns empty Access-Control-Allow-Headers
	// This effectively tells the browser that no custom headers are allowed
	require.Equal(t, "", allowedHeaders, "CORS should return empty allowed headers when disallowed header is requested")
}

func TestCORSPreflightWithAllowedHeader(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		CORSAllowedOrigins: []string{
			"https://example.com",
		},
	})

	// Test preflight request with allowed headers
	req, err := http.NewRequest(http.MethodOptions, "/collect", nil)
	require.NoError(t, err)
	req.Header.Set("Origin", "https://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	sortedHeaders := make([]string, 0, len(defaultAllowedHeaders))
	sortedHeaders = append(sortedHeaders, defaultAllowedHeaders...)
	sort.Strings(sortedHeaders)
	// Library github.com/rs/cors expects values listed in Access-Control-Request-Headers header
	// are unique and sorted;
	// see https://github.com/rs/cors/blob/1084d89a16921942356d1c831fbe523426cf836e/cors.go#L115-L120
	req.Header.Set("Access-Control-Request-Headers", strings.Join(sortedHeaders, ","))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	// The preflight should succeed and include the requested headers
	require.Equal(t, http.StatusNoContent, rr.Result().StatusCode)
	allowedHeaders := rr.Header().Get("Access-Control-Allow-Headers")
	for _, allowed := range defaultAllowedHeaders {
		require.Contains(t, allowedHeaders, allowed)
	}
}

func TestRateLimiter(t *testing.T) {
	var (
		exporter1 = &testExporter{"exporter1", false, nil}
		exporter2 = &testExporter{"exporter2", false, nil}

		h = newHandler(
			util.TestLogger(t),
			prometheus.NewRegistry(),
			[]exporter{exporter1, exporter2},
		)
	)

	h.Update(ServerArguments{
		RateLimiting: RateLimitingArguments{
			Enabled:   true,
			Strategy:  RateLimitingStrategyGlobal, // Default is Global via SetToDefault()
			Rate:      1,
			BurstSize: 2,
		},
	})

	doRequest := func() *httptest.ResponseRecorder {
		req, err := http.NewRequest(http.MethodPost, "/collect", strings.NewReader(emptyPayload))
		require.NoError(t, err)

		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		return rr
	}

	reqs := make([]*httptest.ResponseRecorder, 5)
	for i := range reqs {
		reqs[i] = doRequest()
	}

	// Only 1 request is allowed per second, with a burst of 2; meaning the third
	// request and beyond should be rejected.
	assert.Equal(t, http.StatusAccepted, reqs[0].Result().StatusCode)
	assert.Equal(t, http.StatusAccepted, reqs[1].Result().StatusCode)
	assert.Equal(t, http.StatusTooManyRequests, reqs[2].Result().StatusCode)
	assert.Equal(t, http.StatusTooManyRequests, reqs[3].Result().StatusCode)
	assert.Equal(t, http.StatusTooManyRequests, reqs[4].Result().StatusCode)
}

func TestHandler_RateLimitingPerApp(t *testing.T) {
	tests := []struct {
		name           string
		app            string
		env            string
		requests       int
		expectedStatus []int
	}{
		{
			name:           "within burst limit for app1",
			app:            "app1",
			env:            "production",
			requests:       2,
			expectedStatus: []int{http.StatusAccepted, http.StatusAccepted},
		},
		{
			name:           "exceed burst limit for app1",
			app:            "app1",
			env:            "production",
			requests:       3,
			expectedStatus: []int{http.StatusAccepted, http.StatusAccepted, http.StatusTooManyRequests},
		},
		{
			name:           "app2 should have its own quota",
			app:            "app2",
			env:            "production",
			requests:       2,
			expectedStatus: []int{http.StatusAccepted, http.StatusAccepted},
		},
		{
			name:           "same app different env should have separate quota",
			app:            "app1",
			env:            "staging",
			requests:       2,
			expectedStatus: []int{http.StatusAccepted, http.StatusAccepted},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh handler for each test to avoid interference
			handler := newHandler(util.TestLogger(t), prometheus.NewRegistry(), []exporter{})

			args := ServerArguments{
				RateLimiting: RateLimitingArguments{
					Enabled:   true,
					Strategy:  "per_app", // Enable per-app rate limiting
					Rate:      1.0,       // 1 request per second per app
					BurstSize: 2.0,       // burst of 2 requests per app
				},
			}
			handler.Update(args)

			for i := 0; i < tt.requests; i++ {
				// Create payload with app/env metadata
				payloadStr := `{
					"traces": {"resourceSpans": []},
					"logs": [],
					"exceptions": [],
					"measurements": [],
					"meta": {
						"app": {
							"name": "` + tt.app + `",
							"environment": "` + tt.env + `"
						}
					}
				}`

				req := httptest.NewRequest("POST", "/", strings.NewReader(payloadStr))
				req.Header.Set("Content-Type", "application/json")

				rr := httptest.NewRecorder()
				handler.handleRequest(rr, req)

				assert.Equal(t, tt.expectedStatus[i], rr.Code, "request %d for %s:%s", i+1, tt.app, tt.env)
			}
		})
	}
}

func TestHandler_RateLimitingGlobal(t *testing.T) {
	// Create a test handler with global rate limiting (per-app disabled)
	handler := newHandler(util.TestLogger(t), prometheus.NewRegistry(), []exporter{})

	args := ServerArguments{
		RateLimiting: RateLimitingArguments{
			Enabled:   true,
			Strategy:  "global", // Global rate limiting (default)
			Rate:      1.0,      // 1 request per second
			BurstSize: 2.0,      // burst of 2 requests
		},
	}
	handler.Update(args)

	// Create payloads for different apps
	apps := []struct {
		name string
		env  string
	}{
		{"app1", "production"},
		{"app2", "production"},
	}

	successCount := 0

	for _, app := range apps {
		for i := 0; i < 2; i++ { // 2 requests per app = 4 total
			payloadStr := `{
				"traces": {"resourceSpans": []},
				"logs": [],
				"exceptions": [],
				"measurements": [],
				"meta": {
					"app": {
						"name": "` + app.name + `",
						"environment": "` + app.env + `"
					}
				}
			}`

			req := httptest.NewRequest("POST", "/", strings.NewReader(payloadStr))
			req.Header.Set("Content-Type", "application/json")

			rr := httptest.NewRecorder()
			handler.handleRequest(rr, req)

			if rr.Code == http.StatusAccepted {
				successCount++
			}
		}
	}

	// With global rate limiting (burst=2), only 2 requests should succeed
	// regardless of which app they come from
	assert.Equal(t, 2, successCount, "Expected only 2 requests to succeed with global rate limiting")
}

func TestHandler_RateLimitingPerApp_EmptyMetadata(t *testing.T) {
	tests := []struct {
		name           string
		payloadStr     string
		description    string
		requests       int
		expectedStatus []int
	}{
		{
			name: "entirely missing meta",
			payloadStr: `{
				"traces": {"resourceSpans": []},
				"logs": [],
				"exceptions": [],
				"measurements": []
			}`,
			description:    "Apps without meta field share ':' rate limiter (empty app and env)",
			requests:       3,
			expectedStatus: []int{http.StatusAccepted, http.StatusAccepted, http.StatusTooManyRequests},
		},
		{
			name: "empty app name",
			payloadStr: `{
				"traces": {"resourceSpans": []},
				"logs": [],
				"exceptions": [],
				"measurements": [],
				"meta": {
					"app": {
						"name": "",
						"environment": "production"
					}
				}
			}`,
			description:    "Empty app name uses ':production' key",
			requests:       3,
			expectedStatus: []int{http.StatusAccepted, http.StatusAccepted, http.StatusTooManyRequests},
		},
		{
			name: "empty environment",
			payloadStr: `{
				"traces": {"resourceSpans": []},
				"logs": [],
				"exceptions": [],
				"measurements": [],
				"meta": {
					"app": {
						"name": "myapp",
						"environment": ""
					}
				}
			}`,
			description:    "Empty environment uses 'myapp:' key",
			requests:       3,
			expectedStatus: []int{http.StatusAccepted, http.StatusAccepted, http.StatusTooManyRequests},
		},
		{
			name: "both empty",
			payloadStr: `{
				"traces": {"resourceSpans": []},
				"logs": [],
				"exceptions": [],
				"measurements": [],
				"meta": {
					"app": {
						"name": "",
						"environment": ""
					}
				}
			}`,
			description:    "Both empty uses ':' key (empty app and env)",
			requests:       3,
			expectedStatus: []int{http.StatusAccepted, http.StatusAccepted, http.StatusTooManyRequests},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a fresh handler for each test
			handler := newHandler(util.TestLogger(t), prometheus.NewRegistry(), []exporter{})

			args := ServerArguments{
				RateLimiting: RateLimitingArguments{
					Enabled:   true,
					Strategy:  "per_app",
					Rate:      1.0,
					BurstSize: 2.0, // burst of 2 requests
				},
			}
			handler.Update(args)

			for i := 0; i < tt.requests; i++ {
				req := httptest.NewRequest("POST", "/", strings.NewReader(tt.payloadStr))
				req.Header.Set("Content-Type", "application/json")

				rr := httptest.NewRecorder()
				handler.handleRequest(rr, req)

				assert.Equal(t, tt.expectedStatus[i], rr.Code, "%s: request %d", tt.description, i+1)
			}
		})
	}
}

func TestHandler_RateLimitingPerApp_MultipleAppsWithoutMetadata(t *testing.T) {
	// This test verifies that multiple apps without metadata share the same rate limiter (':' - empty app and env)
	handler := newHandler(util.TestLogger(t), prometheus.NewRegistry(), []exporter{})

	args := ServerArguments{
		RateLimiting: RateLimitingArguments{
			Enabled:   true,
			Strategy:  "per_app",
			Rate:      1.0,
			BurstSize: 2.0, // burst of 2 requests total for all apps without metadata
		},
	}
	handler.Update(args)

	payloads := []string{
		`{"logs": [], "meta": {"app": {"name": "", "environment": ""}}}`,
		`{"logs": [], "meta": {}}`,
		`{"logs": []}`,
	}

	successCount := 0
	for _, payloadStr := range payloads {
		req := httptest.NewRequest("POST", "/", strings.NewReader(payloadStr))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.handleRequest(rr, req)

		if rr.Code == http.StatusAccepted {
			successCount++
		}
	}

	// Only 2 requests should succeed (burst size = 2) since all share ':' rate limiter
	assert.Equal(t, 2, successCount, "All apps without metadata should share the same rate limiter (':' - empty strings)")
}

func TestHandler_RateLimitingPerApp_ProperMetadataNotAffectedByUnknown(t *testing.T) {
	// This test verifies that apps with proper metadata are not affected by apps without metadata
	handler := newHandler(util.TestLogger(t), prometheus.NewRegistry(), []exporter{})

	args := ServerArguments{
		RateLimiting: RateLimitingArguments{
			Enabled:   true,
			Strategy:  "per_app",
			Rate:      1.0,
			BurstSize: 2.0,
		},
	}
	handler.Update(args)

	// First, exhaust the ':' quota (empty app/env) with 2 requests (burst = 2)
	emptyMetadataPayloads := []string{
		`{"logs": [], "meta": {}}`,
		`{"logs": []}`,
	}

	for _, payloadStr := range emptyMetadataPayloads {
		req := httptest.NewRequest("POST", "/", strings.NewReader(payloadStr))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.handleRequest(rr, req)

		assert.Equal(t, http.StatusAccepted, rr.Code, "apps without metadata should succeed within burst limit")
	}

	// Verify that a third request without metadata is rejected
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{"logs": []}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	handler.handleRequest(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "third request without metadata should be rate limited")

	// Now send requests from an app with proper metadata - should have its own quota
	properAppPayload := `{
		"logs": [],
		"meta": {
			"app": {
				"name": "myapp",
				"environment": "production"
			}
		}
	}`

	// Should be able to make 2 requests (burst = 2) even though ':' quota is exhausted
	// Not affected by previous requests without metadata
	for i := range 2 {
		req := httptest.NewRequest("POST", "/", strings.NewReader(properAppPayload))
		req.Header.Set("Content-Type", "application/json")

		rr := httptest.NewRecorder()
		handler.handleRequest(rr, req)

		assert.Equal(t, http.StatusAccepted, rr.Code, "myapp:production should have its own quota, request %d", i+1)
	}

	// Third request for myapp:production should be rate limited (its own limit, not affected by ':' limiter)
	req = httptest.NewRequest("POST", "/", strings.NewReader(properAppPayload))
	req.Header.Set("Content-Type", "application/json")
	rr = httptest.NewRecorder()
	handler.handleRequest(rr, req)
	assert.Equal(t, http.StatusTooManyRequests, rr.Code, "myapp:production should be rate limited after exceeding its own quota")
}

func TestHandler_ExtractAppEnv(t *testing.T) {
	handler := newHandler(util.TestLogger(t), prometheus.NewRegistry(), []exporter{})

	tests := []struct {
		name        string
		payload     payload.Payload
		expectedApp string
		expectedEnv string
	}{
		{
			name: "valid app and env",
			payload: payload.Payload{
				Meta: payload.Meta{
					App: payload.App{
						Name:        "myapp",
						Environment: "production",
					},
				},
			},
			expectedApp: "myapp",
			expectedEnv: "production",
		},
		{
			name: "missing app name",
			payload: payload.Payload{
				Meta: payload.Meta{
					App: payload.App{
						Environment: "production",
					},
				},
			},
			expectedApp: "",
			expectedEnv: "production",
		},
		{
			name: "missing environment",
			payload: payload.Payload{
				Meta: payload.Meta{
					App: payload.App{
						Name: "myapp",
					},
				},
			},
			expectedApp: "myapp",
			expectedEnv: "",
		},
		{
			name:        "empty payload",
			payload:     payload.Payload{},
			expectedApp: "",
			expectedEnv: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, env := handler.extractAppEnv(tt.payload)
			assert.Equal(t, tt.expectedApp, app)
			assert.Equal(t, tt.expectedEnv, env)
		})
	}
}

// TestHandler_Update_PreservesAppRateLimiterState verifies that calling Update multiple times
// with per_app strategy doesn't recreate the AppRateLimitingConfig, preserving existing state.
func TestHandler_Update_PreservesAppRateLimiterState(t *testing.T) {
	h := newHandler(util.TestLogger(t), prometheus.NewRegistry(), []exporter{})

	// First Update with per_app strategy
	h.Update(ServerArguments{
		RateLimiting: RateLimitingArguments{
			Enabled:   true,
			Strategy:  RateLimitingStrategyPerApp,
			Rate:      10,
			BurstSize: 5,
		},
	})

	// Store reference to the created AppRateLimitingConfig
	firstAppRateLimiter := h.appRateLimiter
	require.NotNil(t, firstAppRateLimiter)

	// Second Update with same per_app strategy
	h.Update(ServerArguments{
		RateLimiting: RateLimitingArguments{
			Enabled:   true,
			Strategy:  RateLimitingStrategyPerApp,
			Rate:      10,
			BurstSize: 5,
		},
	})

	// Verify the AppRateLimitingConfig instance wasn't recreated
	require.Same(t, firstAppRateLimiter, h.appRateLimiter, "AppRateLimitingConfig should be preserved across Updates")

	// Update with global strategy should clear it
	h.Update(ServerArguments{
		RateLimiting: RateLimitingArguments{
			Enabled:   true,
			Strategy:  RateLimitingStrategyGlobal,
			Rate:      10,
			BurstSize: 5,
		},
	})

	require.Nil(t, h.appRateLimiter, "AppRateLimitingConfig should be nil when switching to global strategy")

	// Disable rate limiting should also clear it
	h.Update(ServerArguments{
		RateLimiting: RateLimitingArguments{
			Enabled: false,
		},
	})

	require.Nil(t, h.appRateLimiter, "AppRateLimitingConfig should be nil when rate limiting is disabled")
}

type testExporter struct {
	name     string
	broken   bool
	payloads []payload.Payload
}

func (te *testExporter) Name() string {
	return te.name
}

func (te *testExporter) Export(ctx context.Context, payload payload.Payload) error {
	if te.broken {
		return errors.New("this exporter is broken")
	}
	te.payloads = append(te.payloads, payload)
	return nil
}
