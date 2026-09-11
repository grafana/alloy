package gather

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// spanGrabber captures the ReadOnlySpan handed to OnEnd, for benchmarking.
type spanGrabber struct{ span sdktrace.ReadOnlySpan }

func (spanGrabber) OnStart(context.Context, sdktrace.ReadWriteSpan) {}
func (g *spanGrabber) OnEnd(s sdktrace.ReadOnlySpan)                { g.span = s }
func (spanGrabber) Shutdown(context.Context) error                  { return nil }
func (spanGrabber) ForceFlush(context.Context) error                { return nil }

func benchSpan() sdktrace.ReadOnlySpan {
	g := &spanGrabber{}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithSpanProcessor(g),
	)
	_, span := tp.Tracer("bench").Start(context.Background(), "processing batch")
	span.SetAttributes(
		attribute.String("component", "otlp"),
		attribute.String("pipeline", "traces"),
		attribute.Int("count", 512),
	)
	span.End()
	return g.span
}

// Disabled: OnEnd is a lock-free no-op.
func BenchmarkTraceOnEndDisabled(b *testing.B) {
	s := benchSpan()
	p := NewTraceProcessor()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.OnEnd(s)
	}
}

// Enabled: OnEnd snapshots the span (SpanStub copy) and stores it in the ring.
func BenchmarkTraceOnEndEnabled(b *testing.B) {
	s := benchSpan()
	p := NewTraceProcessor()
	p.Enable(4096)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.OnEnd(s)
	}
}

func BenchmarkTraceOnEndEnabledParallel(b *testing.B) {
	s := benchSpan()
	p := NewTraceProcessor()
	p.Enable(4096)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			p.OnEnd(s)
		}
	})
}
