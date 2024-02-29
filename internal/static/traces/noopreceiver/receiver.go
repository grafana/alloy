package noopreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.uber.org/zap"
)

type noopReceiver struct{}

// New creates a dummy receiver.
func newNoopReceiver(_ *zap.Logger, _ component.Config, _ consumer.Metrics) *noopReceiver {
	return &noopReceiver{}
}

// Start implements the Component interface.
func (r *noopReceiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

// Shutdown implements the Component interface.
func (r *noopReceiver) Shutdown(context.Context) error {
	return nil
}
