// Package lazyconsumer implements a lazy OpenTelemetry Collector consumer
// which can lazily forward request to another consumer implementation.
package lazyconsumer

import (
	"context"
	"sync"

	otelcomponent "go.opentelemetry.io/collector/component"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// Consumer is a lazily-loaded consumer.
type Consumer struct {
	ctx context.Context

	mut             sync.RWMutex
	metricsConsumer otelconsumer.Metrics
	logsConsumer    otelconsumer.Logs
	tracesConsumer  otelconsumer.Traces
}

var (
	_ otelconsumer.Traces  = (*Consumer)(nil)
	_ otelconsumer.Metrics = (*Consumer)(nil)
	_ otelconsumer.Logs    = (*Consumer)(nil)
)

// New creates a new Consumer. The provided ctx is used to determine when the
// Consumer should stop accepting data; if the ctx is closed, no further data
// will be accepted.
func New(ctx context.Context) *Consumer {
	return &Consumer{ctx: ctx}
}

// Capabilities implements otelconsumer.baseConsumer.
func (c *Consumer) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{
		// MutatesData is always set to false; the lazy consumer will check the
		// underlying consumer's capabilities prior to forwarding data and will
		// pass a copy if the underlying consumer mutates data.
		MutatesData: false,
	}
}

// ConsumeTraces implements otelconsumer.Traces.
func (c *Consumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	if c.ctx.Err() != nil {
		return c.ctx.Err()
	}

	c.mut.RLock()
	defer c.mut.RUnlock()

	if c.tracesConsumer == nil {
		return otelcomponent.ErrDataTypeIsNotSupported
	}

	if c.tracesConsumer.Capabilities().MutatesData {
		newTraces := ptrace.NewTraces()
		td.CopyTo(newTraces)
		td = newTraces
	}
	return c.tracesConsumer.ConsumeTraces(ctx, td)
}

// ConsumeMetrics implements otelconsumer.Metrics.
func (c *Consumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if c.ctx.Err() != nil {
		return c.ctx.Err()
	}

	c.mut.RLock()
	defer c.mut.RUnlock()

	if c.metricsConsumer == nil {
		return otelcomponent.ErrDataTypeIsNotSupported
	}

	if c.metricsConsumer.Capabilities().MutatesData {
		newMetrics := pmetric.NewMetrics()
		md.CopyTo(newMetrics)
		md = newMetrics
	}
	return c.metricsConsumer.ConsumeMetrics(ctx, md)
}

// ConsumeLogs implements otelconsumer.Logs.
func (c *Consumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if c.ctx.Err() != nil {
		return c.ctx.Err()
	}

	c.mut.RLock()
	defer c.mut.RUnlock()

	if c.logsConsumer == nil {
		return otelcomponent.ErrDataTypeIsNotSupported
	}

	if c.logsConsumer.Capabilities().MutatesData {
		newLogs := plog.NewLogs()
		ld.CopyTo(newLogs)
		ld = newLogs
	}
	return c.logsConsumer.ConsumeLogs(ctx, ld)
}

// SetConsumers updates the internal consumers that Consumer will forward data
// to. It is valid for any combination of m, l, and t to be nil.
func (c *Consumer) SetConsumers(t otelconsumer.Traces, m otelconsumer.Metrics, l otelconsumer.Logs) {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.metricsConsumer = m
	c.logsConsumer = l
	c.tracesConsumer = t
}
