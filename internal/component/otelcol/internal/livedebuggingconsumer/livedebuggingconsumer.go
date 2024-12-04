// Package livedebuggingconsumer implements an OpenTelemetry Collector consumer
// which can be used to send live debugging data to Alloy UI.
package livedebuggingconsumer

import (
	"context"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazyconsumer"
	"github.com/grafana/alloy/internal/service/livedebugging"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

type Consumer struct {
	debugDataPublisher       livedebugging.DebugDataPublisher
	componentID              livedebugging.ComponentID
	logsMarshaler            plog.Marshaler
	metricsMarshaler         pmetric.Marshaler
	tracesMarshaler          ptrace.Marshaler
	targetComponentIDsMetric []string
	targetComponentIDsLog    []string
	targetComponentIDsTraces []string
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

func (c *Consumer) SetTargetConsumers(metric, log, trace []otelcol.Consumer) {
	c.targetComponentIDsMetric = extractIds(metric)
	c.targetComponentIDsLog = extractIds(log)
	c.targetComponentIDsTraces = extractIds(trace)
}

func extractIds(consumers []otelcol.Consumer) []string {
	ids := make([]string, 0)
	for _, cons := range consumers {
		if lazy, ok := cons.(*lazyconsumer.Consumer); ok {
			ids = append(ids, lazy.ComponentID())
		}
	}
	return ids
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
		c.debugDataPublisher.Publish(c.componentID, livedebugging.FeedData{
			ComponentID:        c.componentID,
			TargetComponentIDs: c.targetComponentIDsTraces,
			Type:               livedebugging.OtelTrace,
			Count:              td.SpanCount(),
			Data:               string(data),
		})
	}
	return nil
}

// ConsumeMetrics implements otelcol.ConsumeMetrics.
func (c *Consumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if c.debugDataPublisher.IsActive(c.componentID) {
		data, _ := c.metricsMarshaler.MarshalMetrics(md)
		c.debugDataPublisher.Publish(c.componentID, livedebugging.FeedData{
			ComponentID:        c.componentID,
			TargetComponentIDs: c.targetComponentIDsMetric,
			Type:               livedebugging.OtelMetric,
			Count:              md.MetricCount(),
			Data:               string(data),
		})
	}
	return nil
}

// ConsumeLogs implements otelcol.ConsumeLogs.
func (c *Consumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if c.debugDataPublisher.IsActive(c.componentID) {
		data, _ := c.logsMarshaler.MarshalLogs(ld)
		c.debugDataPublisher.Publish(c.componentID, livedebugging.FeedData{
			ComponentID:        c.componentID,
			TargetComponentIDs: c.targetComponentIDsLog,
			Type:               livedebugging.OtelLog,
			Count:              ld.LogRecordCount(),
			Data:               string(data),
		})
	}
	return nil
}
