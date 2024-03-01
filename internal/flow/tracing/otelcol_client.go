package tracing

import (
	"context"
	"fmt"
	"sync"

	"github.com/grafana/agent/internal/component/otelcol"
	"github.com/hashicorp/go-multierror"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
	"go.uber.org/atomic"
)

type client struct {
	started atomic.Bool

	mut     sync.RWMutex
	writeTo []otelcol.Consumer
}

var _ otlptrace.Client = (*client)(nil)

func (cli *client) UpdateWriteTo(consumers []otelcol.Consumer) {
	cli.mut.Lock()
	defer cli.mut.Unlock()
	cli.writeTo = consumers
}

func (cli *client) Start(ctx context.Context) error {
	if !cli.started.CompareAndSwap(false, true) {
		return fmt.Errorf("already started")
	}
	return nil
}

func (cli *client) Stop(ctx context.Context) error {
	if !cli.started.CompareAndSwap(true, false) {
		return fmt.Errorf("not running")
	}
	return nil
}

func (cli *client) UploadTraces(ctx context.Context, protoSpans []*tracepb.ResourceSpans) error {
	if !cli.started.Load() {
		// Client didn't start (may be a no-op client); ignore traces.
		return nil
	}

	payload := protoToCollector(protoSpans)

	cli.mut.RLock()
	defer cli.mut.RUnlock()

	var errs error

	for _, target := range cli.writeTo {
		send := payload

		if target.Capabilities().MutatesData {
			send = ptrace.NewTraces()
			payload.CopyTo(send)
		}

		if err := target.ConsumeTraces(ctx, send); err != nil {
			errs = multierror.Append(errs, err)
		}
	}

	return errs
}

// protoToCollector converts OpenTelemetry SDK traces to OpenTelemetry
// Collector traces.
func protoToCollector(in []*tracepb.ResourceSpans) ptrace.Traces {
	out := ptrace.NewTraces()

	for _, resourceIn := range in {
		resourceOut := out.ResourceSpans().AppendEmpty()
		resourceOut.SetSchemaUrl(resourceIn.GetSchemaUrl())
		resourceOut.Resource().SetDroppedAttributesCount(resourceIn.GetResource().GetDroppedAttributesCount())
		copyMap(resourceIn.GetResource().GetAttributes(), resourceOut.Resource().Attributes())

		resourceOut.ScopeSpans().EnsureCapacity(len(resourceIn.GetScopeSpans()))

		for _, scopeIn := range resourceIn.GetScopeSpans() {
			scopeOut := resourceOut.ScopeSpans().AppendEmpty()
			scopeOut.SetSchemaUrl(scopeIn.GetSchemaUrl())
			scopeOut.Scope().SetName(scopeIn.GetScope().GetName())
			scopeOut.Scope().SetVersion(scopeIn.GetScope().GetVersion())
			scopeOut.Scope().SetDroppedAttributesCount(scopeIn.GetScope().GetDroppedAttributesCount())
			copyMap(scopeIn.GetScope().GetAttributes(), scopeOut.Scope().Attributes())

			scopeOut.Spans().EnsureCapacity(len(scopeIn.GetSpans()))

			for _, spanIn := range scopeIn.GetSpans() {
				spanOut := scopeOut.Spans().AppendEmpty()
				spanOut.SetName(spanIn.GetName())
				spanOut.SetKind(convertKind(spanIn.Kind))
				spanOut.SetTraceID(convertTraceID(spanIn.GetTraceId()))
				spanOut.SetSpanID(convertSpanID(spanIn.GetSpanId()))
				spanOut.SetParentSpanID(convertSpanID(spanIn.GetParentSpanId()))
				spanOut.SetStartTimestamp(convertTimestamp(spanIn.GetStartTimeUnixNano()))
				spanOut.SetEndTimestamp(convertTimestamp(spanIn.GetEndTimeUnixNano()))

				spanOut.Status().SetCode(convertStatus(spanIn.GetStatus().GetCode()))
				spanOut.Status().SetMessage(spanIn.GetStatus().GetMessage())

				spanOut.TraceState().FromRaw(spanIn.GetTraceState())

				spanOut.SetDroppedAttributesCount(spanIn.GetDroppedAttributesCount())
				spanOut.SetDroppedEventsCount(spanIn.GetDroppedEventsCount())
				spanOut.SetDroppedAttributesCount(spanIn.GetDroppedAttributesCount())
				spanOut.SetDroppedLinksCount(spanIn.GetDroppedLinksCount())
				copyMap(spanIn.GetAttributes(), spanOut.Attributes())

				spanOut.Events().EnsureCapacity(len(spanIn.GetEvents()))
				for _, eventIn := range spanIn.GetEvents() {
					eventOut := spanOut.Events().AppendEmpty()
					eventOut.SetName(eventIn.GetName())
					eventOut.SetTimestamp(convertTimestamp(eventIn.GetTimeUnixNano()))

					eventOut.SetDroppedAttributesCount(eventIn.GetDroppedAttributesCount())
					copyMap(eventIn.GetAttributes(), eventOut.Attributes())
				}

				spanOut.Links().EnsureCapacity(len(spanIn.GetLinks()))
				for _, linkIn := range spanIn.GetLinks() {
					linkOut := spanOut.Links().AppendEmpty()
					linkOut.SetTraceID(convertTraceID(linkIn.GetTraceId()))
					linkOut.SetSpanID(convertSpanID(linkIn.GetSpanId()))

					linkOut.SetDroppedAttributesCount(linkIn.GetDroppedAttributesCount())
					copyMap(linkIn.GetAttributes(), linkOut.Attributes())
				}
			}
		}
	}

	return out
}

