package remotecfg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"go.uber.org/atomic"
	"google.golang.org/protobuf/proto"

	"connectrpc.com/connect"
	collectorv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
	"github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1/collectorv1connect"
	"github.com/grafana/alloy/internal/component"
	_ "github.com/grafana/alloy/internal/component/loki/process"
	"github.com/grafana/alloy/internal/featuregate"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/grafana/alloy/syntax/ast"
	"github.com/grafana/alloy/syntax/diag"
	"github.com/grafana/alloy/syntax/token"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnDiskCache(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	client := &mockCollectorClient{}

	var registerCalled atomic.Bool
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registerCalled)
	url := "https://example.com/"

	// The contents of the on-disk cache.
	cacheContents := `loki.process "default" { forward_to = [] }`

	// Create a new service.
	env := newTestEnvironment(t, client)
	require.NoError(t, env.ApplyConfig(fmt.Sprintf(`
		url = "%s"
	`, url)))

	// Mock client to return an unparseable response.
	client.getConfigFunc = buildGetConfigHandler("unparseable config", "", false)

	// Write the cache contents, and run the service.
	err := os.WriteFile(env.svc.cm.getCachedConfigPath(), []byte(cacheContents), 0644)
	require.NoError(t, err)

	// Run the service.
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, env.Run(ctx))
	}()

	// As the API response was unparseable, verify that the service has loaded
	// the on-disk cache contents.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		b, err := env.svc.cm.getCachedConfig()
		assert.NoError(c, err)
		assert.Equal(c, cacheContents, string(b))
	}, time.Second, 10*time.Millisecond)

	cancel()
	wg.Wait()
}

func TestGoodBadGood(t *testing.T) {
	cfgGood := `loki.process "default" { forward_to = [] }`
	cfgBad := `unparseable config`

	// Create managed test service with automatic lifecycle management
	svc := newManagedTestService(t, cfgGood)

	// Verify initial good config is applied
	svc.AssertConfigHash(cfgGood)
	svc.helper.AssertApplied()

	// Switch to bad config
	svc.SetConfig(cfgBad)

	// Verify bad config is received but good config remains loaded (fallback behavior)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfgBad)), svc.env.svc.cm.getLastReceivedCfgHash())
	}, time.Second, 10*time.Millisecond)
	svc.AssertConfigHash(cfgGood) // Still the good config
	svc.helper.AssertFailed()

	// Switch back to good config
	svc.SetConfig(cfgGood)

	// Verify good config is restored
	svc.AssertConfigHash(cfgGood)
	svc.helper.AssertApplied()

	// Verify status transitions were captured
	svc.AssertStatusHistory(
		collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED,
		collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_FAILED,
	)
}

func TestAPIResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	url := "https://example.com/"
	cfg1 := `loki.process "default" { forward_to = [] }`
	cfg2 := `loki.process "updated" { forward_to = [] }`

	client := &mockCollectorClient{}

	// Mock client to return a valid response.
	var registerCalled atomic.Bool
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(cfg1, "", false)
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registerCalled)
	client.mut.Unlock()

	// Create a new service.
	env := newTestEnvironment(t, client)
	require.NoError(t, env.ApplyConfig(fmt.Sprintf(`
		url            = "%s"
		poll_frequency = "10s"
	`, url)))

	// Run the service.
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, env.Run(ctx))
	}()

	require.Eventually(t, func() bool { return registerCalled.Load() }, 1*time.Second, 10*time.Millisecond)

	// As the API response was successful, verify that the service has loaded
	// the valid response.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfg1)), env.svc.cm.getLastLoadedCfgHash())
	}, time.Second, 10*time.Millisecond)

	// Update the response returned by the API.
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(cfg2, "", false)
	client.mut.Unlock()

	// Verify that the service has loaded the updated response.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfg2)), env.svc.cm.getLastLoadedCfgHash())
	}, 1*time.Second, 10*time.Millisecond)

	cancel()
	wg.Wait()
}

