package remotecfg

import (
	"context"
	"time"

	"connectrpc.com/connect"
	collectorv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
	"github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1/collectorv1connect"
	"github.com/grafana/alloy/internal/useragent"
	commonconfig "github.com/prometheus/common/config"
)

var userAgent = useragent.Get()

// Package-level function for creating API clients - can be replaced in tests to
// use a mock client which doesn't make any API calls.
var createAPIClient = newAPIClient

// apiClient is a wrapper around the collectorv1connect.CollectorServiceClient that
// provides metrics and error handling.
type apiClient struct {
	client  collectorv1connect.CollectorServiceClient
	metrics *metrics
}

var _ collectorv1connect.CollectorServiceClient = (*apiClient)(nil)

// newAPIClient creates a CollectorServiceClient instance with metrics wrapper based on the provided Arguments configuration.
func newAPIClient(args Arguments, metrics *metrics) (collectorv1connect.CollectorServiceClient, error) {
	client, err := newCollectorClient(args)
	if err != nil {
		return nil, err
	}
	return newAPIClientWithClient(client, metrics), nil
}

// newAPIClientWithClient creates a metrics-wrapped apiClient from an existing CollectorServiceClient.
// This is primarily used for testing with mock clients.
func newAPIClientWithClient(client collectorv1connect.CollectorServiceClient, metrics *metrics) *apiClient {
	return &apiClient{
		client:  client,
		metrics: metrics,
	}
}

// newCollectorClient creates a CollectorServiceClient instance based on the provided Arguments configuration.
func newCollectorClient(args Arguments) (collectorv1connect.CollectorServiceClient, error) {
	httpClient, err := commonconfig.NewClientFromConfig(*args.HTTPClientConfig.Convert(), "remoteconfig")
	if err != nil {
		return nil, err
	}
	return collectorv1connect.NewCollectorServiceClient(
		httpClient,
		args.URL,
		connect.WithInterceptors(newAgentInterceptor()),
	), nil
}

func (c *apiClient) GetConfig(ctx context.Context, req *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
	start := time.Now()
	resp, err := c.client.GetConfig(ctx, req)
	if err != nil {
		return nil, err
	}
	c.metrics.getConfigTime.Observe(time.Since(start).Seconds())
	if resp.Msg.NotModified {
		return nil, errNotModified
	}

	return resp, nil
}

func (c *apiClient) RegisterCollector(ctx context.Context, req *connect.Request[collectorv1.RegisterCollectorRequest]) (*connect.Response[collectorv1.RegisterCollectorResponse], error) {
	resp, err := c.client.RegisterCollector(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *apiClient) UnregisterCollector(ctx context.Context, req *connect.Request[collectorv1.UnregisterCollectorRequest]) (*connect.Response[collectorv1.UnregisterCollectorResponse], error) {
	resp, err := c.client.UnregisterCollector(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// agentInterceptor adds User-Agent headers to requests
type agentInterceptor struct {
	agent string
}

func newAgentInterceptor() *agentInterceptor {
	return &agentInterceptor{
		agent: userAgent,
	}
}

func (i *agentInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		req.Header().Set("User-Agent", i.agent)
		return next(ctx, req)
	}
}

func (i *agentInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return func(ctx context.Context, spec connect.Spec) connect.StreamingClientConn {
		conn := next(ctx, spec)
		conn.RequestHeader().Set("User-Agent", i.agent)
		return conn
	}
}

func (i *agentInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return next
}
