package gather

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

func newTracer(p *TraceProcessor) trace.Tracer {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(p),
	)
	return tp.Tracer("test")
}

func emitSpan(tr trace.Tracer, name string, isErr bool) {
	_, span := tr.Start(context.Background(), name)
	span.SetAttributes(attribute.String("k", "v"))
	span.AddEvent("evt", trace.WithAttributes(attribute.Int("n", 1)))
	if isErr {
		span.SetStatus(codes.Error, "boom")
	}
	span.End()
}

func gatherReports(t *testing.T, p *TraceProcessor) []traceReport {
	t.Helper()
	files, err := Traces{Processor: p}.Gather(context.Background(), Options{})
	require.NoError(t, err)
	if len(files) == 0 {
		return nil
	}
	var reports []traceReport
	require.NoError(t, json.Unmarshal(gatherToMap(t, files)["traces.json"], &reports))
	return reports
}

func totalSamples(r traceReport) int {
	n := len(r.ErrorSamples)
	for _, recs := range r.LatencySamples {
		n += len(recs)
	}
	return n
}

func TestTraceProcessorDisabledIsNoOp(t *testing.T) {
	p := NewTraceProcessor() // not enabled
	emitSpan(newTracer(p), "op", false)
	require.Nil(t, gatherReports(t, p))
}

func TestTraceProcessorAggregatesByName(t *testing.T) {
	p := NewTraceProcessor()
	p.Enable(10)
	tr := newTracer(p)
	emitSpan(tr, "op", false)
	emitSpan(tr, "op", false)
	emitSpan(tr, "op", true) // error

	reports := gatherReports(t, p)
	require.Len(t, reports, 1)
	r := reports[0]
	require.Equal(t, "op", r.Name)
	require.Equal(t, 3, r.Count)      // exact total
	require.Equal(t, 1, r.ErrorCount) // exact total
	require.NotEmpty(t, r.LatencySamples)
	require.Len(t, r.ErrorSamples, 1)
	require.Equal(t, "Error", r.ErrorSamples[0].StatusCode)

	// A latency sample carries the rich record (attributes + events).
	var sample spanRecord
	for _, recs := range r.LatencySamples {
		if len(recs) > 0 {
			sample = recs[0]
			break
		}
	}
	require.NotEmpty(t, sample.TraceID)
	require.Equal(t, "v", sample.Attributes["k"])
	require.Len(t, sample.Events, 1)
	require.Equal(t, "evt", sample.Events[0].Name)
	require.Equal(t, "1", sample.Events[0].Attributes["n"])
}

func TestTraceProcessorNamesAreUnbounded(t *testing.T) {
	p := NewTraceProcessor()
	p.Enable(10)
	tr := newTracer(p)
	for _, name := range []string{"a", "b", "c", "d", "e"} {
		emitSpan(tr, name, false)
	}
	// No name cap: every distinct name is tracked.
	require.Len(t, gatherReports(t, p), 5)
}

func TestTraceProcessorThrottlesSamples(t *testing.T) {
	p := NewTraceProcessor()
	p.Enable(10)
	tr := newTracer(p)
	const n = 50
	for i := 0; i < n; i++ {
		emitSpan(tr, "op", false) // rapid, same name -> same latency bucket
	}

	reports := gatherReports(t, p)
	require.Len(t, reports, 1)
	require.Equal(t, n, reports[0].Count) // count is exact
	// The 1s per-bucket throttle keeps samples far below the raw count.
	require.LessOrEqual(t, totalSamples(reports[0]), 10)
	require.Positive(t, totalSamples(reports[0]))
}
