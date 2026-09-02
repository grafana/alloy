package gather

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/atomic"
)

var _ sdktrace.SpanProcessor = (*TraceProcessor)(nil)

// spanRing keeps the most recent N spans, evicting the oldest. One mutex guards
// it, like the log ring. It holds the SDK's ReadOnlySpan directly; that is safe
// to retain after OnEnd (the SDK's own BatchSpanProcessor queues ReadOnlySpans
// and reads them later). Fields are read at snapshot time, not on the hot path.
type spanRing struct {
	mu   sync.Mutex
	buf  []sdktrace.ReadOnlySpan
	pos  int
	full bool
}

func newSpanRing(n int) *spanRing { return &spanRing{buf: make([]sdktrace.ReadOnlySpan, n)} }

func (r *spanRing) add(s sdktrace.ReadOnlySpan) {
	r.mu.Lock()
	r.buf[r.pos] = s
	r.pos = (r.pos + 1) % len(r.buf)
	if r.pos == 0 {
		r.full = true
	}
	r.mu.Unlock()
}

func (r *spanRing) snapshot() []sdktrace.ReadOnlySpan {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		out := make([]sdktrace.ReadOnlySpan, r.pos)
		copy(out, r.buf[:r.pos])
		return out
	}
	out := make([]sdktrace.ReadOnlySpan, len(r.buf))
	n := copy(out, r.buf[r.pos:])
	copy(out[n:], r.buf[:r.pos])
	return out
}

// TraceProcessor is an SDK span processor that keeps the most recent collector
// spans (self-observability traces) for the bundle. The extension registers it
// on the collector's tracer provider, the way the zpages extension does. Until
// a size is set, OnEnd is a lock-free no-op, so tracing pays nothing.
type TraceProcessor struct {
	ring atomic.Pointer[spanRing]
}

// NewTraceProcessor returns a disabled processor.
func NewTraceProcessor() *TraceProcessor { return &TraceProcessor{} }

// Enable turns on capture with a ring of the given number of spans. A size of 0
// leaves it disabled.
func (p *TraceProcessor) Enable(spans int) {
	if spans > 0 {
		p.ring.Store(newSpanRing(spans))
	}
}

func (p *TraceProcessor) OnStart(context.Context, sdktrace.ReadWriteSpan) {}

func (p *TraceProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	if r := p.ring.Load(); r != nil {
		r.add(s)
	}
}

func (p *TraceProcessor) Shutdown(context.Context) error   { return nil }
func (p *TraceProcessor) ForceFlush(context.Context) error { return nil }

func (p *TraceProcessor) snapshot() []sdktrace.ReadOnlySpan {
	if r := p.ring.Load(); r != nil {
		return r.snapshot()
	}
	return nil
}

// spanRecord is the JSON shape written to traces.json. It is a flat, greppable
// projection of a span, not the full SDK structure.
type spanRecord struct {
	Name          string            `json:"name"`
	TraceID       string            `json:"trace_id"`
	SpanID        string            `json:"span_id"`
	ParentSpanID  string            `json:"parent_span_id,omitempty"`
	Kind          string            `json:"kind"`
	Scope         string            `json:"scope,omitempty"`
	StartTime     time.Time         `json:"start_time"`
	EndTime       time.Time         `json:"end_time"`
	DurationMS    float64           `json:"duration_ms"`
	StatusCode    string            `json:"status_code,omitempty"`
	StatusMessage string            `json:"status_message,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
}

func toSpanRecord(s sdktrace.ReadOnlySpan) spanRecord {
	sc := s.SpanContext()
	rec := spanRecord{
		Name:       s.Name(),
		TraceID:    sc.TraceID().String(),
		SpanID:     sc.SpanID().String(),
		Kind:       s.SpanKind().String(),
		Scope:      s.InstrumentationScope().Name,
		StartTime:  s.StartTime(),
		EndTime:    s.EndTime(),
		DurationMS: float64(s.EndTime().Sub(s.StartTime()).Microseconds()) / 1000.0,
	}
	if parent := s.Parent(); parent.HasSpanID() {
		rec.ParentSpanID = parent.SpanID().String()
	}
	if status := s.Status(); status.Code != codes.Unset {
		rec.StatusCode = status.Code.String()
		rec.StatusMessage = status.Description
	}
	if attrs := s.Attributes(); len(attrs) > 0 {
		rec.Attributes = make(map[string]string, len(attrs))
		for _, kv := range attrs {
			rec.Attributes[string(kv.Key)] = kv.Value.String()
		}
	}
	return rec
}

// Traces writes the most recent collector spans to the bundle.
type Traces struct {
	Processor *TraceProcessor
}

func (Traces) Name() string { return "traces" }

func (g Traces) Gather(_ context.Context, _ Options) ([]File, error) {
	spans := g.Processor.snapshot()
	if len(spans) == 0 {
		// Capture is off, or no spans have ended yet.
		return nil, nil
	}

	records := make([]spanRecord, 0, len(spans))
	for _, s := range spans {
		if s == nil {
			continue
		}
		records = append(records, toSpanRecord(s))
	}

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	return []File{{Path: "traces.json", Content: data}}, nil
}
