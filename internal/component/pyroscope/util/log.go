package util

import (
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

func TraceLog(l *slog.Logger, sp trace.Span) *slog.Logger {
	return l.With("trace_id", sp.SpanContext().TraceID().String())
}
