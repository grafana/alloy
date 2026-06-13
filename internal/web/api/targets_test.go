package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/service"
	"github.com/stretchr/testify/require"
)

type mockHost struct {
	components []*component.Info
}

func (m *mockHost) GetComponent(id component.ID, opts component.InfoOptions) (*component.Info, error) {
	for _, c := range m.components {
		if c.ID.String() == id.String() {
			return c, nil
		}
	}
	return nil, component.ErrComponentNotFound
}

func (m *mockHost) ListComponents(moduleID string, opts component.InfoOptions) ([]*component.Info, error) {
	return m.components, nil
}

func (m *mockHost) GetService(name string) (service.Service, bool) {
	return nil, false
}

func (m *mockHost) GetServiceConsumers(serviceName string) []service.Consumer {
	return nil
}

func (m *mockHost) NewController(id string) service.Controller {
	return nil
}

type mockTargetsProvider struct {
	targets []component.TargetInfo
}

func (m *mockTargetsProvider) GetTargets() []component.TargetInfo {
	return m.targets
}

func (m *mockTargetsProvider) Run(ctx context.Context) error { return nil }
func (m *mockTargetsProvider) Update(args component.Arguments) error { return nil }

func TestGetTargetsHandler_NoComponents(t *testing.T) {
	host := &mockHost{components: []*component.Info{}}

	req := httptest.NewRequest(http.MethodGet, "/api/v0/targets", nil)
	w := httptest.NewRecorder()

	handler := getTargetsHandler(host)
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response TargetsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "success", response.Status)
	require.Empty(t, response.Data)
}

func TestGetTargetsHandler_WithTargets(t *testing.T) {
	provider := &mockTargetsProvider{
		targets: []component.TargetInfo{
			{
				JobName:            "test-job",
				Endpoint:           "http://localhost:9090/metrics",
				State:              "up",
				Labels:             map[string]string{"job": "test-job", "instance": "localhost:9090"},
				DiscoveredLabels:   map[string]string{"__address__": "localhost:9090"},
				LastScrape:         time.Now().Add(-30 * time.Second),
				LastScrapeDuration: 100 * time.Millisecond,
				LastError:          "",
			},
			{
				JobName:            "test-job",
				Endpoint:           "http://localhost:9091/metrics",
				State:              "down",
				Labels:             map[string]string{"job": "test-job", "instance": "localhost:9091"},
				DiscoveredLabels:   map[string]string{"__address__": "localhost:9091"},
				LastScrape:         time.Now().Add(-60 * time.Second),
				LastScrapeDuration: 5 * time.Second,
				LastError:          "connection refused",
			},
		},
	}

	host := &mockHost{
		components: []*component.Info{
			{
				ID:            component.ID{LocalID: "prometheus.scrape.default"},
				ComponentName: "prometheus.scrape",
				Component:     provider,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v0/targets", nil)
	w := httptest.NewRecorder()

	handler := getTargetsHandler(host)
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response TargetsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Equal(t, "success", response.Status)
	require.Len(t, response.Data, 2)

	// Check first target
	require.Equal(t, "prometheus.scrape.default", response.Data[0].ComponentID)
	require.Equal(t, "test-job", response.Data[0].JobName)
	require.Equal(t, "http://localhost:9090/metrics", response.Data[0].URL)
	require.Equal(t, "up", response.Data[0].Health)
	require.Equal(t, "100ms", response.Data[0].LastScrapeDuration)
	require.Empty(t, response.Data[0].LastError)

	// Check second target
	require.Equal(t, "down", response.Data[1].Health)
	require.Equal(t, "connection refused", response.Data[1].LastError)
}

func TestGetTargetsHandler_FilterByJob(t *testing.T) {
	provider := &mockTargetsProvider{
		targets: []component.TargetInfo{
			{
				JobName:  "job-a",
				Endpoint: "http://localhost:9090/metrics",
				State:    "up",
			},
			{
				JobName:  "job-b",
				Endpoint: "http://localhost:9091/metrics",
				State:    "up",
			},
		},
	}

	host := &mockHost{
		components: []*component.Info{
			{
				ID:            component.ID{LocalID: "prometheus.scrape.default"},
				ComponentName: "prometheus.scrape",
				Component:     provider,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v0/targets?job=job-a", nil)
	w := httptest.NewRecorder()

	handler := getTargetsHandler(host)
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response TargetsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Data, 1)
	require.Equal(t, "job-a", response.Data[0].JobName)
}

func TestGetTargetsHandler_FilterByHealth(t *testing.T) {
	provider := &mockTargetsProvider{
		targets: []component.TargetInfo{
			{
				JobName:  "test-job",
				Endpoint: "http://localhost:9090/metrics",
				State:    "up",
			},
			{
				JobName:  "test-job",
				Endpoint: "http://localhost:9091/metrics",
				State:    "down",
			},
		},
	}

	host := &mockHost{
		components: []*component.Info{
			{
				ID:            component.ID{LocalID: "prometheus.scrape.default"},
				ComponentName: "prometheus.scrape",
				Component:     provider,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v0/targets?health=down", nil)
	w := httptest.NewRecorder()

	handler := getTargetsHandler(host)
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response TargetsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Data, 1)
	require.Equal(t, "down", response.Data[0].Health)
}

func TestGetTargetsHandler_FilterByComponent(t *testing.T) {
	providerA := &mockTargetsProvider{
		targets: []component.TargetInfo{
			{
				JobName:  "job-a",
				Endpoint: "http://localhost:9090/metrics",
				State:    "up",
			},
		},
	}
	providerB := &mockTargetsProvider{
		targets: []component.TargetInfo{
			{
				JobName:  "job-b",
				Endpoint: "http://localhost:9091/metrics",
				State:    "up",
			},
		},
	}

	host := &mockHost{
		components: []*component.Info{
			{
				ID:            component.ID{LocalID: "prometheus.scrape.a"},
				ComponentName: "prometheus.scrape",
				Component:     providerA,
			},
			{
				ID:            component.ID{LocalID: "prometheus.scrape.b"},
				ComponentName: "prometheus.scrape",
				Component:     providerB,
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v0/targets?component=prometheus.scrape.a", nil)
	w := httptest.NewRecorder()

	handler := getTargetsHandler(host)
	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response TargetsResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)
	require.Len(t, response.Data, 1)
	require.Equal(t, "prometheus.scrape.a", response.Data[0].ComponentID)
	require.Equal(t, "job-a", response.Data[0].JobName)
}

func TestTargetsRouteRegistration(t *testing.T) {
	host := &mockHost{components: []*component.Info{}}
	api := NewAlloyAPI(host, nil, nil)

	r := mux.NewRouter()
	api.RegisterRoutes("/api/v0", r)

	// Test that the route is registered
	req := httptest.NewRequest(http.MethodGet, "/api/v0/targets", nil)
	match := &mux.RouteMatch{}
	require.True(t, r.Match(req, match))
}
