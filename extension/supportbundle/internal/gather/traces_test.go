package gather

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// emitSpans records n spans through a tracer that has the processor attached.
func emitSpans(p *TraceProcessor, n int) {
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(p),
	)
	tr := tp.Tracer("test")
	for i := 0; i < n; i++ {
		_, span := tr.Start(context.Background(), "op-"+strconv.Itoa(i))
		span.SetAttributes(attribute.String("k", "v"))
		span.End()
	}
}

func TestTraceProcessorDisabledIsNoOp(t *testing.T) {
	p := NewTraceProcessor() // not enabled
	emitSpans(p, 3)

	files, err := Traces{Processor: p}.Gather(context.Background(), Options{})
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestTraceProcessorCaptures(t *testing.T) {
	p := NewTraceProcessor()
	p.Enable(8)
	emitSpans(p, 3)

	files, err := Traces{Processor: p}.Gather(context.Background(), Options{})
	require.NoError(t, err)

	m := gatherToMap(t, files)
	require.Contains(t, m, "traces.json")

	var recs []spanRecord
	require.NoError(t, json.Unmarshal(m["traces.json"], &recs))
	require.Len(t, recs, 3)
	require.Equal(t, "op-0", recs[0].Name) // oldest first
	require.Equal(t, "op-2", recs[2].Name) // newest last
	require.Equal(t, "v", recs[0].Attributes["k"])
	require.NotEmpty(t, recs[0].TraceID)
	require.NotEmpty(t, recs[0].SpanID)
}

func TestTraceProcessorEvictsOldest(t *testing.T) {
	p := NewTraceProcessor()
	p.Enable(2) // keep only the last 2
	emitSpans(p, 5)

	files, err := Traces{Processor: p}.Gather(context.Background(), Options{})
	require.NoError(t, err)

	var recs []spanRecord
	require.NoError(t, json.Unmarshal(gatherToMap(t, files)["traces.json"], &recs))
	require.Len(t, recs, 2)
	require.Equal(t, "op-3", recs[0].Name)
	require.Equal(t, "op-4", recs[1].Name)
}
