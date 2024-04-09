package remotewriteexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type remoteWriteExporter struct{}

func newRemoteWriteExporter(cfg *Config) (exporter.Metrics, error) {
	// NOTE(rfratto): remotewriteexporter has been kept for config conversions,
	// but is never used, so the implementation of the component has been
	// removed.
	return &remoteWriteExporter{}, nil
}

func (e *remoteWriteExporter) Start(ctx context.Context, _ component.Host) error {
	return nil
}

func (e *remoteWriteExporter) Shutdown(ctx context.Context) error {
	return nil
}

func (e *remoteWriteExporter) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{}
}

func (e *remoteWriteExporter) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return nil
}
