// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

package livedebugging

import (
	"context"
	"encoding/binary"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.uber.org/zap"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenttest"
	"go.opentelemetry.io/collector/consumer/consumertest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/collector/processor"
)

func newTestProcessor(t *testing.T, cfg *Config, next *consumertest.TracesSink) (*tracesProcessor, *sdkmetric.ManualReader) {
	t.Helper()
	if cfg == nil {
		cfg = createDefaultConfig().(*Config)
	}
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))

	set := processor.Settings{
		ID:                component.MustNewID("live_debugging"),
		TelemetrySettings: component.TelemetrySettings{MeterProvider: mp},
	}
	p, err := newTracesProcessor(cfg, set, next)
	require.NoError(t, err)
	return p.(*tracesProcessor), reader
}

func makeTraces(traceIDByte byte, serviceName, namespace, spanName string, extraSpans int) ptrace.Traces {
	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty()
	rs.Resource().Attributes().PutStr("service.name", serviceName)
	rs.Resource().Attributes().PutStr("k8s.namespace.name", namespace)
	ss := rs.ScopeSpans().AppendEmpty()

	traceID := pcommon.TraceID{}
	for i := range traceID {
		traceID[i] = traceIDByte
	}

	for i := 0; i <= extraSpans; i++ {
		span := ss.Spans().AppendEmpty()
		span.SetTraceID(traceID)
		span.SetSpanID(pcommon.SpanID{byte(i) + 1})
		span.SetName(spanName)
	}
	return td
}

func TestConsumeTraces_ForwardsUnchanged(t *testing.T) {
	next := new(consumertest.TracesSink)
	p, _ := newTestProcessor(t, nil, next)

	td := makeTraces(1, "checkout", "payments", "POST /charge", 0)
	require.NoError(t, p.ConsumeTraces(context.Background(), td))

	require.Len(t, next.AllTraces(), 1)
	assert.Equal(t, td, next.AllTraces()[0])
}

func TestConsumeTraces_PropagatesNextError(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	mp := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	set := processor.Settings{
		ID:                component.MustNewID("live_debugging"),
		TelemetrySettings: component.TelemetrySettings{MeterProvider: mp},
	}
	p, err := newTracesProcessor(createDefaultConfig().(*Config), set, consumertest.NewErr(assert.AnError))
	require.NoError(t, err)

	consumeErr := p.ConsumeTraces(context.Background(), makeTraces(1, "svc", "ns", "op", 0))
	assert.ErrorIs(t, consumeErr, assert.AnError)
}

func TestLargestTraceSize_OnlyUpdatesOnLargerObservation(t *testing.T) {
	next := new(consumertest.TracesSink)
	p, reader := newTestProcessor(t, nil, next)

	small := makeTraces(1, "checkout", "payments", "op", 0)  // 1 span
	big := makeTraces(2, "checkout", "payments", "op", 50)   // 51 spans, same attribution key

	require.NoError(t, p.ConsumeTraces(context.Background(), big))
	afterBig := gaugeValue(t, reader, "payments", "checkout")

	require.NoError(t, p.ConsumeTraces(context.Background(), small))
	afterSmall := gaugeValue(t, reader, "payments", "checkout")

	assert.Equal(t, afterBig, afterSmall, "a smaller trace must not lower the recorded high-water mark")
	assert.Positive(t, afterBig)
}

func TestLargestTraceSize_UpdatesOnLargerObservation(t *testing.T) {
	next := new(consumertest.TracesSink)
	p, reader := newTestProcessor(t, nil, next)

	require.NoError(t, p.ConsumeTraces(context.Background(), makeTraces(1, "checkout", "payments", "op", 0)))
	first := gaugeValue(t, reader, "payments", "checkout")

	require.NoError(t, p.ConsumeTraces(context.Background(), makeTraces(2, "checkout", "payments", "op", 50)))
	second := gaugeValue(t, reader, "payments", "checkout")

	assert.Greater(t, second, first)
}

func TestLargestTraceSize_OverflowBucket(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaxAttributionSeries = 1
	next := new(consumertest.TracesSink)
	p, reader := newTestProcessor(t, cfg, next)

	require.NoError(t, p.ConsumeTraces(context.Background(), makeTraces(1, "svc-a", "ns", "op", 0)))
	require.NoError(t, p.ConsumeTraces(context.Background(), makeTraces(2, "svc-b", "ns", "op", 0)))

	assert.Equal(t, int64(0), gaugeValue(t, reader, "ns", "svc-b"), "second distinct key beyond the cap must not get its own series")
	assert.Positive(t, gaugeValue(t, reader, overflowLabel, overflowLabel))
}

func TestSpanNameFrequency_OverflowBucket(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.MaxSpanNameSeries = 1
	next := new(consumertest.TracesSink)
	p, reader := newTestProcessor(t, cfg, next)

	require.NoError(t, p.ConsumeTraces(context.Background(), makeTraces(1, "svc", "ns", "op-a", 0)))
	require.NoError(t, p.ConsumeTraces(context.Background(), makeTraces(2, "svc", "ns", "op-b", 0)))

	assert.Equal(t, int64(1), counterValue(t, reader, "svc", "op-a"))
	assert.Equal(t, int64(0), counterValue(t, reader, "svc", "op-b"))
	assert.Equal(t, int64(1), counterValue(t, reader, overflowLabel, overflowLabel))
}

