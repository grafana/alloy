package remotecfg

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"go.uber.org/atomic"

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
	defer func() { cancel(); wg.Wait() }()

	// As the API response was unparseable, verify that the service has loaded
	// the on-disk cache contents.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		b, err := env.svc.cm.getCachedConfig()
		assert.NoError(c, err)
		assert.Equal(c, cacheContents, string(b))
	}, time.Second, 10*time.Millisecond)
}

func TestGoodBadGood(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	url := "https://example.com/"
	cfgGood := `loki.process "default" { forward_to = [] }`
	cfgBad := `unparseable config`

	client := &mockCollectorClient{}

	// Mock client to return a valid response.
	var registerCalled atomic.Bool
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(cfgGood, "", false)
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
	defer func() { cancel(); wg.Wait() }()

	require.Eventually(t, func() bool { return registerCalled.Load() }, 1*time.Second, 10*time.Millisecond)

	// As the API response was successful, verify that the service has loaded
	// the valid response.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfgGood)), env.svc.cm.getLastLoadedCfgHash())
	}, time.Second, 10*time.Millisecond)

	// Update the response returned by the API to an invalid configuration.
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(cfgBad, "", false)
	client.mut.Unlock()

	// Verify that the service has still the same "good" configuration has
	// loaded and flushed on disk, and that the loaded hash still reflects the good config
	// (since the bad config failed to parse and was never actually loaded).
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		b, err := env.svc.cm.getCachedConfig()
		assert.NoError(c, err)
		assert.Equal(c, cfgGood, string(b))
	}, 1*time.Second, 10*time.Millisecond)

	// The loaded hash should still be the good config since bad config failed to parse
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfgGood)), env.svc.cm.getLastLoadedCfgHash())
	}, 1*time.Second, 10*time.Millisecond)

	// But we should have recorded the bad config as received (for API optimization)
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfgBad)), env.svc.cm.getLastReceivedCfgHash())
	}, 1*time.Second, 10*time.Millisecond)

	// Update the response returned by the API to the previous "good"
	// configuration.
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(cfgGood, "", false)
	client.mut.Unlock()

	// Verify that the service has updated the hash.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfgGood)), env.svc.cm.getLastLoadedCfgHash())
	}, 1*time.Second, 10*time.Millisecond)
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
	defer func() { cancel(); wg.Wait() }()

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
	defer func() { cancel(); wg.Wait() }()

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

// hasStatusInHistory checks if a specific status was captured (simple helper function)
func hasStatusInHistory(history *[]collectorv1.RemoteConfigStatuses, mutex *sync.Mutex, status collectorv1.RemoteConfigStatuses) bool {
	mutex.Lock()
	defer mutex.Unlock()
	for _, s := range *history {
		if s == status {
			return true
		}
	}
	return false
}

// assertRemoteConfigStatus verifies the current remote config status (simple helper function)
func assertRemoteConfigStatus(t *testing.T, env *testEnvironment, expectedStatus collectorv1.RemoteConfigStatuses, shouldHaveError bool) {
	t.Helper()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		status := env.svc.cm.getRemoteConfigStatus()
		assert.Equal(c, expectedStatus, status.Status)
		if shouldHaveError {
			assert.NotEmpty(c, status.ErrorMessage)
		} else {
			assert.Equal(c, "", status.ErrorMessage)
		}
	}, time.Second, 10*time.Millisecond)
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
		OnExportsChange: func(map[string]any) {},
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
	assertRemoteConfigStatus(t, env, collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_UNSET, false)
}

