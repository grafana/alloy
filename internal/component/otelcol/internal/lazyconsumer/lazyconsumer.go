// Package lazyconsumer implements a lazy OpenTelemetry Collector consumer
// which can lazily forward request to another consumer implementation.
package lazyconsumer

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/grafana/alloy/internal/component/otelcol"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/pipeline"
)

// defaultDrainTimeout is how long Pause waits for in-flight Consume* calls, before and after cancelling them.
const defaultDrainTimeout = 5 * time.Second

// errPaused is the cancellation cause of in-flight Consume* calls that outlast drainTimeout.
var errPaused = errors.New("consumer paused while the request was in flight")

// Consumer is a lazily-loaded consumer.
type Consumer struct {
	ctx context.Context

	componentID string

	// drainTimeout bounds each of the two waits in Pause.
	drainTimeout time.Duration

	// mut guards everything below.
	mut sync.Mutex

	// resumed is set while the consumer is paused and closed by Resume. See Pause method for more info.
	resumed chan struct{}

	// inflight counts the Consume* calls in progress. drained is closed when it drops to zero while Pause waits.
	inflight int
	drained  chan struct{}

	// Cancelling inflightCtx cancels the contexts of the Consume* calls in progress.
	inflightCtx    context.Context
	cancelInflight context.CancelCauseFunc

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
	c := &Consumer{ctx: ctx, componentID: componentID, drainTimeout: defaultDrainTimeout}
	c.inflightCtx, c.cancelInflight = context.WithCancelCause(context.Background())
	return c
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

	var tracesConsumer otelconsumer.Traces
	ctx, done, err := c.begin(ctx, func() { tracesConsumer = c.tracesConsumer })
	if err != nil {
		return err
	}
	defer done()

	if tracesConsumer == nil {
		return pipeline.ErrSignalNotSupported
	}

	if tracesConsumer.Capabilities().MutatesData {
		newTraces := ptrace.NewTraces()
		td.CopyTo(newTraces)
		td = newTraces
	}
	return tracesConsumer.ConsumeTraces(ctx, td)
}

// ConsumeMetrics implements otelconsumer.Metrics.
func (c *Consumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	if c.ctx.Err() != nil {
		return c.ctx.Err()
	}

	var metricsConsumer otelconsumer.Metrics
	ctx, done, err := c.begin(ctx, func() { metricsConsumer = c.metricsConsumer })
	if err != nil {
		return err
	}
	defer done()

	if metricsConsumer == nil {
		return pipeline.ErrSignalNotSupported
	}

	if metricsConsumer.Capabilities().MutatesData {
		newMetrics := pmetric.NewMetrics()
		md.CopyTo(newMetrics)
		md = newMetrics
	}
	return metricsConsumer.ConsumeMetrics(ctx, md)
}

// ConsumeLogs implements otelconsumer.Logs.
func (c *Consumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	if c.ctx.Err() != nil {
		return c.ctx.Err()
	}

	var logsConsumer otelconsumer.Logs
	ctx, done, err := c.begin(ctx, func() { logsConsumer = c.logsConsumer })
	if err != nil {
		return err
	}
	defer done()

	if logsConsumer == nil {
		return pipeline.ErrSignalNotSupported
	}

	if logsConsumer.Capabilities().MutatesData {
		newLogs := plog.NewLogs()
		ld.CopyTo(newLogs)
		ld = newLogs
	}
	return logsConsumer.ConsumeLogs(ctx, ld)
}

// begin waits until the consumer is resumed, counts the call as in flight and runs snapshot under the lock.
// It returns the context to forward with, which Pause can cancel, and done, which the caller must call at the end.
func (c *Consumer) begin(ctx context.Context, snapshot func()) (context.Context, func(), error) {
	for {
		c.mut.Lock()
		resumed := c.resumed
		if resumed == nil {
			break
		}
		c.mut.Unlock()
		select {
		case <-resumed:
		case <-c.ctx.Done():
			return nil, nil, c.ctx.Err()
		}
	}
	defer c.mut.Unlock()

	c.inflight++
	inflightCtx := c.inflightCtx
	snapshot()

	ctx, cancel := context.WithCancelCause(ctx)
	stop := context.AfterFunc(inflightCtx, func() {
		cancel(context.Cause(inflightCtx))
	})
	done := func() {
		stop()
		cancel(nil)

		c.mut.Lock()
		defer c.mut.Unlock()
		c.inflight--
		if c.inflight == 0 && c.drained != nil {
			close(c.drained)
			c.drained = nil
		}
	}
	return ctx, done, nil
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
// It waits up to drainTimeout for in-flight Consume* calls, then cancels them and waits up to drainTimeout again.
// Pause can be called multiple times, but a single call to Resume will un-pause this consumer. Thread-safe.
func (c *Consumer) Pause() {
	c.mut.Lock()
	if c.resumed != nil {
		c.mut.Unlock()
		return // already paused
	}

	c.resumed = make(chan struct{})

	if c.inflight == 0 {
		c.mut.Unlock()
		return
	}
	drained := make(chan struct{})
	c.drained = drained
	cancelInflight := c.cancelInflight
	c.mut.Unlock()

	timer := time.NewTimer(c.drainTimeout)
	defer timer.Stop()
	select {
	case <-drained:
		return
	case <-timer.C:
	}

	cancelInflight(errPaused)
	timer.Reset(c.drainTimeout)
	select {
	case <-drained:
	case <-timer.C:
	}
}

// Resume will revert the Pause call and the consumer will continue to work. See Pause for more details.
func (c *Consumer) Resume() {
	c.mut.Lock()
	defer c.mut.Unlock()

	if c.resumed == nil {
		return // already resumed
	}

	// Calls started after this must not inherit a cancellation from the last Pause.
	if c.inflightCtx.Err() != nil {
		c.inflightCtx, c.cancelInflight = context.WithCancelCause(context.Background())
	}

	close(c.resumed) // release all waiting
	c.resumed = nil
}

// IsPaused returns whether the consumer is currently paused. See Pause for details.
func (c *Consumer) IsPaused() bool {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.resumed != nil
}
