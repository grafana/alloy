package source

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// testConsumer records what it receives. It implements otelcol.Consumer.
type testConsumer struct {
	mu      sync.Mutex
	metrics []pmetric.Metrics
	logs    []plog.Logs
}

func (c *testConsumer) Capabilities() consumer.Capabilities { return consumer.Capabilities{} }

func (c *testConsumer) ConsumeTraces(context.Context, ptrace.Traces) error { return nil }

func (c *testConsumer) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.metrics = append(c.metrics, md)
	return nil
}

func (c *testConsumer) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logs = append(c.logs, ld)
	return nil
}

func (c *testConsumer) snapshot() ([]pmetric.Metrics, []plog.Logs) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]pmetric.Metrics(nil), c.metrics...), append([]plog.Logs(nil), c.logs...)
}

// pointsFor counts the data points that c received for the query instance.
// It counts the stale markers apart.
func (c *testConsumer) pointsFor(instance string) (points, stale int) {
	ms, _ := c.snapshot()
	for _, md := range ms {
		for _, rm := range md.ResourceMetrics().All() {
			if v, ok := rm.Resource().Attributes().Get("service.instance.id"); !ok || v.Str() != instance {
				continue
			}
			for _, sm := range rm.ScopeMetrics().All() {
				for _, m := range sm.Metrics().All() {
					for _, dp := range m.Gauge().DataPoints().All() {
						points++
						if dp.Flags().NoRecordedValue() {
							stale++
						}
					}
				}
			}
		}
	}
	return points, stale
}

// logRecorder is a slog.Handler that keeps each record as one line of text.
// Tests use it to check what each log level shows.
type logRecorder struct {
	mu    sync.Mutex
	lines map[slog.Level][]string
}

func newLogRecorder() *logRecorder { return &logRecorder{lines: map[slog.Level][]string{}} }

func (r *logRecorder) Enabled(context.Context, slog.Level) bool { return true }

func (r *logRecorder) Handle(_ context.Context, rec slog.Record) error {
	var b strings.Builder
	b.WriteString(rec.Message)
	rec.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%q", a.Key, a.Value.String())
		return true
	})
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines[rec.Level] = append(r.lines[rec.Level], b.String())
	return nil
}

// WithAttrs and WithGroup drop the attributes. This package does not use them.
func (r *logRecorder) WithAttrs([]slog.Attr) slog.Handler { return r }
func (r *logRecorder) WithGroup(string) slog.Handler      { return r }

// at returns a copy of the lines logged at level.
func (r *logRecorder) at(level slog.Level) []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.lines[level]...)
}