func TestRemoteConfigStatusTransitions(t *testing.T) {
	cfgGood := `loki.process "default" { forward_to = [] }`
	cfgBad := `unparseable config`

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client := &mockCollectorClient{}
	var statusHistory []collectorv1.RemoteConfigStatuses
	var statusMessages []string
	var statusMutex sync.Mutex
	var registerCalled atomic.Bool

	// Start with good config
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandlerWithOptions(GetConfigHandlerOptions{
		Content:        cfgGood,
		StatusHistory:  &statusHistory,
		StatusMessages: &statusMessages,
		StatusMutex:    &statusMutex,
	})
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registerCalled)
	client.mut.Unlock()

	env := newTestEnvironment(t, client)
	require.NoError(t, env.ApplyConfig(`
		url = "https://example.com/"
		poll_frequency = "10s"
	`))

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, env.Run(ctx))
	}()
	defer func() { cancel(); wg.Wait() }()

	require.Eventually(t, func() bool { return registerCalled.Load() }, time.Second, 10*time.Millisecond)

	// Verify initial status: UNSET → APPLIED
	assertRemoteConfigStatus(t, env, collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, false)

	// Switch to bad config → FAILED
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandlerWithOptions(GetConfigHandlerOptions{
		Content:        cfgBad,
		StatusHistory:  &statusHistory,
		StatusMessages: &statusMessages,
		StatusMutex:    &statusMutex,
	})
	client.mut.Unlock()

	assertRemoteConfigStatus(t, env, collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_FAILED, true)

	// Switch back to good config → APPLIED again
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandlerWithOptions(GetConfigHandlerOptions{
		Content:        cfgGood,
		StatusHistory:  &statusHistory,
		StatusMessages: &statusMessages,
		StatusMutex:    &statusMutex,
	})
	client.mut.Unlock()

	assertRemoteConfigStatus(t, env, collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED, false)

	// Verify we captured the complete status transition: APPLIED → FAILED → APPLIED
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, hasStatusInHistory(&statusHistory, &statusMutex, collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED))
		assert.True(c, hasStatusInHistory(&statusHistory, &statusMutex, collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_FAILED))
	}, 2*time.Second, 10*time.Millisecond)
}

func TestRemoteConfigStatusErrorMessages(t *testing.T) {
	cfgBad := `unparseable config`

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client := &mockCollectorClient{}
	var registerCalled atomic.Bool

	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(cfgBad, "", false)
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registerCalled)
	client.mut.Unlock()

	env := newTestEnvironment(t, client)
	require.NoError(t, env.ApplyConfig(`
		url = "https://example.com/"
		poll_frequency = "10s"
	`))

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, env.Run(ctx))
	}()
	defer func() { cancel(); wg.Wait() }()

	require.Eventually(t, func() bool { return registerCalled.Load() }, time.Second, 10*time.Millisecond)

	// Verify FAILED status with descriptive error message
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		status := env.svc.cm.getRemoteConfigStatus()
		assert.Equal(c, collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_FAILED, status.Status)
		assert.NotEmpty(c, status.ErrorMessage, "Should have error message for parse failure")
		assert.Contains(c, status.ErrorMessage, "expected block label", "Error message should contain parse details")
	}, time.Second, 10*time.Millisecond)
}

func TestRemoteConfigStatusNotifications(t *testing.T) {
	cfgGood := `loki.process "default" { forward_to = [] }`
	cfgBad := `unparseable config`

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	client := &mockCollectorClient{}
	var statusHistory []collectorv1.RemoteConfigStatuses
	var statusMessages []string
	var statusMutex sync.Mutex
	var registerCalled atomic.Bool

	// Track multiple status updates
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandlerWithOptions(GetConfigHandlerOptions{
		Content:        cfgGood,
		StatusHistory:  &statusHistory,
		StatusMessages: &statusMessages,
		StatusMutex:    &statusMutex,
	})
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registerCalled)
	client.mut.Unlock()

	env := newTestEnvironment(t, client)
	require.NoError(t, env.ApplyConfig(`
		url = "https://example.com/"
		poll_frequency = "10s"
	`))

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		require.NoError(t, env.Run(ctx))
	}()
	defer func() { cancel(); wg.Wait() }()

	require.Eventually(t, func() bool { return registerCalled.Load() }, time.Second, 10*time.Millisecond)

	// Let initial status be sent
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, hasStatusInHistory(&statusHistory, &statusMutex, collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED))
	}, time.Second, 10*time.Millisecond)

	// Switch to bad config and verify FAILED status is sent
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandlerWithOptions(GetConfigHandlerOptions{
		Content:        cfgBad,
		StatusHistory:  &statusHistory,
		StatusMessages: &statusMessages,
		StatusMutex:    &statusMutex,
	})
	client.mut.Unlock()

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.True(c, hasStatusInHistory(&statusHistory, &statusMutex, collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_FAILED))
	}, time.Second, 10*time.Millisecond)

	// Verify we have both statuses captured in notifications
	statusMutex.Lock()
	appliedCount := 0
	failedCount := 0
	for _, status := range statusHistory {
		if status == collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_APPLIED {
			appliedCount++
		}
		if status == collectorv1.RemoteConfigStatuses_RemoteConfigStatuses_FAILED {
			failedCount++
		}
	}
	statusMutex.Unlock()

	assert.GreaterOrEqual(t, appliedCount, 1, "Should have sent at least one APPLIED status")
	assert.GreaterOrEqual(t, failedCount, 1, "Should have sent at least one FAILED status")
}

// EffectiveConfig integration test