func TestAPIResponseNotModified(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	url := "https://example.com/"
	cfg1 := `loki.process "default" { forward_to = [] }`

	client := &mockCollectorClient{}

	// Mock client to return a valid response.
	var registerCalled atomic.Bool
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(cfg1, "12345", false)
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registerCalled)
	client.mut.Unlock()

	// Create a new service.
	env := newTestEnvironment(t, client)
	require.NoError(t, env.ApplyConfig(fmt.Sprintf(`
		url            = "%s"
		poll_frequency = "10s"
	`, url)))

	// Run the service.
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, env.Run(ctx))
	}()

	require.Eventually(t, func() bool { return registerCalled.Load() }, 1*time.Second, 10*time.Millisecond)

	// As the API response was successful, verify that the service has loaded
	// the valid response.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfg1)), env.svc.cm.getLastLoadedCfgHash())
	}, time.Second, 10*time.Millisecond)

	// Update the response returned by the API.
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler("", "12345", true)
	client.mut.Unlock()

	calls := client.getConfigCallCount.Load()

	// Verify that the service has loaded the updated response.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		// Ensure that getConfig has been called again since changing the response.
		assert.Greater(c, client.getConfigCallCount.Load(), calls)
		assert.Equal(c, getHash([]byte(cfg1)), env.svc.cm.getLastLoadedCfgHash())
	}, 1*time.Second, 10*time.Millisecond)

	cancel()
	wg.Wait()
}

func TestUserAgentHeader(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	cfg := `loki.process "default" { forward_to = [] }`

	// Track captured User-Agent headers
	var capturedUserAgent atomic.Value
	var registerCalled atomic.Bool

	// Create a test server that captures the User-Agent header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Capture the User-Agent header
		capturedUserAgent.Store(r.Header.Get("User-Agent"))

		// Mock a successful register collector response
		if r.URL.Path == "/collector.v1.CollectorService/RegisterCollector" {
			registerCalled.Store(true)
			w.Header().Set("Content-Type", "application/proto")
			w.WriteHeader(http.StatusOK)
			// Create empty protobuf response for RegisterCollectorResponse
			response := &collectorv1.RegisterCollectorResponse{}
			data, err := proto.Marshal(response)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Write(data)
			return
		}

		// Mock a successful get config response
		if r.URL.Path == "/collector.v1.CollectorService/GetConfig" {
			w.Header().Set("Content-Type", "application/proto")
			w.WriteHeader(http.StatusOK)
			// Create a minimal protobuf response for GetConfigResponse
			// This is a simple hardcoded protobuf message with the content field
			response := &collectorv1.GetConfigResponse{
				Content: cfg,
			}
			data, err := proto.Marshal(response)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Write(data)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Create a new service with default factory (uses real interceptor)
	svc, err := New(Options{
		Logger:      util.TestLogger(t),
		StoragePath: t.TempDir(),
	})
	require.NoError(t, err)

	env := &testEnvironment{
		t:   t,
		svc: svc,
	}

	// Configure the service to use our test server
	require.NoError(t, env.ApplyConfig(fmt.Sprintf(`
		url            = "%s"
		poll_frequency = "10s"
	`, server.URL)))

	// Run the service
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		err := env.Run(ctx)
		// Accept context cancellation as expected since we're testing User-Agent headers,
		// not service lifecycle. Status notification calls may fail due to context cancellation.
		if err != nil && ctx.Err() == context.Canceled {
			// Context was cancelled, which is expected in this test
			return
		}
		require.NoError(t, err)
	}()

	// Wait for the register call to complete
	require.Eventually(t, func() bool {
		return registerCalled.Load()
	}, 2*time.Second, 10*time.Millisecond)

	// Verify that the User-Agent header was captured and contains "Alloy"
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		userAgent := capturedUserAgent.Load()
		assert.NotNil(c, userAgent, "User-Agent header should be captured")
		if userAgent != nil {
			userAgentStr := userAgent.(string)
			assert.NotEmpty(c, userAgentStr, "User-Agent header should be present")
			assert.Contains(c, userAgentStr, "Alloy", "User-Agent should contain 'Alloy'")
		}
	}, 1*time.Second, 10*time.Millisecond)

	cancel()
	wg.Wait()
}

