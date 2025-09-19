package remotecfg

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	collectorv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
	"github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1/collectorv1connect"
	"github.com/grafana/alloy/internal/component/common/config"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

// mockCollectorClient is a mock implementation of CollectorServiceClient for testing
type mockCollectorClient struct {
	mut                     sync.RWMutex
	getConfigFunc           func(context.Context, *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error)
	registerCollectorFunc   func(context.Context, *connect.Request[collectorv1.RegisterCollectorRequest]) (*connect.Response[collectorv1.RegisterCollectorResponse], error)
	unregisterCollectorFunc func(context.Context, *connect.Request[collectorv1.UnregisterCollectorRequest]) (*connect.Response[collectorv1.UnregisterCollectorResponse], error)
	getConfigCallCount      atomic.Int32
	registerCallCount       atomic.Int32
	unregisterCallCount     atomic.Int32
}

func (m *mockCollectorClient) GetConfig(ctx context.Context, req *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
	m.mut.RLock()
	defer m.mut.RUnlock()
	m.getConfigCallCount.Inc()

	if m.getConfigFunc != nil {
		return m.getConfigFunc(ctx, req)
	}

	return &connect.Response[collectorv1.GetConfigResponse]{
		Msg: &collectorv1.GetConfigResponse{
			Content: "test config",
			Hash:    "test-hash",
		},
	}, nil
}

func (m *mockCollectorClient) RegisterCollector(ctx context.Context, req *connect.Request[collectorv1.RegisterCollectorRequest]) (*connect.Response[collectorv1.RegisterCollectorResponse], error) {
	m.mut.RLock()
	defer m.mut.RUnlock()
	m.registerCallCount.Inc()

	if m.registerCollectorFunc != nil {
		return m.registerCollectorFunc(ctx, req)
	}

	return &connect.Response[collectorv1.RegisterCollectorResponse]{
		Msg: &collectorv1.RegisterCollectorResponse{},
	}, nil
}

func (m *mockCollectorClient) UnregisterCollector(ctx context.Context, req *connect.Request[collectorv1.UnregisterCollectorRequest]) (*connect.Response[collectorv1.UnregisterCollectorResponse], error) {
	m.mut.RLock()
	defer m.mut.RUnlock()
	m.unregisterCallCount.Inc()

	if m.unregisterCollectorFunc != nil {
		return m.unregisterCollectorFunc(ctx, req)
	}

	return &connect.Response[collectorv1.UnregisterCollectorResponse]{
		Msg: &collectorv1.UnregisterCollectorResponse{},
	}, nil
}

func TestNewAPIClient(t *testing.T) {
	metrics := registerMetrics(prometheus.NewRegistry())

	args := Arguments{
		URL:              "https://example.com/api",
		ID:               "test-id",
		Name:             "test-collector",
		PollFrequency:    30 * time.Second,
		HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
	}

	client, err := newAPIClient(args, metrics)

	require.NoError(t, err)
	require.NotNil(t, client)
	assert.IsType(t, &apiClient{}, client)
}

// newMockAPIClient creates a fresh mock API client for testing
func newMockAPIClient(t *testing.T) (*apiClient, *mockCollectorClient, *metrics) {
	t.Helper()
	mockClient := &mockCollectorClient{}
	reg := prometheus.NewRegistry()
	metrics := registerMetrics(reg)
	client := newAPIClientWithClient(mockClient, metrics)
	return client, mockClient, metrics
}

func TestAPIClient_GetConfig_Success(t *testing.T) {
	client, mockClient, metrics := newMockAPIClient(t)

	ctx := t.Context()
	req := &connect.Request[collectorv1.GetConfigRequest]{
		Msg: &collectorv1.GetConfigRequest{
			Id:              "test-id",
			LocalAttributes: map[string]string{"test": "value"},
			Hash:            "old-hash",
		},
	}

	// Mock successful response
	expectedResponse := &connect.Response[collectorv1.GetConfigResponse]{
		Msg: &collectorv1.GetConfigResponse{
			Content:     "new config content",
			Hash:        "new-hash",
			NotModified: false,
		},
	}
	mockClient.getConfigFunc = func(ctx context.Context, req *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
		return expectedResponse, nil
	}

	resp, err := client.GetConfig(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, expectedResponse, resp)
	assert.Equal(t, int32(1), mockClient.getConfigCallCount.Load())

	// Verify metrics were recorded
	metricDto := &dto.Metric{}
	err = metrics.getConfigTime.Write(metricDto)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), metricDto.GetHistogram().GetSampleCount())
}

