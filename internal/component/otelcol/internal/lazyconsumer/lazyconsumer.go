// Package lazyconsumer implements a lazy OpenTelemetry Collector consumer
// which can lazily forward request to another consumer implementation.
package lazyconsumer

import (
	"context"
	"sync"

	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
)

// Consumer is a lazily-loaded consumer.
type Consumer struct {
	ctx context.Context

	// pauseMut is used to implement Pause & Resume semantics. When a write lock is held - this consumer is paused.
	// See Pause method for more info.
	pauseMut sync.RWMutex

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

// NewPaused is like New, but returns a Consumer that is paused by calling Pause method.
func NewPaused(ctx context.Context) *Consumer {
	c := New(ctx)
	c.Pause()
	return c
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

	c.pauseMut.RLock() // wait until resumed
	defer c.pauseMut.RUnlock()

	c.mut.RLock()
	defer c.mut.RUnlock()

	if c.tracesConsumer == nil {
		return pipeline.ErrSignalNotSupported
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

	c.pauseMut.RLock() // wait until resumed
	defer c.pauseMut.RUnlock()

	c.mut.RLock()
	defer c.mut.RUnlock()

	if c.metricsConsumer == nil {
		return pipeline.ErrSignalNotSupported
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

	c.pauseMut.RLock() // wait until resumed
	defer c.pauseMut.RUnlock()

	c.mut.RLock()
	defer c.mut.RUnlock()

	if c.logsConsumer == nil {
		return pipeline.ErrSignalNotSupported
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

// Pause will stop the consumer until Resume is called. While paused, the calls to Consume* methods will block.
// After calling Pause once, it must not be called again until Resume is called.
func (c *Consumer) Pause() {
	c.pauseMut.Lock()
}

// Resume will revert the Pause call and the consumer will continue to work. Resume must not be called if Pause wasn't
// called before. See Pause for more details.
func (c *Consumer) Resume() {
	c.pauseMut.Unlock()
}