// setupFallbackTest is a helper to reduce code duplication in fallback tests
func setupFallbackTest(t *testing.T, initialConfig string) (*testEnvironment, *mockCollectorClient) {
	ctx, cancel := context.WithCancel(t.Context())
	url := "https://example.com/"

	client := &mockCollectorClient{}
	var registerCalled atomic.Bool

	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(initialConfig, "", false)
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registerCalled)
	client.mut.Unlock()

	env := newTestEnvironment(t, client)
	require.NoError(t, env.ApplyConfig(fmt.Sprintf(`
		url            = "%s"
		poll_frequency = "10s"
	`, url)))

	// Run the service
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, env.Run(ctx))
	}()

	// Wait for initial registration
	require.Eventually(t, func() bool { return registerCalled.Load() }, 1*time.Second, 10*time.Millisecond)

	// Cleanup function
	t.Cleanup(func() {
		cancel()
		wg.Wait()
	})

	return env, client
}

func TestConfigFallbackToCache(t *testing.T) {
	cfgGood := `loki.process "good" { forward_to = [] }`
	cfgBad := `unparseable bad config`

	env, client := setupFallbackTest(t, cfgGood)

	// Verify initial state: good config loaded and cached
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfgGood)), env.svc.cm.getLastLoadedCfgHash())
		b, err := env.svc.cm.getCachedConfig()
		assert.NoError(c, err)
		assert.Equal(c, cfgGood, string(b))
	}, time.Second, 10*time.Millisecond)

	// Switch API to return bad config that will fail to parse
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(cfgBad, "", false)
	client.mut.Unlock()

	// Verify fallback behavior: bad config received, cache restoration succeeds
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		// Cache should still contain the good config (restoration source)
		cachedContent, err := env.svc.cm.getCachedConfig()
		assert.NoError(c, err)
		assert.Equal(c, cfgGood, string(cachedContent), "cache should contain the good config used for restoration")

		// Received hash = bad config (proves remote processing occurred)
		assert.Equal(c, getHash([]byte(cfgBad)), env.svc.cm.getLastReceivedCfgHash(), "should record bad config as received")

		// Loaded hash = good config (proves cache restoration succeeded)
		assert.Equal(c, getHash([]byte(cfgGood)), env.svc.cm.getLastLoadedCfgHash(), "should maintain good config after cache restoration")

		// Metrics show success (system has working config)
		assert.Equal(c, float64(1), testutil.ToFloat64(env.svc.metrics.lastLoadSuccess), "should indicate success after cache restoration")
	}, 1*time.Second, 10*time.Millisecond)
}

func TestConfigFallbackToCacheFailure(t *testing.T) {
	cfgGood := `loki.process "good" { forward_to = [] }`
	cfgBad := `unparseable bad config`
	corruptedCache := `corrupted cache content`

	env, client := setupFallbackTest(t, cfgGood)

	// Verify initial state: good config loaded
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfgGood)), env.svc.cm.getLastLoadedCfgHash())
	}, time.Second, 10*time.Millisecond)

	// Corrupt the cache to simulate cache failure
	err := os.WriteFile(env.svc.cm.getCachedConfigPath(), []byte(corruptedCache), 0644)
	require.NoError(t, err)

	// Switch API to return bad config that will fail to parse
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(cfgBad, "", false)
	client.mut.Unlock()

	// Verify double failure: both remote config and cache restoration fail
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		// Validate test setup: cache is actually corrupted
		cachedContent, err := env.svc.cm.getCachedConfig()
		assert.NoError(c, err)
		assert.Equal(c, corruptedCache, string(cachedContent), "cache should contain corrupted content")

		// Received hash = bad config (proves remote processing occurred)
		assert.Equal(c, getHash([]byte(cfgBad)), env.svc.cm.getLastReceivedCfgHash(), "should record bad config as received")

		// Loaded hash unchanged = good config (proves both remote and cache failed)
		assert.Equal(c, getHash([]byte(cfgGood)), env.svc.cm.getLastLoadedCfgHash(), "should keep original good config when both remote and cache fail")

		// Metrics show failure (neither remote nor cache succeeded)
		assert.Equal(c, float64(0), testutil.ToFloat64(env.svc.metrics.lastLoadSuccess), "should indicate failure when both remote and cache fail")
	}, 1*time.Second, 10*time.Millisecond)
}

