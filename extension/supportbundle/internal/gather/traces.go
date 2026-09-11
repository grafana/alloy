package gather

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/atomic"
)

var _ sdktrace.SpanProcessor = (*TraceProcessor)(nil)

// samplePeriod is the minimum time between spans accepted into one bucket. It
// spreads samples over time instead of keeping a burst, matching the zpages
// span processor.
const samplePeriod = time.Second

// latencyBounds are the upper bounds of the latency buckets, matching zpages. A
// span falls in the first bucket whose bound it is under; spans at or above the
// last bound fall in the final (overflow) bucket.
var latencyBounds = []time.Duration{
	10 * time.Microsecond,
	100 * time.Microsecond,
	time.Millisecond,
	10 * time.Millisecond,
	100 * time.Millisecond,
	time.Second,
	10 * time.Second,
	100 * time.Second,
}

// bucketLabels name each bucket, including the overflow bucket at the end.
var bucketLabels = []string{
	"<10us", "10us-100us", "100us-1ms", "1ms-10ms", "10ms-100ms", "100ms-1s", "1s-10s", "10s-100s", ">=100s",
}

func bucketFor(d time.Duration) int {
	for i, b := range latencyBounds {
		if d < b {
			return i
		}
	}
	return len(latencyBounds)
}

// bucket is a circular buffer of sampled spans, throttled to one span per
// samplePeriod so a burst does not crowd out earlier samples.
type bucket struct {
	nextTime time.Time
	buf      []sdktrace.ReadOnlySpan
	pos      int
	full     bool
}

func newBucket(capacity int) *bucket {
	return &bucket{buf: make([]sdktrace.ReadOnlySpan, capacity)}
}

func (b *bucket) add(s sdktrace.ReadOnlySpan) {
	if s.EndTime().Before(b.nextTime) || len(b.buf) == 0 {
		return
	}
	b.nextTime = s.EndTime().Add(samplePeriod)
	b.buf[b.pos] = s
	b.pos++
	if b.pos == len(b.buf) {
		b.pos = 0
		b.full = true
	}
}

func (b *bucket) spans() []sdktrace.ReadOnlySpan {
	if !b.full {
		return append([]sdktrace.ReadOnlySpan(nil), b.buf[:b.pos]...)
	}
	out := make([]sdktrace.ReadOnlySpan, 0, len(b.buf))
	out = append(out, b.buf[b.pos:]...)
	out = append(out, b.buf[:b.pos]...)
	return out
}

// sampleStore aggregates the spans of one name: exact counts plus a throttled
// sample per latency bucket and a sample of errors. Its own mutex guards it.
type sampleStore struct {
	mu         sync.Mutex
	count      int
	errorCount int
	latency    []*bucket
	errors     *bucket
}

func newSampleStore(capacity int) *sampleStore {
	ss := &sampleStore{
		latency: make([]*bucket, len(latencyBounds)+1),
		errors:  newBucket(capacity),
	}
	for i := range ss.latency {
		ss.latency[i] = newBucket(capacity)
	}
	return ss
}

func (ss *sampleStore) add(s sdktrace.ReadOnlySpan) {
	ss.mu.Lock()
	defer ss.mu.Unlock()
	ss.count++
	if s.Status().Code == codes.Error {
		ss.errorCount++
		ss.errors.add(s)
		return
	}
	latency := s.EndTime().Sub(s.StartTime())
	if latency < 0 {
		latency = 0
	}
	ss.latency[bucketFor(latency)].add(s)
}

// TraceProcessor is an SDK span processor that aggregates the collector's spans
// by name, keeping latency and error samples, mirroring the zpages extension.
// The extension registers it on the collector's tracer provider. Until enabled,
// OnEnd is a lock-free no-op, so tracing pays nothing.
//
// Like zpages, the set of tracked span names is unbounded, so memory grows with
// span-name cardinality. This is safe for the low-cardinality internal span
// names the collector emits.
type TraceProcessor struct {
	enabled  atomic.Bool
	capacity int
	stores   sync.Map // name (string) -> *sampleStore
}

// NewTraceProcessor returns a disabled processor.
func NewTraceProcessor() *TraceProcessor { return &TraceProcessor{} }

// Enable turns on capture, keeping up to capacity span samples per latency and
// error bucket for each name. A value of 0 leaves it disabled.
func (p *TraceProcessor) Enable(capacity int) {
	if capacity <= 0 {
		return
	}
	p.capacity = capacity
	p.enabled.Store(true)
}

func (p *TraceProcessor) OnStart(context.Context, sdktrace.ReadWriteSpan) {}

func (p *TraceProcessor) OnEnd(s sdktrace.ReadOnlySpan) {
	if !p.enabled.Load() {
		return
	}
	name := s.Name()
	v, ok := p.stores.Load(name)
	if !ok {
		v, _ = p.stores.LoadOrStore(name, newSampleStore(p.capacity))
	}
	v.(*sampleStore).add(s)
}

func (p *TraceProcessor) Shutdown(context.Context) error   { return nil }
func (p *TraceProcessor) ForceFlush(context.Context) error { return nil }