func copyMap(from []*commonpb.KeyValue, to pcommon.Map) {
	to.EnsureCapacity(len(from))

	for _, kvp := range from {
		out := to.PutEmpty(kvp.GetKey())
		copyValue(kvp.GetValue(), out)
	}
}

func copyValue(from *commonpb.AnyValue, to pcommon.Value) {
	switch val := from.GetValue().(type) {
	case *commonpb.AnyValue_StringValue:
		to.SetStr(val.StringValue)

	case *commonpb.AnyValue_BoolValue:
		to.SetBool(val.BoolValue)

	case *commonpb.AnyValue_IntValue:
		to.SetInt(val.IntValue)

	case *commonpb.AnyValue_DoubleValue:
		to.SetDouble(val.DoubleValue)

	case *commonpb.AnyValue_ArrayValue:
		slice := to.SetEmptySlice()
		slice.EnsureCapacity(len(val.ArrayValue.Values))

		for _, element := range val.ArrayValue.Values {
			sliceVal := slice.AppendEmpty()
			copyValue(element, sliceVal)
		}

	case *commonpb.AnyValue_KvlistValue:
		targetMap := to.SetEmptyMap()
		copyMap(val.KvlistValue.GetValues(), targetMap)

	case *commonpb.AnyValue_BytesValue:
		to.SetEmptyBytes().FromRaw(val.BytesValue)
	}
}

func convertTimestamp(inUnixNano uint64) pcommon.Timestamp {
	return pcommon.Timestamp(inUnixNano)
}

func convertKind(in tracepb.Span_SpanKind) ptrace.SpanKind {
	switch in {
	case tracepb.Span_SPAN_KIND_UNSPECIFIED:
		return ptrace.SpanKindUnspecified
	case tracepb.Span_SPAN_KIND_INTERNAL:
		return ptrace.SpanKindInternal
	case tracepb.Span_SPAN_KIND_SERVER:
		return ptrace.SpanKindServer
	case tracepb.Span_SPAN_KIND_CLIENT:
		return ptrace.SpanKindClient
	case tracepb.Span_SPAN_KIND_PRODUCER:
		return ptrace.SpanKindProducer
	case tracepb.Span_SPAN_KIND_CONSUMER:
		return ptrace.SpanKindConsumer
	}

	return ptrace.SpanKindUnspecified
}

func convertTraceID(in []byte) pcommon.TraceID {
	var out pcommon.TraceID
	copy(out[:], in)
	return out
}

func convertSpanID(in []byte) pcommon.SpanID {
	var out pcommon.SpanID
	copy(out[:], in)
	return out
}

func convertStatus(in tracepb.Status_StatusCode) ptrace.StatusCode {
	switch in {
	case tracepb.Status_STATUS_CODE_UNSET:
		return ptrace.StatusCodeUnset
	case tracepb.Status_STATUS_CODE_OK:
		return ptrace.StatusCodeOk
	case tracepb.Status_STATUS_CODE_ERROR:
		return ptrace.StatusCodeError
	}

	return ptrace.StatusCodeUnset
}