func TestConfigNeverSuccessfullyLoaded(t *testing.T) {
	cfgBad := `unparseable bad config`

	env, _ := setupFallbackTest(t, cfgBad)

	// Write bad config to cache to simulate a scenario where both remote and cache are broken
	err := os.WriteFile(env.svc.cm.getCachedConfigPath(), []byte(cfgBad), 0644)
	require.NoError(t, err)

	// Verify failure scenario: nothing ever successfully loaded
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		// Received hash = bad config (proves remote processing occurred)
		assert.Equal(c, getHash([]byte(cfgBad)), env.svc.cm.getLastReceivedCfgHash(), "should record bad config as received")

		// Loaded hash empty (proves nothing was ever successfully loaded)
		assert.Equal(c, "", env.svc.cm.getLastLoadedCfgHash(), "should have empty loaded hash when nothing ever loaded successfully")

		// Metrics show failure
		assert.Equal(c, float64(0), testutil.ToFloat64(env.svc.metrics.lastLoadSuccess), "should indicate failure when nothing loads successfully")
	}, 1*time.Second, 10*time.Millisecond)
}

func TestConfigSkipCacheRestorationWhenSameHash(t *testing.T) {
	cfgGood := `loki.process "good" { forward_to = [] }`
	cfgBad := `unparseable bad config`

	env, client := setupFallbackTest(t, cfgGood)

	// Verify initial state: good config loaded and cached
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfgGood)), env.svc.cm.getLastLoadedCfgHash())
	}, time.Second, 10*time.Millisecond)

	// Replace cache with the bad config (simulating a scenario where someone manually corrupted the cache with the same content that will be sent remotely)
	err := os.WriteFile(env.svc.cm.getCachedConfigPath(), []byte(cfgBad), 0644)
	require.NoError(t, err)

	// Switch API to return the same bad config
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(cfgBad, "", false)
	client.mut.Unlock()

	// Verify that cache restoration is skipped when cached config has same hash as failed remote config
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		// Received hash = bad config (proves remote processing occurred)
		assert.Equal(c, getHash([]byte(cfgBad)), env.svc.cm.getLastReceivedCfgHash(), "should record bad config as received")

		// Loaded hash should still be the original good config (cache restoration was skipped because cache has same bad content)
		assert.Equal(c, getHash([]byte(cfgGood)), env.svc.cm.getLastLoadedCfgHash(), "should keep original good config when cache contains same bad config")

		// Cache should contain the bad config we wrote
		cachedContent, err := env.svc.cm.getCachedConfig()
		assert.NoError(c, err)
		assert.Equal(c, cfgBad, string(cachedContent), "cache should contain the bad config")

		// Metrics show failure (remote failed and cache restoration was skipped)
		assert.Equal(c, float64(0), testutil.ToFloat64(env.svc.metrics.lastLoadSuccess), "should indicate failure when remote fails and cache restoration is skipped")
	}, 1*time.Second, 10*time.Millisecond)
}

// GetConfigHandlerOptions provides configuration for buildGetConfigHandler
type GetConfigHandlerOptions struct {
	Content     string
	Hash        string
	NotModified bool
	// Optional status capture functionality
	StatusHistory  *[]collectorv1.RemoteConfigStatuses
	StatusMessages *[]string
	StatusMutex    *sync.Mutex
}

// buildGetConfigHandler creates a GetConfig handler function with optional status capturing
func buildGetConfigHandler(content, hash string, notModified bool) func(context.Context, *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
	return buildGetConfigHandlerWithOptions(GetConfigHandlerOptions{
		Content:     content,
		Hash:        hash,
		NotModified: notModified,
	})
}

