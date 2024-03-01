package app_agent_receiver

import (
	"context"

	"go.opentelemetry.io/collector/consumer"
)

type tracesConsumerGetter func() (consumer.Traces, error)

// TracesExporter will send traces to a traces instance
type TracesExporter struct {
	getTracesConsumer tracesConsumerGetter
}

// NewTracesExporter creates a trace exporter for the app agent receiver.
func NewTracesExporter(getTracesConsumer tracesConsumerGetter) AppAgentReceiverExporter {
	return &TracesExporter{getTracesConsumer}
}

// Name of the exporter, for logging purposes
func (te *TracesExporter) Name() string {
	return "traces exporter"
}

// Export implements the AppDataExporter interface
func (te *TracesExporter) Export(ctx context.Context, payload Payload) error {
	if payload.Traces == nil {
		return nil
	}
	consumer, err := te.getTracesConsumer()
	if err != nil {
		return err
	}
	return consumer.ConsumeTraces(ctx, payload.Traces.Traces)
}

// Static typecheck tests
var (
	_ AppAgentReceiverExporter = (*TracesExporter)(nil)
)