func TestAPIClient_GetConfig_NotModified(t *testing.T) {
	client, mockClient, metrics := newMockAPIClient(t)

	ctx := t.Context()
	req := &connect.Request[collectorv1.GetConfigRequest]{
		Msg: &collectorv1.GetConfigRequest{
			Id:   "test-id",
			Hash: "same-hash",
		},
	}

	// Mock not modified response
	mockClient.getConfigFunc = func(ctx context.Context, req *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
		return &connect.Response[collectorv1.GetConfigResponse]{
			Msg: &collectorv1.GetConfigResponse{
				NotModified: true,
			},
		}, nil
	}

	resp, err := client.GetConfig(ctx, req)

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Equal(t, errNotModified, err)
	assert.Equal(t, int32(1), mockClient.getConfigCallCount.Load())

	// Verify metrics were still recorded
	metricDto := &dto.Metric{}
	err = metrics.getConfigTime.Write(metricDto)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), metricDto.GetHistogram().GetSampleCount())
}

func TestAPIClient_GetConfig_Error(t *testing.T) {
	client, mockClient, metrics := newMockAPIClient(t)

	ctx := t.Context()
	req := &connect.Request[collectorv1.GetConfigRequest]{
		Msg: &collectorv1.GetConfigRequest{
			Id: "test-id",
		},
	}

	expectedError := errors.New("network error")
	mockClient.getConfigFunc = func(ctx context.Context, req *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
		return nil, expectedError
	}

	resp, err := client.GetConfig(ctx, req)

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Equal(t, expectedError, err)
	assert.Equal(t, int32(1), mockClient.getConfigCallCount.Load())

	// Verify no metrics were recorded on error
	metricDto := &dto.Metric{}
	err = metrics.getConfigTime.Write(metricDto)
	require.NoError(t, err)
	assert.Equal(t, uint64(0), metricDto.GetHistogram().GetSampleCount())
}

func TestAPIClient_RegisterCollector_Success(t *testing.T) {
	client, mockClient, _ := newMockAPIClient(t)

	ctx := t.Context()
	req := &connect.Request[collectorv1.RegisterCollectorRequest]{
		Msg: &collectorv1.RegisterCollectorRequest{
			Id:              "test-id",
			LocalAttributes: map[string]string{"test": "value"},
			Name:            "test-collector",
		},
	}

	expectedResponse := &connect.Response[collectorv1.RegisterCollectorResponse]{
		Msg: &collectorv1.RegisterCollectorResponse{},
	}
	mockClient.registerCollectorFunc = func(ctx context.Context, req *connect.Request[collectorv1.RegisterCollectorRequest]) (*connect.Response[collectorv1.RegisterCollectorResponse], error) {
		return expectedResponse, nil
	}

	resp, err := client.RegisterCollector(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, expectedResponse, resp)
	assert.Equal(t, int32(1), mockClient.registerCallCount.Load())
}

func TestAPIClient_GetConfig_MetricsTiming(t *testing.T) {
	client, mockClient, metrics := newMockAPIClient(t)

	ctx := t.Context()
	req := &connect.Request[collectorv1.GetConfigRequest]{
		Msg: &collectorv1.GetConfigRequest{Id: "test-id"},
	}

	// Add a small delay to ensure measurable time
	mockClient.getConfigFunc = func(ctx context.Context, req *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
		time.Sleep(10 * time.Millisecond)
		return &connect.Response[collectorv1.GetConfigResponse]{
			Msg: &collectorv1.GetConfigResponse{
				Content:     "test",
				NotModified: false,
			},
		}, nil
	}

	_, err := client.GetConfig(ctx, req)

	require.NoError(t, err)

	// Verify timing metrics were recorded and are reasonable
	metricDto := &dto.Metric{}
	err = metrics.getConfigTime.Write(metricDto)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), metricDto.GetHistogram().GetSampleCount())
	assert.Greater(t, metricDto.GetHistogram().GetSampleSum(), 0.01) // At least 10ms
	assert.Less(t, metricDto.GetHistogram().GetSampleSum(), 1.0)     // Less than 1 second
}

func TestAPIClient_Integration(t *testing.T) {
	metrics := registerMetrics(prometheus.NewRegistry())

	args := Arguments{
		URL:              "https://example.com/api",
		ID:               "test-id",
		Name:             "test-collector",
		PollFrequency:    30 * time.Second,
		HTTPClientConfig: config.CloneDefaultHTTPClientConfig(),
	}

	client, err := newAPIClient(args, metrics)

	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.Implements(t, (*collectorv1connect.CollectorServiceClient)(nil), client)

	// Verify it's the apiClient wrapper
	assert.IsType(t, &apiClient{}, client)
}
