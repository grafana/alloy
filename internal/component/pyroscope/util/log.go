package util

import (
	"github.com/go-kit/log"
	"go.opentelemetry.io/otel/trace"
)

func TraceLog(l log.Logger, sp trace.Span) log.Logger {
	return log.With(l,
		"trace_id", sp.SpanContext().TraceID().String(),
	)
}
