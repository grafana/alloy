package dbo11yreceiver

import (
	"context"
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// zapHandler is a slog.Handler writing to the collector's zap logger, since
// dbinstance and the collectors log through slog.
// POC-HACK: hand-rolled because go.uber.org/zap/exp/zapslog isn't in the
// module graph. Groups only prefix keys.
type zapHandler struct {
	logger *zap.Logger
	prefix string
}

func newZapHandler(l *zap.Logger) slog.Handler { return &zapHandler{logger: l} }

func zapLevel(l slog.Level) zapcore.Level {
	switch {
	case l >= slog.LevelError:
		return zapcore.ErrorLevel
	case l >= slog.LevelWarn:
		return zapcore.WarnLevel
	case l >= slog.LevelInfo:
		return zapcore.InfoLevel
	default:
		return zapcore.DebugLevel
	}
}

func (h *zapHandler) Enabled(_ context.Context, l slog.Level) bool {
	return h.logger.Core().Enabled(zapLevel(l))
}

func (h *zapHandler) Handle(_ context.Context, rec slog.Record) error {
	fields := make([]zap.Field, 0, rec.NumAttrs())
	rec.Attrs(func(a slog.Attr) bool {
		fields = append(fields, h.field(a))
		return true
	})
	if ce := h.logger.Check(zapLevel(rec.Level), rec.Message); ce != nil {
		ce.Write(fields...)
	}
	return nil
}

func (h *zapHandler) field(a slog.Attr) zap.Field {
	return zap.Any(h.prefix+a.Key, a.Value.Resolve().Any())
}

func (h *zapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	fields := make([]zap.Field, 0, len(attrs))
	for _, a := range attrs {
		fields = append(fields, h.field(a))
	}
	return &zapHandler{logger: h.logger.With(fields...), prefix: h.prefix}
}

func (h *zapHandler) WithGroup(name string) slog.Handler {
	return &zapHandler{logger: h.logger, prefix: h.prefix + name + "."}
}