func TestEffectiveConfigInGetConfig(t *testing.T) {
	client := &mockCollectorClient{}

	var capturedEffectiveConfig *collectorv1.EffectiveConfig
	var captureMutex sync.Mutex

	client.getConfigFunc = func(ctx context.Context, req *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
		captureMutex.Lock()
		capturedEffectiveConfig = req.Msg.EffectiveConfig
		captureMutex.Unlock()
		return &connect.Response[collectorv1.GetConfigResponse]{
			Msg: &collectorv1.GetConfigResponse{
				Content: "test config from server",
				Hash:    "test-hash",
			},
		}, nil
	}

	var registerCalled atomic.Bool
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registerCalled)

	env := newTestEnvironment(t, client)
	require.NoError(t, env.ApplyConfig(`url = "https://example.com/"`))

	// First GetConfig call - should not include effective config (not set yet)
	_, err := env.svc.getConfig()
	require.NoError(t, err)
	captureMutex.Lock()
	captured := capturedEffectiveConfig
	captureMutex.Unlock()
	assert.Nil(t, captured, "effective config should be nil on first call before any config is loaded")

	// Manually trigger a config load to set effective config
	env.svc.cm.setEffectiveConfig([]byte("current running config"))

	// Second GetConfig call - should include effective config (first time sending it)
	captureMutex.Lock()
	capturedEffectiveConfig = nil
	captureMutex.Unlock()
	_, err = env.svc.getConfig()
	require.NoError(t, err)
	captureMutex.Lock()
	captured = capturedEffectiveConfig
	captureMutex.Unlock()
	assert.NotNil(t, captured, "effective config should be sent on first call after being set")
	assert.Equal(t, []byte("current running config"), captured.ConfigMap.ConfigMap[""].Body)

	// Third GetConfig call - should not include effective config (no change)
	captureMutex.Lock()
	capturedEffectiveConfig = nil
	captureMutex.Unlock()
	_, err = env.svc.getConfig()
	require.NoError(t, err)
	captureMutex.Lock()
	captured = capturedEffectiveConfig
	captureMutex.Unlock()
	assert.Nil(t, captured, "effective config should not be sent when unchanged")

	// Update the effective config
	env.svc.cm.setEffectiveConfig([]byte("updated running config"))

	// Fourth GetConfig call - should include updated effective config
	captureMutex.Lock()
	capturedEffectiveConfig = nil
	captureMutex.Unlock()
	_, err = env.svc.getConfig()
	require.NoError(t, err)
	captureMutex.Lock()
	captured = capturedEffectiveConfig
	captureMutex.Unlock()
	assert.NotNil(t, captured, "effective config should be sent after change")
	assert.Equal(t, []byte("updated running config"), captured.ConfigMap.ConfigMap[""].Body)
}

func TestUnregisterCollectorOnShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	cfg := `loki.process "default" { forward_to = [] }`
	client := &mockCollectorClient{}

	var registerCalled atomic.Bool
	var unregisterCalled atomic.Bool
	client.getConfigFunc = buildGetConfigHandler(cfg, "", false)
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registerCalled)
	client.unregisterCollectorFunc = buildUnregisterCollectorFunc(&unregisterCalled)

	env := newTestEnvironment(t, client)
	require.NoError(t, env.ApplyConfig(`
		url            = "https://example.com/"
		poll_frequency = "10s"
	`))

	wg := sync.WaitGroup{}
	wg.Go(func() {
		require.NoError(t, env.Run(ctx))
	})
	defer func() { cancel(); wg.Wait() }()

	// Wait for registration to complete.
	require.Eventually(t, func() bool { return registerCalled.Load() }, time.Second, 10*time.Millisecond)

	// Verify unregister hasn't been called yet.
	assert.False(t, unregisterCalled.Load(), "unregister should not be called while service is running")

	// Cancel the context to trigger shutdown.
	cancel()
	wg.Wait()

	// Verify unregister was called during shutdown.
	assert.True(t, unregisterCalled.Load(), "unregister should be called on shutdown")
}

func buildUnregisterCollectorFunc(called *atomic.Bool) func(ctx context.Context, req *connect.Request[collectorv1.UnregisterCollectorRequest]) (*connect.Response[collectorv1.UnregisterCollectorResponse], error) {
	return func(ctx context.Context, req *connect.Request[collectorv1.UnregisterCollectorRequest]) (*connect.Response[collectorv1.UnregisterCollectorResponse], error) {
		called.Store(true)
		return &connect.Response[collectorv1.UnregisterCollectorResponse]{
			Msg: &collectorv1.UnregisterCollectorResponse{},
		}, nil
	}
}
