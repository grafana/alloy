package tracing

import (
	"context"
	"path/filepath"
	"strings"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

var (
	componentIDAttributeKey  = "grafana_agent.component_id"
	controllerIDAttributeKey = "grafana_agent.controller_id"
)

// WrapTracer returns a new trace.TracerProvider which will inject the provided
// componentID as an attribute to each span.
func WrapTracer(inner trace.TracerProvider, componentID string) trace.TracerProvider {
	return &wrappedProvider{
		TracerProvider: inner,
		id:             componentID,
		spanName:       componentIDAttributeKey,
	}
}

// WrapTracerForLoader returns a new trace.TracerProvider which will inject the provided
// controller id as an attribute to each span.
func WrapTracerForLoader(inner trace.TracerProvider, componentID string) trace.TracerProvider {
	return &wrappedProvider{
		TracerProvider: inner,
		id:             componentID,
		spanName:       controllerIDAttributeKey,
	}
}

type wrappedProvider struct {
	trace.TracerProvider
	id       string
	spanName string
}

var _ trace.TracerProvider = (*wrappedProvider)(nil)

func (wp *wrappedProvider) Tracer(name string, options ...trace.TracerOption) trace.Tracer {
	// Inject the component name as instrumentation scope attribute.
	// This would not have component's exact ID, aligning with OTEL's definition
	if wp.id != "" {
		otelComponentName := strings.TrimSuffix(wp.id, filepath.Ext(wp.id))
		options = append(options, trace.WithInstrumentationAttributes(attribute.String(wp.spanName, otelComponentName)))
	}
	innerTracer := wp.TracerProvider.Tracer(name, options...)
	return &wrappedTracer{
		Tracer:   innerTracer,
		id:       wp.id,
		spanName: wp.spanName,
	}
}

type wrappedTracer struct {
	trace.Tracer
	id       string
	spanName string
}

var _ trace.Tracer = (*wrappedTracer)(nil)

func (tp *wrappedTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	ctx, span := tp.Tracer.Start(ctx, spanName, opts...)
	if tp.id != "" {
		span.SetAttributes(
			attribute.String(tp.spanName, tp.id),
		)
	}

	return ctx, span
}
