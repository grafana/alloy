package services_test

import (
	"context"
	"errors"
	"testing"

	"github.com/grafana/ckit/peer"
	"github.com/grafana/ckit/shard"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/featuregate"
	"github.com/grafana/alloy/internal/service"
	"github.com/grafana/alloy/internal/service/cluster"
	graphqlservices "github.com/grafana/alloy/internal/service/graphql/services"
	httpservice "github.com/grafana/alloy/internal/service/http"
)

func TestClusterReturnsPeersFromClusterService(t *testing.T) {
	host := newServiceHost()
	host.services[cluster.ServiceName] = fakeService{
		definition: service.Definition{Name: cluster.ServiceName},
		data: fakeCluster{peers: []peer.Peer{{
			Name:  "self",
			Addr:  "127.0.0.1:12345",
			State: peer.StateParticipant,
			Self:  true,
		}}},
	}

	got, err := graphqlservices.Cluster(host)

	require.NoError(t, err)
	require.NotNil(t, got)
	require.True(t, got.Enabled)
	require.Equal(t, "self", got.Peers[0].Name)
	require.Equal(t, "participant", got.Peers[0].State)
	require.True(t, got.Peers[0].IsSelf)
}

func TestClusterReturnsNilAndErrorWhenServiceMissing(t *testing.T) {
	got, err := graphqlservices.Cluster(newServiceHost())

	require.Nil(t, got)
	require.Error(t, err)
}

func TestServicesExposeDefinitionsConsumersAndRuntimeFlags(t *testing.T) {
	host := newServiceHost()
	label := fakeService{definition: service.Definition{
		Name:      "labelstore",
		DependsOn: []string{httpservice.ServiceName},
		Stability: featuregate.StabilityGenerallyAvailable,
	}}
	host.consumers["labelstore"] = []service.Consumer{{Type: service.ConsumerTypeService, ID: "ui"}}

	got := graphqlservices.Services(host, []service.Service{label})

	require.Len(t, got, 1)
	require.Equal(t, "labelstore", got[0].GetName())
	require.Equal(t, []string{httpservice.ServiceName}, got[0].GetDependsOn())
	require.Equal(t, "generally-available", got[0].GetStability())
	require.False(t, got[0].GetHasRuntimeData())
	require.Equal(t, "ui", got[0].GetConsumers()[0].ID)
}

func TestHTTPServiceExposesRuntimeDataAndComponentPath(t *testing.T) {
	host := newServiceHost()
	httpSvc := fakeService{
		definition: service.Definition{Name: httpservice.ServiceName, Stability: featuregate.StabilityGenerallyAvailable},
		data: httpservice.Data{
			HTTPListenAddr:   "127.0.0.1:12345",
			MemoryListenAddr: "alloy.internal:12345",
			BaseHTTPPath:     "/api/v0/component/",
		},
	}

	got := graphqlservices.Services(host, []service.Service{httpSvc})

	require.Len(t, got, 1)
	httpGot, ok := got[0].(*graphqlservices.HTTPService)
	require.True(t, ok)
	require.True(t, httpGot.GetHasRuntimeData())
	require.Equal(t, "127.0.0.1:12345", httpGot.HTTPListenAddr)
	require.Equal(t, "alloy.internal:12345", httpGot.MemoryListenAddr)
	require.Equal(t, "/api/v0/component/", httpGot.BaseHTTPPath)
	require.Equal(t, "/api/v0/component/prometheus.scrape.default/", graphqlservices.ComponentHTTPPath(httpGot, "prometheus.scrape.default"))
}

func TestServiceReturnsNilWhenMissing(t *testing.T) {
	got := graphqlservices.Service(newServiceHost(), nil, "missing")

	require.Nil(t, got)
}

type serviceHost struct {
	services  map[string]service.Service
	consumers map[string][]service.Consumer
}

func newServiceHost() *serviceHost {
	return &serviceHost{
		services:  map[string]service.Service{},
		consumers: map[string][]service.Consumer{},
	}
}

func (h *serviceHost) GetComponent(component.ID, component.InfoOptions) (*component.Info, error) {
	return nil, component.ErrComponentNotFound
}

func (h *serviceHost) ListComponents(string, component.InfoOptions) ([]*component.Info, error) {
	return nil, nil
}

func (h *serviceHost) GetService(name string) (service.Service, bool) {
	svc, ok := h.services[name]
	return svc, ok
}

func (h *serviceHost) GetServiceConsumers(name string) []service.Consumer {
	return h.consumers[name]
}

func (h *serviceHost) NewController(string) (service.Controller, error) {
	return nil, errors.New("not implemented")
}

type fakeService struct {
	definition service.Definition
	data       any
}

func (f fakeService) Definition() service.Definition { return f.definition }
func (f fakeService) Run(context.Context, service.Host) error {
	return nil
}
func (f fakeService) Update(any) error { return nil }
func (f fakeService) Data() any        { return f.data }

type fakeCluster struct {
	peers []peer.Peer
}

func (f fakeCluster) Lookup(shard.Key, int, shard.Op) ([]peer.Peer, error) { return nil, nil }
func (f fakeCluster) Peers() []peer.Peer                                   { return f.peers }
func (f fakeCluster) Ready() bool                                          { return true }
func (f fakeCluster) Enabled() bool                                        { return true }