// snapshot builds the per-name reports. It copies the sampled spans under each
// store's lock, then converts them to records outside the lock.
func (p *TraceProcessor) snapshot() []traceReport {
	if !p.enabled.Load() {
		return nil
	}

	type raw struct {
		name       string
		count      int
		errorCount int
		latency    map[int][]sdktrace.ReadOnlySpan
		errors     []sdktrace.ReadOnlySpan
	}

	var raws []raw
	p.stores.Range(func(k, v any) bool {
		ss := v.(*sampleStore)
		r := raw{name: k.(string), latency: map[int][]sdktrace.ReadOnlySpan{}}
		ss.mu.Lock()
		r.count = ss.count
		r.errorCount = ss.errorCount
		for i, b := range ss.latency {
			if s := b.spans(); len(s) > 0 {
				r.latency[i] = s
			}
		}
		r.errors = ss.errors.spans()
		ss.mu.Unlock()
		raws = append(raws, r)
		return true
	})

	reports := make([]traceReport, 0, len(raws))
	for _, r := range raws {
		rep := traceReport{Name: r.name, Count: r.count, ErrorCount: r.errorCount}
		if len(r.latency) > 0 {
			rep.LatencySamples = make(map[string][]spanRecord, len(r.latency))
			for bucket, spans := range r.latency {
				rep.LatencySamples[bucketLabels[bucket]] = toSpanRecords(spans)
			}
		}
		rep.ErrorSamples = toSpanRecords(r.errors)
		reports = append(reports, rep)
	}
	sort.Slice(reports, func(i, j int) bool { return reports[i].Name < reports[j].Name })
	return reports
}

// traceReport is the per-name aggregation written to traces.json.
type traceReport struct {
	Name           string                  `json:"name"`
	Count          int                     `json:"count"`
	ErrorCount     int                     `json:"error_count"`
	LatencySamples map[string][]spanRecord `json:"latency_samples,omitempty"`
	ErrorSamples   []spanRecord            `json:"error_samples,omitempty"`
}

// spanRecord is a flat, greppable projection of a span.
type spanRecord struct {
	TraceID           string            `json:"trace_id"`
	SpanID            string            `json:"span_id"`
	ParentSpanID      string            `json:"parent_span_id,omitempty"`
	Kind              string            `json:"kind"`
	Scope             string            `json:"scope,omitempty"`
	StartTime         time.Time         `json:"start_time"`
	EndTime           time.Time         `json:"end_time"`
	DurationMS        float64           `json:"duration_ms"`
	StatusCode        string            `json:"status_code,omitempty"`
	StatusMessage     string            `json:"status_message,omitempty"`
	Attributes        map[string]string `json:"attributes,omitempty"`
	Events            []spanEvent       `json:"events,omitempty"`
	Links             []spanLink        `json:"links,omitempty"`
	DroppedAttributes int               `json:"dropped_attributes,omitempty"`
	DroppedEvents     int               `json:"dropped_events,omitempty"`
	DroppedLinks      int               `json:"dropped_links,omitempty"`
}

type spanEvent struct {
	Name       string            `json:"name"`
	Time       time.Time         `json:"time"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

type spanLink struct {
	TraceID    string            `json:"trace_id"`
	SpanID     string            `json:"span_id"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

func toSpanRecords(spans []sdktrace.ReadOnlySpan) []spanRecord {
	if len(spans) == 0 {
		return nil
	}
	out := make([]spanRecord, 0, len(spans))
	for _, s := range spans {
		if s != nil {
			out = append(out, toSpanRecord(s))
		}
	}
	return out
}

func attrMap(kvs []attribute.KeyValue) map[string]string {
	if len(kvs) == 0 {
		return nil
	}
	m := make(map[string]string, len(kvs))
	for _, kv := range kvs {
		m[string(kv.Key)] = kv.Value.String()
	}
	return m
}

func toSpanRecord(s sdktrace.ReadOnlySpan) spanRecord {
	sc := s.SpanContext()
	rec := spanRecord{
		TraceID:           sc.TraceID().String(),
		SpanID:            sc.SpanID().String(),
		Kind:              s.SpanKind().String(),
		Scope:             s.InstrumentationScope().Name,
		StartTime:         s.StartTime(),
		EndTime:           s.EndTime(),
		DurationMS:        float64(s.EndTime().Sub(s.StartTime()).Microseconds()) / 1000.0,
		Attributes:        attrMap(s.Attributes()),
		DroppedAttributes: s.DroppedAttributes(),
		DroppedEvents:     s.DroppedEvents(),
		DroppedLinks:      s.DroppedLinks(),
	}
	if parent := s.Parent(); parent.HasSpanID() {
		rec.ParentSpanID = parent.SpanID().String()
	}
	if status := s.Status(); status.Code != codes.Unset {
		rec.StatusCode = status.Code.String()
		rec.StatusMessage = status.Description
	}
	for _, e := range s.Events() {
		rec.Events = append(rec.Events, spanEvent{
			Name:       e.Name,
			Time:       e.Time,
			Attributes: attrMap(e.Attributes),
		})
	}
	for _, l := range s.Links() {
		rec.Links = append(rec.Links, spanLink{
			TraceID:    l.SpanContext.TraceID().String(),
			SpanID:     l.SpanContext.SpanID().String(),
			Attributes: attrMap(l.Attributes),
		})
	}
	return rec
}

// Traces writes the aggregated collector spans to the bundle.
type Traces struct {
	Processor *TraceProcessor
}

func (Traces) Name() string { return "traces" }

func (g Traces) Gather(_ context.Context, _ Options) ([]File, error) {
	reports := g.Processor.snapshot()
	if len(reports) == 0 {
		// Capture is off, or no spans have ended yet.
		return nil, nil
	}

	data, err := json.MarshalIndent(reports, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	return []File{{Path: "traces.json", Content: data}}, nil
}