func TestConsumeTraces_MissingAttributesDoNotPanic(t *testing.T) {
	next := new(consumertest.TracesSink)
	p, reader := newTestProcessor(t, nil, next)

	td := ptrace.NewTraces()
	rs := td.ResourceSpans().AppendEmpty() // no resource attributes at all
	ss := rs.ScopeSpans().AppendEmpty()
	span := ss.Spans().AppendEmpty()
	span.SetName("op")
	span.SetTraceID(pcommon.TraceID{1})
	span.SetSpanID(pcommon.SpanID{1})

	require.NoError(t, p.ConsumeTraces(context.Background(), td))
	require.Len(t, next.AllTraces(), 1)
	assert.Positive(t, gaugeValue(t, reader, "", ""))
	assert.Equal(t, int64(1), counterValue(t, reader, "", "op"))
}

func TestStream_ForwardsObservedTracesAndKeepsPassThrough(t *testing.T) {
	cfg := createDefaultConfig().(*Config)
	cfg.Stream.Enabled = true
	cfg.Stream.NetAddr.Endpoint = "localhost:0"

	next := new(consumertest.TracesSink)
	set := processor.Settings{
		ID:                component.MustNewID("live_debugging"),
		TelemetrySettings: component.TelemetrySettings{MeterProvider: sdkmetric.NewMeterProvider(), Logger: zap.NewNop()},
	}
	p, err := newTracesProcessor(cfg, set, next)
	require.NoError(t, err)

	host := componenttest.NewNopHost()
	require.NoError(t, p.Start(context.Background(), host))
	defer func() { require.NoError(t, p.Shutdown(context.Background())) }()

	tp := p.(*tracesProcessor)
	require.NotEmpty(t, tp.actualAddr())

	streamURL := "http://" + tp.actualAddr() + pathStream + "?duration=2s"
	req, err := http.NewRequest(http.MethodGet, streamURL, nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	go func() {
		time.Sleep(50 * time.Millisecond)
		td := makeTraces(9, "svc", "ns", "op", 0)
		require.NoError(t, p.ConsumeTraces(context.Background(), td))
	}()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	frames := decodeStreamFrames(t, body)
	require.Len(t, frames, 1)
	assert.Equal(t, 1, frames[0].SpanCount())
	require.Len(t, next.AllTraces(), 1, "the stream must not stop data from reaching next")
}

func decodeStreamFrames(t *testing.T, data []byte) []ptrace.Traces {
	t.Helper()
	var out []ptrace.Traces
	unmarshaler := ptrace.ProtoUnmarshaler{}
	for len(data) > 0 {
		require.GreaterOrEqual(t, len(data), 4)
		n := binary.BigEndian.Uint32(data[:4])
		data = data[4:]
		require.GreaterOrEqual(t, len(data), int(n))
		traces, err := unmarshaler.UnmarshalTraces(data[:n])
		require.NoError(t, err)
		out = append(out, traces)
		data = data[n:]
	}
	return out
}

func TestCapabilities_NeverMutates(t *testing.T) {
	p, _ := newTestProcessor(t, nil, new(consumertest.TracesSink))
	assert.False(t, p.Capabilities().MutatesData)
}

// -- metricdata helpers --

func collect(t *testing.T, reader *sdkmetric.ManualReader) *metricdata.ResourceMetrics {
	t.Helper()
	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))
	return &rm
}

func gaugeValue(t *testing.T, reader *sdkmetric.ManualReader, namespace, serviceName string) int64 {
	t.Helper()
	rm := collect(t, reader)
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "otelcol_processor_live_debugging_largest_trace_size_bytes" {
				continue
			}
			gauge, ok := m.Data.(metricdata.Gauge[int64])
			if !ok {
				continue
			}
			for _, dp := range gauge.DataPoints {
				if attrEquals(dp.Attributes, "namespace", namespace) && attrEquals(dp.Attributes, "service_name", serviceName) {
					return dp.Value
				}
			}
		}
	}
	return 0
}

func counterValue(t *testing.T, reader *sdkmetric.ManualReader, serviceName, spanName string) int64 {
	t.Helper()
	rm := collect(t, reader)
	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "otelcol_processor_live_debugging_span_name_total" {
				continue
			}
			sum, ok := m.Data.(metricdata.Sum[int64])
			if !ok {
				continue
			}
			for _, dp := range sum.DataPoints {
				if attrEquals(dp.Attributes, "service_name", serviceName) && attrEquals(dp.Attributes, "span_name", spanName) {
					return dp.Value
				}
			}
		}
	}
	return 0
}

func attrEquals(set attribute.Set, key, value string) bool {
	v, ok := set.Value(attribute.Key(key))
	return ok && v.AsString() == value
}
