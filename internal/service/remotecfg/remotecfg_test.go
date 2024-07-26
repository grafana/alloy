package remotecfg

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"connectrpc.com/connect"
	collectorv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
	"github.com/grafana/alloy/internal/component"
	_ "github.com/grafana/alloy/internal/component/loki/process"
	"github.com/grafana/alloy/internal/featuregate"
	alloy_runtime "github.com/grafana/alloy/internal/runtime"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/syntax"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOnDiskCache(t *testing.T) {
	ctx := componenttest.TestContext(t)
	url := "https://example.com/"

	// The contents of the on-disk cache.
	cacheContents := `loki.process "default" { forward_to = [] }`
	cacheHash := getHash([]byte(cacheContents))

	// Create a new service.
	env := newTestEnvironment(t)
	require.NoError(t, env.ApplyConfig(fmt.Sprintf(`
		url = "%s"
	`, url)))

	client := &collectorClient{}
	env.svc.asClient = client

	// Mock client to return an unparseable response.
	client.getConfigFunc = buildGetConfigHandler("unparseable config")

	// Write the cache contents, and run the service.
	err := os.WriteFile(env.svc.dataPath, []byte(cacheContents), 0644)
	require.NoError(t, err)

	go func() {
		require.NoError(t, env.Run(ctx))
	}()

	// As the API response was unparseable, verify that the service has loaded
	// the on-disk cache contents.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, cacheHash, env.svc.getCfgHash())
	}, time.Second, 10*time.Millisecond)
}

func TestAPIResponse(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	url := "https://example.com/"
	cfg1 := `loki.process "default" { forward_to = [] }`
	cfg2 := `loki.process "updated" { forward_to = [] }`

	// Create a new service.
	env := newTestEnvironment(t)
	require.NoError(t, env.ApplyConfig(fmt.Sprintf(`
		url            = "%s"
		poll_frequency = "10s"
	`, url)))

	client := &collectorClient{}
	env.svc.asClient = client

	// Mock client to return a valid response.
	var registerCalled, unregisterCalled atomic.Bool
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(cfg1)
	client.registerCollectorFunc = buildRegisterCollectorFunc(&registerCalled)
	client.unregisterCollectorFunc = buildUnregisterCollectorFunc(&unregisterCalled)
	client.mut.Unlock()

	// Run the service.
	go func() {
		require.NoError(t, env.Run(ctx))
	}()

	require.Eventually(t, func() bool { return registerCalled.Load() }, 1*time.Second, 10*time.Millisecond)

	// As the API response was successful, verify that the service has loaded
	// the valid response.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfg1)), env.svc.getCfgHash())
	}, time.Second, 10*time.Millisecond)

	// Update the response returned by the API.
	client.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(cfg2)
	client.mut.Unlock()

	// Verify that the service has loaded the updated response.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfg2)), env.svc.getCfgHash())
	}, 1*time.Second, 10*time.Millisecond)

	cancel()
	require.Eventually(t, func() bool { return unregisterCalled.Load() }, 1*time.Second, 10*time.Millisecond)
}

func buildGetConfigHandler(in string) func(context.Context, *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
	return func(context.Context, *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
		rsp := &connect.Response[collectorv1.GetConfigResponse]{
			Msg: &collectorv1.GetConfigResponse{
				Content: in,
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

func buildUnregisterCollectorFunc(called *atomic.Bool) func(ctx context.Context, req *connect.Request[collectorv1.UnregisterCollectorRequest]) (*connect.Response[collectorv1.UnregisterCollectorResponse], error) {
	return func(ctx context.Context, req *connect.Request[collectorv1.UnregisterCollectorRequest]) (*connect.Response[collectorv1.UnregisterCollectorResponse], error) {
		called.Store(true)
		return &connect.Response[collectorv1.UnregisterCollectorResponse]{
			Msg: &collectorv1.UnregisterCollectorResponse{},
		}, nil
	}
}

type testEnvironment struct {
	t   *testing.T
	svc *Service
}

func newTestEnvironment(t *testing.T) *testEnvironment {
	svc, err := New(Options{
		Logger:      util.TestLogger(t),
		StoragePath: t.TempDir(),
	})
	svc.asClient = nil
	require.NoError(t, err)

	return &testEnvironment{
		t:   t,
		svc: svc,
	}
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

type collectorClient struct {
	mut                     sync.RWMutex
	getConfigFunc           func(context.Context, *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error)
	registerCollectorFunc   func(ctx context.Context, req *connect.Request[collectorv1.RegisterCollectorRequest]) (*connect.Response[collectorv1.RegisterCollectorResponse], error)
	unregisterCollectorFunc func(ctx context.Context, req *connect.Request[collectorv1.UnregisterCollectorRequest]) (*connect.Response[collectorv1.UnregisterCollectorResponse], error)
}

func (ag *collectorClient) GetConfig(ctx context.Context, req *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
	ag.mut.RLock()
	defer ag.mut.RUnlock()

	if ag.getConfigFunc != nil {
		return ag.getConfigFunc(ctx, req)
	}

	panic("getConfigFunc not set")
}

func (ag *collectorClient) RegisterCollector(ctx context.Context, req *connect.Request[collectorv1.RegisterCollectorRequest]) (*connect.Response[collectorv1.RegisterCollectorResponse], error) {
	ag.mut.RLock()
	defer ag.mut.RUnlock()

	if ag.registerCollectorFunc != nil {
		return ag.registerCollectorFunc(ctx, req)
	}

	panic("registerCollectorFunc not set")
}

func (ag *collectorClient) UnregisterCollector(ctx context.Context, req *connect.Request[collectorv1.UnregisterCollectorRequest]) (*connect.Response[collectorv1.UnregisterCollectorResponse], error) {
	ag.mut.RLock()
	defer ag.mut.RUnlock()

	if ag.unregisterCollectorFunc != nil {
		return ag.unregisterCollectorFunc(ctx, req)
	}

	panic("unregisterCollectorFunc not set")
}

type serviceController struct {
	f *alloy_runtime.Runtime
}

func (sc serviceController) Run(ctx context.Context) { sc.f.Run(ctx) }
func (sc serviceController) LoadSource(b []byte, args map[string]any) error {
	source, err := alloy_runtime.ParseSource("", b)
	if err != nil {
		return err
	}
	return sc.f.LoadSource(source, args)
}
func (sc serviceController) Ready() bool { return sc.f.Ready() }
