package source

import (
	"context"
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
