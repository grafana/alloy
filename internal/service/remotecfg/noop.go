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

// GetCollector returns information about the collector.
func (c noopClient) GetCollector(context.Context, *connect.Request[collectorv1.GetCollectorRequest]) (*connect.Response[collectorv1.Collector], error) {
	return nil, errors.New("noop client")
}

// ListCollectors returns information about all collectors.
func (c noopClient) ListCollectors(context.Context, *connect.Request[collectorv1.ListCollectorsRequest]) (*connect.Response[collectorv1.Collectors], error) {
	return nil, errors.New("noop client")
}

// CreateCollector registers a new collector.
func (c noopClient) CreateCollector(context.Context, *connect.Request[collectorv1.CreateCollectorRequest]) (*connect.Response[collectorv1.Collector], error) {
	return nil, errors.New("noop client")
}

// UpdateCollector updates an existing collector.
func (c noopClient) UpdateCollector(context.Context, *connect.Request[collectorv1.UpdateCollectorRequest]) (*connect.Response[collectorv1.Collector], error) {
	return nil, errors.New("noop client")
}

// DeleteCollector deletes an existing collector.
func (c noopClient) DeleteCollector(context.Context, *connect.Request[collectorv1.DeleteCollectorRequest]) (*connect.Response[collectorv1.DeleteCollectorResponse], error) {
	return nil, errors.New("noop client")
}
