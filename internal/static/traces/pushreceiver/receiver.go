package pushreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	otelreceiver "go.opentelemetry.io/collector/receiver"
)

type receiver struct{}

func (r *receiver) Start(_ context.Context, _ component.Host) error {
	return nil
}

func (r *receiver) Shutdown(_ context.Context) error {
	return nil
}

func newPushReceiver() (otelreceiver.Traces, error) {
	return &receiver{}, nil
}
