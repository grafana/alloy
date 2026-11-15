package remotecfg

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	collectorv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
	"github.com/grafana/alloy/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

// The remote configuration's continuity: a collector whose Fleet Management
// registration fails transiently starts from its cached pipeline and registers
// on a later poll, and its shutdown waits for that pipeline's controller.

// recordingHost wraps fakeHost's controller to record when it runs and when its
// Run has returned; stopDelay stands in for components still flushing.
type recordingHost struct {
	fakeHost
	started   chan struct{}
	stopped   *atomic.Bool
	stopDelay time.Duration
}

func newRecordingHost(stopDelay time.Duration) recordingHost {
	return recordingHost{started: make(chan struct{}), stopped: atomic.NewBool(false), stopDelay: stopDelay}
}

func (h recordingHost) NewController(id string) (service.Controller, error) {
	c, err := h.fakeHost.NewController(id)
	if err != nil {
		return nil, err
	}
	return recordingController{Controller: c, host: h}, nil
}

type recordingController struct {
	service.Controller
	host recordingHost
}

func (c recordingController) Run(ctx context.Context) {
	close(c.host.started)
	c.Controller.Run(ctx)
	time.Sleep(c.host.stopDelay)
	c.host.stopped.Store(true)
}

func registerFails(code connect.Code) func(context.Context, *connect.Request[collectorv1.RegisterCollectorRequest]) (*connect.Response[collectorv1.RegisterCollectorResponse], error) {
	return func(context.Context, *connect.Request[collectorv1.RegisterCollectorRequest]) (*connect.Response[collectorv1.RegisterCollectorResponse], error) {
		return nil, connect.NewError(code, errors.New("registration failed"))
	}
}

func getConfigFails(context.Context, *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
	return nil, connect.NewError(connect.CodeUnavailable, errors.New("fleet management unavailable"))
}

// run is one call of the service's Run: done is closed when it returns, and err
// is its result from then on.
type run struct {
	done chan struct{}
	err  error
}

func runService(env *testEnvironment, ctx context.Context, host service.Host) *run {
	r := &run{done: make(chan struct{})}
	go func() {
		defer close(r.done)
		r.err = env.svc.Run(ctx, host)
	}()
	return r
}

func (r *run) returned() bool {
	select {
	case <-r.done:
		return true
	default:
		return false
	}
}

func TestContinuityRegistrationUnavailableStartsFromCache(t *testing.T) {
	cached := `loki.process "cached" { forward_to = [] }`
	remote := `loki.process "remote" { forward_to = [] }`
	client := &mockCollectorClient{}
	var registered atomic.Bool
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registered)
	client.getConfigFunc = getConfigFails

	env := newTestEnvironment(t, client)
	require.NoError(t, env.ApplyConfig(`
		url            = "https://example.com/"
		poll_frequency = "10s"
	`))
	require.NoError(t, os.WriteFile(env.svc.cm.getCachedConfigPath(), []byte(cached), 0644))

	// Fleet Management is down when the collector starts: registration and
	// polls both fail with a transient code.
	client.mut.Lock()
	client.registerCollectorFunc = registerFails(connect.CodeUnavailable)
	client.mut.Unlock()
	registerCalls := client.registerCallCount.Load()

	ctx, cancel := context.WithCancel(t.Context())
	host := newRecordingHost(0)
	r := runService(env, ctx, host)
	t.Cleanup(func() { cancel(); <-r.done })

	// The cached pipeline is loaded and its controller runs; the service keeps running.
	select {
	case <-host.started:
	case <-r.done:
		t.Fatalf("the service returned before running the cached pipeline: %v", r.err)
	case <-time.After(2 * time.Second):
		t.Fatal("the cached pipeline's controller never ran")
	}
	assert.Equal(t, getHash([]byte(cached)), env.svc.cm.getLastLoadedCfgHash())
	require.Never(t, func() bool { return r.returned() }, 300*time.Millisecond, 10*time.Millisecond)

	// Fleet Management recovers: a later poll registers and loads the remote pipeline.
	client.mut.Lock()
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registered)
	client.getConfigFunc = buildGetConfigHandler(remote, "", false)
	client.mut.Unlock()
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Greater(c, client.registerCallCount.Load(), registerCalls+1, "registration is retried on a poll")
		assert.Equal(c, getHash([]byte(remote)), env.svc.cm.getLastLoadedCfgHash())
	}, 2*time.Second, 10*time.Millisecond)
}

func TestContinuityRegistrationUnavailableOnUpdate(t *testing.T) {
	client := &mockCollectorClient{}
	client.registerCollectorFunc = registerFails(connect.CodeUnavailable)
	client.getConfigFunc = getConfigFails

	env := newTestEnvironment(t, client)
	// A transient registration failure applies the arguments; the poll retries it.
	require.NoError(t, env.ApplyConfig(`
		url            = "https://example.com/"
		poll_frequency = "10s"
	`))
	env.svc.mut.RLock()
	defer env.svc.mut.RUnlock()
	assert.Equal(t, "https://example.com/", env.svc.args.URL)
}

// Negative case: a rejected registration (credentials, permissions, arguments)
// still fails startup, as upstream does; only transient codes start from the cache.
func TestContinuityRegistrationRejectedFailsStartup(t *testing.T) {
	for _, code := range []connect.Code{connect.CodeUnauthenticated, connect.CodePermissionDenied, connect.CodeInvalidArgument} {
		t.Run(code.String(), func(t *testing.T) {
			client := &mockCollectorClient{}
			var registered atomic.Bool
			client.registerCollectorFunc = buildRegisterCollectorFunc(&registered)
			client.getConfigFunc = getConfigFails
			env := newTestEnvironment(t, client)
			require.NoError(t, env.ApplyConfig(`url = "https://example.com/"`))

			client.mut.Lock()
			client.registerCollectorFunc = registerFails(code)
			client.mut.Unlock()
			require.Error(t, env.ApplyConfig(`url = "https://example.com/"`))

			r := runService(env, t.Context(), newRecordingHost(0))
			select {
			case <-r.done:
				require.Equal(t, code, connect.CodeOf(r.err))
			case <-time.After(2 * time.Second):
				t.Fatal("a rejected registration did not fail startup")
			}
		})
	}
}

func TestContinuityShutdownJoinsController(t *testing.T) {
	client := &mockCollectorClient{}
	var registered, unregistered atomic.Bool
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registered)
	client.unregisterCollectorFunc = buildUnregisterCollectorFunc(&unregistered)
	client.getConfigFunc = buildGetConfigHandler(`loki.process "default" { forward_to = [] }`, "", false)

	env := newTestEnvironment(t, client)
	require.NoError(t, env.ApplyConfig(`
		url            = "https://example.com/"
		poll_frequency = "10s"
	`))

	ctx, cancel := context.WithCancel(t.Context())
	host := newRecordingHost(300 * time.Millisecond)
	r := runService(env, ctx, host)
	t.Cleanup(func() { cancel(); <-r.done })
	select {
	case <-host.started:
	case <-time.After(2 * time.Second):
		t.Fatal("the controller never ran")
	}

	cancel()
	select {
	case <-r.done:
		require.NoError(t, r.err)
	case <-time.After(5 * time.Second):
		t.Fatal("the service did not stop")
	}
	// Run returned only after the controller's Run did: its components had
	// finished before the process could exit.
	assert.True(t, host.stopped.Load(), "the service returned while its controller was still stopping")
	assert.True(t, unregistered.Load())
}
