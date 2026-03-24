// Package lazyconsumer implements a lazy OpenTelemetry Collector consumer
// which can lazily forward request to another consumer implementation.
package lazyconsumer

import (
	"context"
	"sync"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
)

// Consumer is a lazily-loaded consumer.
type Consumer struct {
	ctx context.Context

	componentID string

	// pauseMut and pausedWg are used to implement Pause & Resume semantics. See Pause method for more info.
	pauseMut sync.RWMutex
	pausedWg *sync.WaitGroup

	mut             sync.RWMutex
	metricsConsumer otelconsumer.Metrics
	logsConsumer    otelconsumer.Logs
	tracesConsumer  otelconsumer.Traces
}

var (
	_ otelconsumer.Traces       = (*Consumer)(nil)
	_ otelconsumer.Metrics      = (*Consumer)(nil)
	_ otelconsumer.Logs         = (*Consumer)(nil)
	_ otelcol.ComponentMetadata = (*Consumer)(nil)
)

// New creates a new Consumer. The provided ctx is used to determine when the
// Consumer should stop accepting data; if the ctx is closed, no further data
// will be accepted.
func New(ctx context.Context, componentID string) *Consumer {
	return &Consumer{ctx: ctx, componentID: componentID}
}

// NewPaused is like New, but returns a Consumer that is paused by calling Pause method.
func NewPaused(ctx context.Context, componentID string) *Consumer {
	c := New(ctx, componentID)
	c.Pause()
	return c
}

// ComponentID returns the componentID associated with the consumer.
// TODO: find a way to decouple the lazyconsumer from the component for better abstraction.
func (c *Consumer) ComponentID() string {
	return c.componentID
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

	c.waitUntilResumed()

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

	c.waitUntilResumed()

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

	c.waitUntilResumed()

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

func (c *Consumer) waitUntilResumed() {
	c.pauseMut.RLock()
	pausedWg := c.pausedWg
	c.pauseMut.RUnlock()
	if pausedWg != nil {
		pausedWg.Wait()
	}
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
// Pause can be called multiple times, but a single call to Resume will un-pause this consumer. Thread-safe.
func (c *Consumer) Pause() {
	c.pauseMut.Lock()
	defer c.pauseMut.Unlock()

	if c.pausedWg != nil {
		return // already paused
	}

	c.pausedWg = &sync.WaitGroup{}
	c.pausedWg.Add(1)
}

// Resume will revert the Pause call and the consumer will continue to work. See Pause for more details.
func (c *Consumer) Resume() {
	c.pauseMut.Lock()
	defer c.pauseMut.Unlock()

	if c.pausedWg == nil {
		return // already resumed
	}

	c.pausedWg.Done() // release all waiting
	c.pausedWg = nil
}

// IsPaused returns whether the consumer is currently paused. See Pause for details.
func (c *Consumer) IsPaused() bool {
	c.pauseMut.RLock()
	defer c.pauseMut.RUnlock()
	return c.pausedWg != nil
}
