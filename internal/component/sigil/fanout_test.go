package sigil

import (
	"context"
	"testing"

	sigilv1 "github.com/grafana/sigil-sdk/go/proto/sigil/v1"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

type fanOutTestReceiver struct {
	calls atomic.Int32
}

func (r *fanOutTestReceiver) ExportGenerations(context.Context, *GenerationsRequest) (*GenerationsResponse, error) {
	r.calls.Add(1)
	return &GenerationsResponse{}, nil
}

func TestFanOutRejectsNilReceiver(t *testing.T) {
	req := &GenerationsRequest{
		Request: &sigilv1.ExportGenerationsRequest{},
	}

	resp, err := FanOut(context.Background(), req, []GenerationsReceiver{nil}, nil, FanOutMetrics{})
	require.Error(t, err)
	require.Nil(t, resp)
}

func TestFanOutRejectsNilReceiverBeforeStartingBranches(t *testing.T) {
	recv := &fanOutTestReceiver{}
	req := &GenerationsRequest{
		Request: &sigilv1.ExportGenerationsRequest{},
	}

	resp, err := FanOut(context.Background(), req, []GenerationsReceiver{recv, nil}, nil, FanOutMetrics{})
	require.Error(t, err)
	require.Nil(t, resp)
	require.Equal(t, int32(0), recv.calls.Load())
}
