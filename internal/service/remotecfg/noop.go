package remotecfg

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	collectorv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
)

type noopClient struct{}

// GetConfig returns the collector's configuration.
func (c noopClient) GetConfig(context.Context, *connect.Request[collectorv1.GetConfigRequest]) (*connect.Response[collectorv1.GetConfigResponse], error) {
	return nil, errors.New("noop client")
}

// RegisterCollector checks in the current collector to the API on startup.
func (c noopClient) RegisterCollector(context.Context, *connect.Request[collectorv1.RegisterCollectorRequest]) (*connect.Response[collectorv1.RegisterCollectorResponse], error) {
	return nil, errors.New("noop client")
}

// UnregisterCollector checks out the current collector to the API on shutdown.
func (c noopClient) UnregisterCollector(context.Context, *connect.Request[collectorv1.UnregisterCollectorRequest]) (*connect.Response[collectorv1.UnregisterCollectorResponse], error) {
	return nil, errors.New("noop client")
}