// buildGetConfigHandlerWithOptions creates a GetConfig handler with full configuration options
func buildGetConfigHandlerWithOptions(opts GetConfigHandlerOptions) func(context.Context, *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
	return func(ctx context.Context, req *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
		// Capture status updates if configured
		if opts.StatusHistory != nil && opts.StatusMessages != nil && opts.StatusMutex != nil && req.Msg.RemoteConfigStatus != nil {
			opts.StatusMutex.Lock()
			*opts.StatusHistory = append(*opts.StatusHistory, req.Msg.RemoteConfigStatus.Status)
			*opts.StatusMessages = append(*opts.StatusMessages, req.Msg.RemoteConfigStatus.ErrorMessage)
			opts.StatusMutex.Unlock()
		}

		rsp := &connect.Response[collectorv1.GetConfigResponse]{
			Msg: &collectorv1.GetConfigResponse{
				Content:     opts.Content,
				NotModified: opts.NotModified,
				Hash:        opts.Hash,
			},
		}
		return rsp, nil
	}
}

func buildRegisterCollectorFunc(called *atomic.Bool) func(ctx context.Context, req *connect.Request[collectorv1.RegisterCollectorRequest]) (*connect.Response[collectorv1.RegisterCollectorResponse], error) {
	return func(ctx context.Context, req *connect.Request[collectorv1.RegisterCollectorRequest]) (*connect.Response[collectorv1.RegisterCollectorResponse], error) {
		called.Store(true)
		return &connect.Response[collectorv1.RegisterCollectorResponse]{
			Msg: &collectorv1.RegisterCollectorResponse{},
		}, nil
	}
}

type testEnvironment struct {
	t   *testing.T
	svc *Service
}

func newTestEnvironment(t *testing.T, client *mockCollectorClient) *testEnvironment {
	svc, err := New(Options{
		Logger:      util.TestLogger(t),
		StoragePath: t.TempDir(),
	})
	require.NoError(t, err)

	// Replace the package-level function with our mock
	originalCreateAPIClient := createAPIClient
	createAPIClient = func(args Arguments, metrics *metrics) (collectorv1connect.CollectorServiceClient, error) {
		// Return the mock wrapped in an apiClient to preserve the NotModified handling
		return newAPIClientWithClient(client, metrics), nil
	}

	// Restore original function when test completes
	t.Cleanup(func() {
		createAPIClient = originalCreateAPIClient
	})

	return &testEnvironment{
		t:   t,
		svc: svc,
	}
}

// StatusTracker helps track and verify remote config status updates
type StatusTracker struct {
	History  []collectorv1.RemoteConfigStatuses
	Messages []string
	mutex    sync.Mutex
}

// NewStatusTracker creates a new status tracker
func NewStatusTracker() *StatusTracker {
	return &StatusTracker{
		History:  make([]collectorv1.RemoteConfigStatuses, 0),
		Messages: make([]string, 0),
	}
}

// GetReferences returns pointers for use with GetConfigHandlerOptions
func (st *StatusTracker) GetReferences() (*[]collectorv1.RemoteConfigStatuses, *[]string, *sync.Mutex) {
	return &st.History, &st.Messages, &st.mutex
}

// HasStatus checks if a specific status was captured
func (st *StatusTracker) HasStatus(status collectorv1.RemoteConfigStatuses) bool {
	st.mutex.Lock()
	defer st.mutex.Unlock()
	for _, s := range st.History {
		if s == status {
			return true
		}
	}
	return false
}

// HasStatuses checks if all specified statuses were captured
func (st *StatusTracker) HasStatuses(statuses ...collectorv1.RemoteConfigStatuses) bool {
	for _, status := range statuses {
		if !st.HasStatus(status) {
			return false
		}
	}
	return true
}

// statusHelper provides concise status assertion methods
type statusHelper struct {
	env *testEnvironment
	t   *testing.T
}

// newStatusHelper creates a status assertion helper
func newStatusHelper(env *testEnvironment) *statusHelper {
	return &statusHelper{env: env, t: env.t}
}

