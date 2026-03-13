package fanoutconsumer

import (
	"context"
	"errors"
	"testing"

	"github.com/grafana/alloy/internal/component/otelcol"
	"github.com/prometheus/client_golang/prometheus"
	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

// fakeConsumer implements otelcol.Consumer for all three signals.
// It optionally exposes a ComponentID for destination label tests.
type fakeConsumer struct {
	traces  *consumertest.TracesSink
	metrics *consumertest.MetricsSink
	logs    *consumertest.LogsSink

	id          string // empty = no ComponentMetadata
	mutatesData bool
}

func newFakeConsumer(id string) *fakeConsumer {
	return &fakeConsumer{
		id:      id,
		traces:  new(consumertest.TracesSink),
		metrics: new(consumertest.MetricsSink),
		logs:    new(consumertest.LogsSink),
	}
}

func (f *fakeConsumer) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{MutatesData: f.mutatesData}
}

func (f *fakeConsumer) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return f.traces.ConsumeTraces(ctx, td)
}

func (f *fakeConsumer) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return f.metrics.ConsumeMetrics(ctx, md)
}

func (f *fakeConsumer) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return f.logs.ConsumeLogs(ctx, ld)
}

func (f *fakeConsumer) ComponentID() string { return f.id }

var _ otelcol.Consumer = (*fakeConsumer)(nil)

// fakeConsumerNoID implements otelcol.Consumer but NOT ComponentMetadata.
type fakeConsumerNoID struct {
	traces  *consumertest.TracesSink
	metrics *consumertest.MetricsSink
	logs    *consumertest.LogsSink
}

func newFakeConsumerNoID() *fakeConsumerNoID {
	return &fakeConsumerNoID{
		traces:  new(consumertest.TracesSink),
		metrics: new(consumertest.MetricsSink),
		logs:    new(consumertest.LogsSink),
	}
}

func (f *fakeConsumerNoID) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{}
}

func (f *fakeConsumerNoID) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	return f.traces.ConsumeTraces(ctx, td)
}

func (f *fakeConsumerNoID) ConsumeMetrics(ctx context.Context, md pmetric.Metrics) error {
	return f.metrics.ConsumeMetrics(ctx, md)
}

func (f *fakeConsumerNoID) ConsumeLogs(ctx context.Context, ld plog.Logs) error {
	return f.logs.ConsumeLogs(ctx, ld)
}

var _ otelcol.Consumer = (*fakeConsumerNoID)(nil)

// errorConsumer always returns an error for all signal methods.
type errorConsumer struct{}

func (e *errorConsumer) Capabilities() otelconsumer.Capabilities  { return otelconsumer.Capabilities{} }
func (e *errorConsumer) ConsumeTraces(_ context.Context, _ ptrace.Traces) error {
	return errors.New("consume error")
}
func (e *errorConsumer) ConsumeMetrics(_ context.Context, _ pmetric.Metrics) error {
	return errors.New("consume error")
}
func (e *errorConsumer) ConsumeLogs(_ context.Context, _ plog.Logs) error {
	return errors.New("consume error")
}
func (e *errorConsumer) ComponentID() string { return "error-consumer" }

var _ otelcol.Consumer = (*errorConsumer)(nil)

// makeTraces creates a ptrace.Traces with the given number of spans.
func makeTraces(spanCount int) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	ss := rs.ScopeSpans().AppendEmpty()
	for i := 0; i < spanCount; i++ {
		ss.Spans().AppendEmpty()
	}
	return td
}

// makeMetrics creates a pmetric.Metrics with the given number of gauge data points.
func makeMetrics(dpCount int) pmetric.Metrics {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	for i := 0; i < dpCount; i++ {
		m := sm.Metrics().AppendEmpty()
		m.SetEmptyGauge().DataPoints().AppendEmpty()
	}
	return md
}

// makeLogs creates a plog.Logs with the given number of log records.
func makeLogs(recordCount int) plog.Logs {
	ld := plog.NewLogs()
	rl := ld.ResourceLogs().AppendEmpty()
	sl := rl.ScopeLogs().AppendEmpty()
	for i := 0; i < recordCount; i++ {
		sl.LogRecords().AppendEmpty()
	}
	return ld
}

