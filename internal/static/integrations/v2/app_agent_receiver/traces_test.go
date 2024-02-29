package app_agent_receiver

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type mockTracesConsumer struct {
	consumed []ptrace.Traces
}

func (c *mockTracesConsumer) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: false}
}

func (c *mockTracesConsumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	c.consumed = append(c.consumed, td)
	return nil
}

func Test_exportTraces_success(t *testing.T) {
	ctx := context.Background()
	tracesConsumer := &mockTracesConsumer{}
	exporter := NewTracesExporter(func() (consumer.Traces, error) { return tracesConsumer, nil })
	payload := loadTestPayload(t)
	err := exporter.Export(ctx, payload)
	require.NoError(t, err)
	require.Len(t, tracesConsumer.consumed, 1)
}

func Test_exportTraces_noTracesInpayload(t *testing.T) {
	ctx := context.Background()
	tracesConsumer := &mockTracesConsumer{consumed: nil}
	exporter := NewTracesExporter(func() (consumer.Traces, error) { return tracesConsumer, nil })
	payload := loadTestPayload(t)
	payload.Traces = nil
	err := exporter.Export(ctx, payload)
	require.NoError(t, err)
	require.Len(t, tracesConsumer.consumed, 0)
}

func Test_exportTraces_noConsumer(t *testing.T) {
	ctx := context.Background()
	exporter := NewTracesExporter(func() (consumer.Traces, error) { return nil, errors.New("it dont work") })
	payload := loadTestPayload(t)
	err := exporter.Export(ctx, payload)
	require.Error(t, err, "it don't work")
}