// AssertStatus verifies the current remote config status
func (sh *statusHelper) AssertStatus(expectedStatus collectorv1.RemoteConfigStatuses, expectedMessage string) {
	sh.t.Helper()
	require.EventuallyWithT(sh.t, func(c *assert.CollectT) {
		status := sh.env.svc.cm.getRemoteConfigStatus()
		assert.Equal(c, expectedStatus, status.Status)
		if expectedMessage == "" {
			assert.Equal(c, "", status.ErrorMessage)
		} else {
			assert.Contains(c, status.ErrorMessage, expectedMessage)
		}
	}, time.Second, 10*time.Millisecond)
}

// AssertApplied verifies status is APPLIED with no error
func (sh *statusHelper) AssertApplied() {
	sh.AssertStatus(collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, "")
}

// AssertFailed verifies status is FAILED with an error message
func (sh *statusHelper) AssertFailed() {
	sh.t.Helper()
	require.EventuallyWithT(sh.t, func(c *assert.CollectT) {
		status := sh.env.svc.cm.getRemoteConfigStatus()
		assert.Equal(c, collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_FAILED, status.Status)
		assert.NotEmpty(c, status.ErrorMessage, "Should have error message for failure")
	}, time.Second, 10*time.Millisecond)
}

// AssertUnset verifies status is UNSET
func (sh *statusHelper) AssertUnset() {
	sh.AssertStatus(collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_UNSET, "")
}

// managedTestService handles service lifecycle automatically
type managedTestService struct {
	env     *testEnvironment
	client  *mockCollectorClient
	tracker *StatusTracker
	helper  *statusHelper
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	t       *testing.T
}

// newManagedTestService creates a managed test service with automatic cleanup
func newManagedTestService(t *testing.T, initialConfig string) *managedTestService {
	ctx, cancel := context.WithCancel(t.Context())

	client := &mockCollectorClient{}
	tracker := NewStatusTracker()

	var registerCalled atomic.Bool
	statusHistory, statusMessages, statusMutex := tracker.GetReferences()

	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandlerWithOptions(GetConfigHandlerOptions{
		Content:        initialConfig,
		Hash:           "",
		NotModified:    false,
		StatusHistory:  statusHistory,
		StatusMessages: statusMessages,
		StatusMutex:    statusMutex,
	})
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registerCalled)
	client.mut.Unlock()

	env := newTestEnvironment(t, client)
	require.NoError(t, env.ApplyConfig(`
		url            = "https://example.com/"
		poll_frequency = "10s"
	`))

	mts := &managedTestService{
		env:     env,
		client:  client,
		tracker: tracker,
		helper:  newStatusHelper(env),
		ctx:     ctx,
		cancel:  cancel,
		t:       t,
	}

	// Start service
	mts.wg.Add(1)
	go func() {
		defer mts.wg.Done()
		require.NoError(t, env.Run(ctx))
	}()

	// Wait for registration
	require.Eventually(t, func() bool { return registerCalled.Load() }, time.Second, 10*time.Millisecond)

	// Setup cleanup
	t.Cleanup(func() {
		mts.cancel()
		mts.wg.Wait()
	})

	return mts
}

// SetConfig updates the mock client to return new config
func (mts *managedTestService) SetConfig(config string) {
	statusHistory, statusMessages, statusMutex := mts.tracker.GetReferences()

	mts.client.mut.Lock()
	mts.client.getConfigFunc = buildGetConfigHandlerWithOptions(GetConfigHandlerOptions{
		Content:        config,
		Hash:           "",
		NotModified:    false,
		StatusHistory:  statusHistory,
		StatusMessages: statusMessages,
		StatusMutex:    statusMutex,
	})
	mts.client.mut.Unlock()
}

// AssertConfigHash verifies the loaded config hash
func (mts *managedTestService) AssertConfigHash(expectedConfig string) {
	mts.t.Helper()
	require.EventuallyWithT(mts.t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(expectedConfig)), mts.env.svc.cm.getLastLoadedCfgHash())
	}, time.Second, 10*time.Millisecond)
}

