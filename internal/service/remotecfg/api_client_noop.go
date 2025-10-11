package remotecfg

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	collectorv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
	"github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1/collectorv1connect"
)

// noopClient is a no-op implementation of the collectorv1connect.CollectorServiceClient.
// It is used when the remotecfg service is disabled.
type noopClient struct{}

var _ collectorv1connect.CollectorServiceClient = (*noopClient)(nil)

var errNoopClient = errors.New("noop client")

func newNoopClient() *noopClient {
	return &noopClient{}
}

// GetConfig returns the collector's configuration.
func (c noopClient) GetConfig(context.Context, *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
	return nil, errNoopClient
}

// RegisterCollector checks in the current collector to the API on startup.
func (c noopClient) RegisterCollector(context.Context, *connect.Request[collectorv1.RegisterCollectorRequest]) (*connect.Response[collectorv1.RegisterCollectorResponse], error) {
	return nil, errNoopClient
}

// UnregisterCollector checks out the current collector to the API on shutdown.
func (c noopClient) UnregisterCollector(context.Context, *connect.Request[collectorv1.UnregisterCollectorRequest]) (*connect.Response[collectorv1.UnregisterCollectorResponse], error) {
	return nil, errNoopClient
}
