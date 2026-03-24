package remotecfg

import (
	"testing"

	"connectrpc.com/connect"
	collectorv1 "github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1"
	"github.com/grafana/alloy-remote-config/api/gen/proto/go/collector/v1/collectorv1connect"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNoopClient(t *testing.T) {
	client := newNoopClient()

	require.NotNil(t, client)
	assert.IsType(t, &noopClient{}, client)
}

func TestNoopClient_InterfaceCompliance(t *testing.T) {
	var _ collectorv1connect.CollectorServiceClient = (*noopClient)(nil)

	client := newNoopClient()
	var _ collectorv1connect.CollectorServiceClient = client
}

func TestNoopClient_GetConfig(t *testing.T) {
	client := newNoopClient()
	ctx := t.Context()

	req := &connect.Request[collectorv1.GetConfigRequest]{
		Msg: &collectorv1.GetConfigRequest{
			Id:              "test-id",
			LocalAttributes: map[string]string{"test": "value"},
			Hash:            "test-hash",
		},
	}

	resp, err := client.GetConfig(ctx, req)

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Equal(t, errNoopClient, err)
	assert.Equal(t, "noop client", err.Error())
}

func TestNoopClient_RegisterCollector(t *testing.T) {
	client := newNoopClient()
	ctx := t.Context()

	req := &connect.Request[collectorv1.RegisterCollectorRequest]{
		Msg: &collectorv1.RegisterCollectorRequest{
			Id:              "test-id",
			LocalAttributes: map[string]string{"test": "value"},
			Name:            "test-collector",
		},
	}

	resp, err := client.RegisterCollector(ctx, req)

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Equal(t, errNoopClient, err)
	assert.Equal(t, "noop client", err.Error())
}

func TestNoopClient_UnregisterCollector(t *testing.T) {
	client := newNoopClient()
	ctx := t.Context()

	req := &connect.Request[collectorv1.UnregisterCollectorRequest]{
		Msg: &collectorv1.UnregisterCollectorRequest{
			Id: "test-id",
		},
	}

	resp, err := client.UnregisterCollector(ctx, req)

	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Equal(t, errNoopClient, err)
	assert.Equal(t, "noop client", err.Error())
}

func TestErrNoopClient(t *testing.T) {
	assert.Equal(t, "noop client", errNoopClient.Error())
}