// AssertStatusHistory verifies expected statuses were captured
func (mts *managedTestService) AssertStatusHistory(expectedStatuses ...collectorv1.RemoteConfigStatuses) {
	mts.t.Helper()
	require.EventuallyWithT(mts.t, func(c *assert.CollectT) {
		assert.True(c, mts.tracker.HasStatuses(expectedStatuses...),
			"Should have captured all expected statuses: %v", expectedStatuses)
	}, 2*time.Second, 10*time.Millisecond)
}

func (env *testEnvironment) ApplyConfig(config string) error {
	var args Arguments
	if err := syntax.Unmarshal([]byte(config), &args); err != nil {
		return err
	}
	// The lower limit of the poll_frequency argument would slow our tests
	// considerably; let's artificially lower it after the initial validation
	// has taken place.
	args.PollFrequency /= 100
	return env.svc.Update(args)
}

func (env *testEnvironment) Run(ctx context.Context) error {
	return env.svc.Run(ctx, fakeHost{})
}

type fakeHost struct{}

var _ service.Host = (fakeHost{})

func (fakeHost) GetComponent(id component.ID, opts component.InfoOptions) (*component.Info, error) {
	return nil, fmt.Errorf("no such component %s", id)
}

func (fakeHost) ListComponents(moduleID string, opts component.InfoOptions) ([]*component.Info, error) {
	if moduleID == "" {
		return nil, nil
	}
	return nil, fmt.Errorf("no such module %q", moduleID)
}

func (fakeHost) GetServiceConsumers(_ string) []service.Consumer { return nil }
func (fakeHost) GetService(_ string) (service.Service, bool)     { return nil, false }

func (f fakeHost) NewController(id string) service.Controller {
	logger, _ := logging.New(io.Discard, logging.DefaultOptions)
	ctrl := alloy_runtime.New(alloy_runtime.Options{
		ControllerID:    ServiceName,
		Logger:          logger,
		Tracer:          nil,
		DataPath:        "",
		MinStability:    featuregate.StabilityGenerallyAvailable,
		Reg:             prometheus.NewRegistry(),
		OnExportsChange: func(map[string]interface{}) {},
		Services:        []service.Service{livedebugging.New()},
	})

	return serviceController{ctrl}
}

type serviceController struct {
	f *alloy_runtime.Runtime
}

func (sc serviceController) Run(ctx context.Context) { sc.f.Run(ctx) }
func (sc serviceController) LoadSource(b []byte, args map[string]any, configPath string) (*ast.File, error) {
	source, err := alloy_runtime.ParseSource("", b)
	if err != nil {
		return nil, err
	}
	return source.SourceFiles()[""], sc.f.LoadSource(source, args, configPath)
}
func (sc serviceController) Ready() bool { return sc.f.Ready() }

func TestRemoteConfigStatus_InitialState(t *testing.T) {
	env := newTestEnvironment(t, &mockCollectorClient{})
	newStatusHelper(env).AssertUnset()
}

func TestGetErrorMessage_DiagnosticErrors(t *testing.T) {
	// Test that diagnostic errors use AllMessages() for detailed error information

	// Create a mock diagnostic error
	mockDiags := diag.Diagnostics{
		{
			Severity: diag.SeverityLevelError,
			StartPos: token.Position{Filename: "test.alloy", Line: 1, Column: 1},
			Message:  "first error",
		},
		{
			Severity: diag.SeverityLevelError,
			StartPos: token.Position{Filename: "test.alloy", Line: 2, Column: 1},
			Message:  "second error",
		},
	}

	// Test getErrorMessage function directly
	errorMsg := getErrorMessage(mockDiags)

	// Should contain both error messages, not just the first
	assert.Contains(t, errorMsg, "first error")
	assert.Contains(t, errorMsg, "second error")
	assert.Contains(t, errorMsg, ";") // Should be joined with semicolon

	// Test with regular error
	regularErr := errors.New("simple error")
	regularMsg := getErrorMessage(regularErr)
	assert.Equal(t, "simple error", regularMsg)
}
