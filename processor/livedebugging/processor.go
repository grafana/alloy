// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package livedebugging

import (
	"context"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
)

const overflowLabel = "<overflow>"

type attributionKey struct {
	namespace   string
	serviceName string
}

type spanNameKey struct {
	serviceName string
	spanName    string
}

// tracesProcessor tracks, per batch, which (namespace, service_name) sends
// the largest traces and which span names are most frequent, then forwards
// the batch to next unchanged. It never clones or mutates the data it
// observes.
type tracesProcessor struct {
	config    *Config
	next      consumer.Traces
	telemetry component.TelemetrySettings
	stream    *streamServer

	mu                sync.Mutex
	largestTraceBytes map[attributionKey]int64
	trackedSpanNames  map[spanNameKey]struct{}

	spanNameCounter metric.Int64Counter
	marshaler       ptrace.ProtoMarshaler
}

func newTracesProcessor(cfg *Config, set processor.Settings, next consumer.Traces) (processor.Traces, error) {
	meter := set.TelemetrySettings.MeterProvider.Meter("github.com/grafana/alloy/processor/livedebugging")

	p := &tracesProcessor{
		config:            cfg,
		next:              next,
		telemetry:         set.TelemetrySettings,
		largestTraceBytes: make(map[attributionKey]int64),
		trackedSpanNames:  make(map[spanNameKey]struct{}),
	}
	if cfg.Stream.Enabled {
		p.stream = newStreamServer(cfg.Stream, set.TelemetrySettings.Logger)
	}

	spanNameCounter, err := meter.Int64Counter(
		"otelcol_processor_live_debugging_span_name_total",
		metric.WithDescription("Number of spans observed for a given service_name/span_name combination."),
		metric.WithUnit("{span}"),
	)
	if err != nil {
		return nil, fmt.Errorf("create span name counter: %w", err)
	}
	p.spanNameCounter = spanNameCounter

	_, err = meter.Int64ObservableGauge(
		"otelcol_processor_live_debugging_largest_trace_size_bytes",
		metric.WithDescription("The largest observed trace size, in bytes, for a given namespace/service_name combination."),
		metric.WithUnit("By"),
		metric.WithInt64Callback(p.observeLargestTraceBytes),
	)
	if err != nil {
		return nil, fmt.Errorf("create largest trace size gauge: %w", err)
	}

	return p, nil
}

var (
	_ processor.Traces    = (*tracesProcessor)(nil)
	_ component.Component = (*tracesProcessor)(nil)
	_ consumer.Traces     = (*tracesProcessor)(nil)
)

func (p *tracesProcessor) Start(ctx context.Context, host component.Host) error {
	if p.stream == nil {
		return nil
	}
	return p.stream.start(ctx, host, p.telemetry)
}

func (p *tracesProcessor) Shutdown(ctx context.Context) error {
	if p.stream == nil {
		return nil
	}
	return p.stream.shutdown(ctx)
}

// actualAddr returns the stream server's actual listening address, useful
// in tests that configure a dynamic port ("localhost:0"). Empty if the
// stream isn't enabled or hasn't started yet.
func (p *tracesProcessor) actualAddr() string {
	if p.stream == nil || p.stream.listener == nil {
		return ""
	}
	return p.stream.listener.Addr().String()
}

func (p *tracesProcessor) Capabilities() consumer.Capabilities {
	// This processor never mutates the data it observes.
	return consumer.Capabilities{MutatesData: false}
}

func (p *tracesProcessor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	p.observe(td)
	if p.stream != nil {
		p.stream.publish(td)
	}
	return p.next.ConsumeTraces(ctx, td)
}

