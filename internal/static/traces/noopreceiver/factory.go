package noopreceiver

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/receiver"
)

const (
	// TypeStr for noop receiver.
	TypeStr = "noop"
)

// NewFactory creates noop receiver factory.
func NewFactory() receiver.Factory {
	return receiver.NewFactory(
		TypeStr,
		createDefaultConfig,
		receiver.WithMetrics(createMetricsReceiver, component.StabilityLevelUndefined),
	)
}

func createDefaultConfig() component.Config {
	return &struct{}{}
}

// noop receiver is used in the metrics pipeline so we need to
// implement a metrics receiver.
func createMetricsReceiver(
	_ context.Context,
	_ receiver.CreateSettings,
	_ component.Config,
	_ consumer.Metrics,
) (receiver.Metrics, error) {

	return newNoopReceiver(nil, nil, nil), nil
}
