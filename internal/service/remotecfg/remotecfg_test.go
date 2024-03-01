package remotecfg

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"connectrpc.com/connect"
	agentv1 "github.com/grafana/agent-remote-config/api/gen/proto/go/agent/v1"
	"github.com/grafana/agent/internal/component"
	_ "github.com/grafana/agent/internal/component/loki/process"
	"github.com/grafana/agent/internal/featuregate"
	"github.com/grafana/agent/internal/flow"
	"github.com/grafana/agent/internal/flow/componenttest"
	"github.com/grafana/agent/internal/flow/logging"
	"github.com/grafana/agent/internal/service"
	"github.com/grafana/agent/internal/util"
	"github.com/grafana/river"
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

	client := &agentClient{}
	env.svc.asClient = client

	// Mock client to return an unparseable response.
	client.getConfigFunc = buildGetConfigHandler("unparseable river config")

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
	ctx := componenttest.TestContext(t)
	url := "https://example.com/"
	cfg1 := `loki.process "default" { forward_to = [] }`
	cfg2 := `loki.process "updated" { forward_to = [] }`

	// Create a new service.
	env := newTestEnvironment(t)
	require.NoError(t, env.ApplyConfig(fmt.Sprintf(`
		url            = "%s"
		poll_frequency = "10ms"
	`, url)))

	client := &agentClient{}
	env.svc.asClient = client

	// Mock client to return a valid response.
	client.getConfigFunc = buildGetConfigHandler(cfg1)

	// Run the service.
	go func() {
		require.NoError(t, env.Run(ctx))
	}()

	// As the API response was successful, verify that the service has loaded
	// the valid response.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfg1)), env.svc.getCfgHash())
	}, time.Second, 10*time.Millisecond)

	// Update the response returned by the API.
	env.svc.mut.Lock()
	client.getConfigFunc = buildGetConfigHandler(cfg2)
	env.svc.mut.Unlock()

	// Verify that the service has loaded the updated response.
	require.EventuallyWithT(t, func(c *assert.CollectT) {
		assert.Equal(c, getHash([]byte(cfg2)), env.svc.getCfgHash())
	}, time.Second, 10*time.Millisecond)
}

func buildGetConfigHandler(in string) func(context.Context, *connect.Request[agentv1.GetConfigRequest]) (*connect.Response[agentv1.GetConfigResponse], error) {
	return func(context.Context, *connect.Request[agentv1.GetConfigRequest]) (*connect.Response[agentv1.GetConfigResponse], error) {
		rsp := &connect.Response[agentv1.GetConfigResponse]{
			Msg: &agentv1.GetConfigResponse{
				Content: in,
			},
		}
		return rsp, nil
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
	if err := river.Unmarshal([]byte(config), &args); err != nil {
		return err
	}
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
	ctrl := flow.New(flow.Options{
		ControllerID:    ServiceName,
		Logger:          logger,
		Tracer:          nil,
		DataPath:        "",
		MinStability:    featuregate.StabilityStable,
		Reg:             prometheus.NewRegistry(),
		OnExportsChange: func(map[string]interface{}) {},
		Services:        []service.Service{},
	})

	return serviceController{ctrl}
}

type agentClient struct {
	getConfigFunc func(context.Context, *connect.Request[agentv1.GetConfigRequest]) (*connect.Response[agentv1.GetConfigResponse], error)
}

func (ag agentClient) GetConfig(ctx context.Context, req *connect.Request[agentv1.GetConfigRequest]) (*connect.Response[agentv1.GetConfigResponse], error) {
	if ag.getConfigFunc != nil {
		return ag.getConfigFunc(ctx, req)
	}

	panic("getConfigFunc not set")
}
func (ag agentClient) GetAgent(context.Context, *connect.Request[agentv1.GetAgentRequest]) (*connect.Response[agentv1.Agent], error) {
	return nil, nil
}
func (ag agentClient) CreateAgent(context.Context, *connect.Request[agentv1.CreateAgentRequest]) (*connect.Response[agentv1.Agent], error) {
	return nil, nil
}
func (ag agentClient) UpdateAgent(context.Context, *connect.Request[agentv1.UpdateAgentRequest]) (*connect.Response[agentv1.Agent], error) {
	return nil, nil
}
func (ag agentClient) DeleteAgent(context.Context, *connect.Request[agentv1.DeleteAgentRequest]) (*connect.Response[agentv1.DeleteAgentResponse], error) {
	return nil, nil
}
func (ag agentClient) ListAgents(context.Context, *connect.Request[agentv1.ListAgentsRequest]) (*connect.Response[agentv1.Agents], error) {
	return nil, nil
}

type serviceController struct {
	f *flow.Flow
}

func (sc serviceController) Run(ctx context.Context) { sc.f.Run(ctx) }
func (sc serviceController) LoadSource(b []byte, args map[string]any) error {
	source, err := flow.ParseSource("", b)
	if err != nil {
		return err
	}
	return sc.f.LoadSource(source, args)
}
func (sc serviceController) Ready() bool { return sc.f.Ready() }