// observe records span-name frequency and per-trace size attribution for
// the batch. It never mutates td.
func (p *tracesProcessor) observe(td ptrace.Traces) {
	type traceTotal struct {
		bytes int64
		key   attributionKey
	}
	totals := make(map[pcommon.TraceID]*traceTotal)

	p.mu.Lock()
	defer p.mu.Unlock()

	resourceSpans := td.ResourceSpans()
	for i := 0; i < resourceSpans.Len(); i++ {
		rs := resourceSpans.At(i)
		key := attributionKey{
			namespace:   attrStringOrEmpty(rs.Resource().Attributes(), p.config.NamespaceAttributeKey),
			serviceName: attrStringOrEmpty(rs.Resource().Attributes(), p.config.ServiceNameAttributeKey),
		}

		scopeSpans := rs.ScopeSpans()
		for j := 0; j < scopeSpans.Len(); j++ {
			ss := scopeSpans.At(j)
			spans := ss.Spans()
			for k := 0; k < spans.Len(); k++ {
				span := spans.At(k)

				p.recordSpanName(key.serviceName, span.Name())

				size := p.spanByteSize(rs, ss, span)
				total, ok := totals[span.TraceID()]
				if !ok {
					total = &traceTotal{key: key}
					totals[span.TraceID()] = total
				}
				total.bytes += int64(size)
			}
		}
	}

	for _, total := range totals {
		p.recordTraceSize(total.key, total.bytes)
	}
}

// spanByteSize returns the exact OTLP proto size of span in isolation
// (its own resource and scope, no other spans) - the same definition
// tailsamplingprocessor's own TraceData.SizeBytes uses.
func (p *tracesProcessor) spanByteSize(rs ptrace.ResourceSpans, ss ptrace.ScopeSpans, span ptrace.Span) int {
	tmp := ptrace.NewTraces()
	rsCopy := tmp.ResourceSpans().AppendEmpty()
	rs.Resource().CopyTo(rsCopy.Resource())
	ssCopy := rsCopy.ScopeSpans().AppendEmpty()
	ss.Scope().CopyTo(ssCopy.Scope())
	span.CopyTo(ssCopy.Spans().AppendEmpty())

	data, err := p.marshaler.MarshalTraces(tmp)
	if err != nil {
		return 0
	}
	return len(data)
}

// recordSpanName increments the span-name frequency counter, redirecting
// to the overflow bucket once max_span_name_series distinct keys have
// been tracked. Caller must hold p.mu.
func (p *tracesProcessor) recordSpanName(serviceName, spanName string) {
	key := spanNameKey{serviceName: serviceName, spanName: spanName}
	if _, tracked := p.trackedSpanNames[key]; !tracked {
		if len(p.trackedSpanNames) >= p.config.MaxSpanNameSeries {
			key = spanNameKey{serviceName: overflowLabel, spanName: overflowLabel}
		} else {
			p.trackedSpanNames[key] = struct{}{}
		}
	}
	p.spanNameCounter.Add(context.Background(), 1, metric.WithAttributes(
		attrServiceName(key.serviceName),
		attrSpanName(key.spanName),
	))
}

// recordTraceSize updates the high-water mark for key if bytes exceeds
// the currently-recorded value, redirecting to the overflow bucket once
// max_attribution_series distinct keys have been tracked. Caller must
// hold p.mu.
func (p *tracesProcessor) recordTraceSize(key attributionKey, bytes int64) {
	if _, tracked := p.largestTraceBytes[key]; !tracked {
		if len(p.largestTraceBytes) >= p.config.MaxAttributionSeries {
			key = attributionKey{namespace: overflowLabel, serviceName: overflowLabel}
		}
	}
	if bytes > p.largestTraceBytes[key] {
		p.largestTraceBytes[key] = bytes
	}
}

func (p *tracesProcessor) observeLargestTraceBytes(_ context.Context, o metric.Int64Observer) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for key, bytes := range p.largestTraceBytes {
		o.Observe(bytes, metric.WithAttributes(
			attrNamespace(key.namespace),
			attrServiceName(key.serviceName),
		))
	}
	return nil
}

// attrStringOrEmpty reads key from attrs, returning "" if the attribute is
// absent. pcommon.Map.Get returns a zero-value Value when the key is
// missing, and calling AsString() on that zero value panics, so the ok
// bool must always be checked.
func attrStringOrEmpty(attrs pcommon.Map, key string) string {
	v, ok := attrs.Get(key)
	if !ok {
		return ""
	}
	return v.AsString()
}

func attrNamespace(v string) attribute.KeyValue { return attribute.String("namespace", v) }

func attrServiceName(v string) attribute.KeyValue { return attribute.String("service_name", v) }

func attrSpanName(v string) attribute.KeyValue { return attribute.String("span_name", v) }