// gatherCounter returns the float64 value of the first metric with the given label value.
func gatherCounter(t *testing.T, reg *prometheus.Registry, labelVal string) float64 {
	t.Helper()
	mf, err := reg.Gather()
	if err != nil {
		t.Fatalf("failed to gather metrics: %v", err)
	}
	for _, m := range mf {
		for _, metric := range m.GetMetric() {
			for _, lp := range metric.GetLabel() {
				if lp.GetValue() == labelVal {
					return metric.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}

func TestTracesPassthroughCountsSpans(t *testing.T) {
	reg := prometheus.NewRegistry()
	sink := newFakeConsumer("dest1")
	fanout := Traces([]otelcol.Consumer{sink}, reg)

	if err := fanout.ConsumeTraces(context.Background(), makeTraces(3)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gatherCounter(t, reg, "dest1"); got != 3 {
		t.Errorf("expected 3 spans counted, got %v", got)
	}
}

func TestTracesPassthroughNoCountOnError(t *testing.T) {
	reg := prometheus.NewRegistry()
	fanout := Traces([]otelcol.Consumer{&errorConsumer{}}, reg)

	err := fanout.ConsumeTraces(context.Background(), makeTraces(5))
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if got := gatherCounter(t, reg, "error-consumer"); got != 0 {
		t.Errorf("expected 0 spans on error, got %v", got)
	}
}

func TestTracesFanoutMultipleConsumers(t *testing.T) {
	reg := prometheus.NewRegistry()
	sink1 := newFakeConsumer("dest1")
	sink2 := newFakeConsumer("dest2")
	fanout := Traces([]otelcol.Consumer{sink1, sink2}, reg)

	if err := fanout.ConsumeTraces(context.Background(), makeTraces(4)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gatherCounter(t, reg, "dest1"); got != 4 {
		t.Errorf("expected 4 spans for dest1, got %v", got)
	}
	if got := gatherCounter(t, reg, "dest2"); got != 4 {
		t.Errorf("expected 4 spans for dest2, got %v", got)
	}
}

func TestMetricsPassthroughCountsDataPoints(t *testing.T) {
	reg := prometheus.NewRegistry()
	sink := newFakeConsumer("dest1")
	fanout := Metrics([]otelcol.Consumer{sink}, reg)

	if err := fanout.ConsumeMetrics(context.Background(), makeMetrics(7)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gatherCounter(t, reg, "dest1"); got != 7 {
		t.Errorf("expected 7 data points, got %v", got)
	}
}

func TestLogsPassthroughCountsLogRecords(t *testing.T) {
	reg := prometheus.NewRegistry()
	sink := newFakeConsumer("dest1")
	fanout := Logs([]otelcol.Consumer{sink}, reg)

	if err := fanout.ConsumeLogs(context.Background(), makeLogs(5)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gatherCounter(t, reg, "dest1"); got != 5 {
		t.Errorf("expected 5 log records, got %v", got)
	}
}

func TestTracesUndefinedDestinationFallback(t *testing.T) {
	reg := prometheus.NewRegistry()
	sink := newFakeConsumerNoID()
	fanout := Traces([]otelcol.Consumer{sink}, reg)

	if err := fanout.ConsumeTraces(context.Background(), makeTraces(2)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := gatherCounter(t, reg, "undefined"); got != 2 {
		t.Errorf("expected 2 spans under destination=undefined, got %v", got)
	}
}

func TestAlreadyRegisteredCounterReused(t *testing.T) {
	// Simulates multiple Update() calls on the same opts.Registerer.
	reg := prometheus.NewRegistry()
	sink1 := newFakeConsumer("dest1")

	// First call - registers the counter.
	fanout1 := Traces([]otelcol.Consumer{sink1}, reg)
	if err := fanout1.ConsumeTraces(context.Background(), makeTraces(3)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Second call - counter already registered; should reuse it.
	sink2 := newFakeConsumer("dest1")
	fanout2 := Traces([]otelcol.Consumer{sink2}, reg)
	if err := fanout2.ConsumeTraces(context.Background(), makeTraces(2)); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Total should be 3 + 2 = 5 since the same counter is reused.
	if got := gatherCounter(t, reg, "dest1"); got != 5 {
		t.Errorf("expected 5 total spans after two updates, got %v", got)
	}
}
