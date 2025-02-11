// Package livedebuggingconsumer implements an OpenTelemetry Collector consumer
// which can be used to send live debugging data to Alloy UI.
package livedebuggingconsumer

import (
	"context"
	"sync"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/grafana/alloy/internal/component/otelcol/internal/lazyconsumer"
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

	mut                      sync.RWMutex
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

// SetTargetConsumers stores the componentIDs of the next consumers
func (c *Consumer) SetTargetConsumers(metric, log, trace []otelcol.Consumer) {
	c.mut.Lock()
	defer c.mut.Unlock()
	c.targetComponentIDsMetric = extractIds(metric)
	c.targetComponentIDsLog = extractIds(log)
	c.targetComponentIDsTraces = extractIds(trace)
}

func extractIds(consumers []otelcol.Consumer) []string {
	ids := make([]string, 0, len(consumers))
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
	c.mut.RLock()
	defer c.mut.RUnlock()
	c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
		c.componentID,
		livedebugging.OtelTrace,
		uint64(td.SpanCount()),
		func() string {
			data, err := c.tracesMarshaler.MarshalTraces(td)
			if err != nil {
				return ""
			}
			return string(data)
		},
		livedebugging.WithTargetComponentIDs(c.targetComponentIDsTraces),
	))
	return nil
}

// ConsumeMetrics implements otelcol.ConsumeMetrics.
func (c *Consumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	c.mut.RLock()
	defer c.mut.RUnlock()
	c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
		c.componentID,
		livedebugging.OtelMetric,
		uint64(md.MetricCount()),
		func() string {
			data, err := c.metricsMarshaler.MarshalMetrics(md)
			if err != nil {
				return ""
			}
			return string(data)
		},
		livedebugging.WithTargetComponentIDs(c.targetComponentIDsMetric),
	))
	return nil
}

// ConsumeLogs implements otelcol.ConsumeLogs.
func (c *Consumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	c.mut.RLock()
	defer c.mut.RUnlock()
	c.debugDataPublisher.PublishIfActive(livedebugging.NewData(
		c.componentID,
		livedebugging.OtelLog,
		uint64(ld.LogRecordCount()),
		func() string {
			data, err := c.logsMarshaler.MarshalLogs(ld)
			if err != nil {
				return ""
			}
			return string(data)
		},
		livedebugging.WithTargetComponentIDs(c.targetComponentIDsLog),
	))
	return nil
}
