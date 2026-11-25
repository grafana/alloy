package harness

import (
	"context"
	"encoding/hex"

	otelconsumer "go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/plog"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"

	"github.com/grafana/alloy/internal/component/otelcol"
)

// Span is one OTLP span that the sink captured. The sink flattens it, so
// assertions and reports do not walk the resource and scope hierarchy. The
// sink writes attribute values as strings.
type Span struct {
	ResourceAttributes map[string]string
	Scope              string
	Name               string
	Kind               string
	TraceID            string
	SpanID             string
	ParentSpanID       string
	Attributes         map[string]string
	StatusCode         string
}

// otelSink is the otelcol.Consumer the sink exports. It captures traces. OTLP
// metrics and logs are accepted and counted, but cannot be asserted on yet.
type otelSink struct {
	s *Sink
}

var (
	_ otelcol.Consumer          = (*otelSink)(nil)
	_ otelcol.ComponentMetadata = (*otelSink)(nil)
)

func (o *otelSink) ComponentID() string { return o.s.opts.ID }

func (o *otelSink) Capabilities() otelconsumer.Capabilities {
	return otelconsumer.Capabilities{MutatesData: false}
}

func (o *otelSink) ConsumeTraces(_ context.Context, td ptrace.Traces) error {
	spans := flattenSpans(td)

	o.s.mux.Lock()
	defer o.s.mux.Unlock()
	o.s.spans = append(o.s.spans, spans...)
	return nil
}

func (o *otelSink) ConsumeMetrics(_ context.Context, md pmetric.Metrics) error {
	o.s.mux.Lock()
	defer o.s.mux.Unlock()
	o.s.otlpDataPoints += md.DataPointCount()
	return nil
}

func (o *otelSink) ConsumeLogs(_ context.Context, ld plog.Logs) error {
	o.s.mux.Lock()
	defer o.s.mux.Unlock()
	o.s.otlpLogRecords += ld.LogRecordCount()
	return nil
}

func flattenSpans(td ptrace.Traces) []Span {
	var out []Span
	for _, rs := range td.ResourceSpans().All() {
		resourceAttrs := attributesToMap(rs.Resource().Attributes())
		for _, ss := range rs.ScopeSpans().All() {
			scope := ss.Scope().Name()
			for _, span := range ss.Spans().All() {
				out = append(out, Span{
					ResourceAttributes: resourceAttrs,
					Scope:              scope,
					Name:               span.Name(),
					Kind:               span.Kind().String(),
					TraceID:            traceIDString(span.TraceID()),
					SpanID:             spanIDString(span.SpanID()),
					ParentSpanID:       spanIDString(span.ParentSpanID()),
					Attributes:         attributesToMap(span.Attributes()),
					StatusCode:         span.Status().Code().String(),
				})
			}
		}
	}
	return out
}

func attributesToMap(attrs pcommon.Map) map[string]string {
	if attrs.Len() == 0 {
		return nil
	}
	out := make(map[string]string, attrs.Len())
	for k, v := range attrs.All() {
		out[k] = v.AsString()
	}
	return out
}

func traceIDString(id pcommon.TraceID) string {
	if id.IsEmpty() {
		return ""
	}
	return hex.EncodeToString(id[:])
}

func spanIDString(id pcommon.SpanID) string {
	if id.IsEmpty() {
		return ""
	}
	return hex.EncodeToString(id[:])
}
