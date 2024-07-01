// Package livedebuggingconsumer implements an OpenTelemetry Collector consumer
// which can be used to send live debugging data to Alloy UI.
package livedebuggingconsumer

import (
	"context"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/service/livedebugging"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type Consumer struct {
	debugDataPublisher livedebugging.DebugDataPublisher
	componentID        livedebugging.ComponentID
	logsMarshaler      plog.Marshaler
	metricsMarshaler   pmetric.Marshaler
	tracesMarshaler    ptrace.Marshaler
}

var _ otelcol.Consumer = (*Consumer)(nil)

func New(debugDataPublisher livedebugging.DebugDataPublisher, componentID string) *Consumer {
	return &Consumer{
		debugDataPublisher: debugDataPublisher,
		componentID:        livedebugging.ComponentID(componentID),
		logsMarshaler:      NewTextLogsMarshaler(),
		metricsMarshaler:   NewTextMetricsMarshaler(),
		tracesMarshaler:    NewTextTracesMarshaler(),
	}
}

// Capabilities implements otelcol.Consumer.
func (c *Consumer) Capabilities() otelconsumer.Capabilities {
	// streaming data should not modify the value
	return otelconsumer.Capabilities{MutatesData: false}
}

// ConsumeTraces implements otelcol.ConsumeTraces.
func (c *Consumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if c.debugDataPublisher.IsActive(c.componentID) {
		data, _ := c.tracesMarshaler.MarshalTraces(td)
		c.debugDataPublisher.Publish(c.componentID, string(data))
	}
	return nil
}

// ConsumeMetrics implements otelcol.ConsumeMetrics.
func (c *Consumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if c.debugDataPublisher.IsActive(c.componentID) {
		data, _ := c.metricsMarshaler.MarshalMetrics(md)
		c.debugDataPublisher.Publish(c.componentID, string(data))
	}
	return nil
}

// ConsumeLogs implements otelcol.ConsumeLogs.
func (c *Consumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if c.debugDataPublisher.IsActive(c.componentID) {
		data, _ := c.logsMarshaler.MarshalLogs(ld)
		c.debugDataPublisher.Publish(c.componentID, string(data))
	}
	return nil
}
