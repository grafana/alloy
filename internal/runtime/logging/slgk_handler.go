package logging

import (
	"context"
	"log/slog"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
)

var _ slog.Handler = (*SlogGoKitHandler)(nil)

// SlogGoKitHandler is an slog.Handler that wraps a go-kit logger.
// This is specific to Alloy's logging system, as we expect the go-kit
// logger to be configured with the correct level.
type SlogGoKitHandler struct {
	logger       log.Logger
	group        string
	preformatted []any
}

func NewSlogGoKitHandler(logger log.Logger) *SlogGoKitHandler {
	return &SlogGoKitHandler{
		logger: logger,
	}
}

func (h SlogGoKitHandler) Enabled(ctx context.Context, level slog.Level) bool {
	// Sometimes libraries will use this to check if certain logs should be emitted
	// and then write logs using other methods such as fmt package.
	// See https://github.com/percona/mongodb_exporter/blob/8290ba50eeb73d6380885d2546619afc878a6016/exporter/debug.go#L26-L42.
	// We try to assert the concrete type so we can delegate the check.
	// As a fallback we keep the old behavior.
	l, ok := h.logger.(*Logger)
	if !ok {
		// return always true, we expect the underlying logger to handle the level
		return true
	}

	return l.Enabled(ctx, level)
}

func (h SlogGoKitHandler) Handle(ctx context.Context, record slog.Record) error {
	var logger log.Logger
	switch record.Level {
	case slog.LevelInfo:
		logger = level.Info(h.logger)
	case slog.LevelWarn:
		logger = level.Warn(h.logger)
	case slog.LevelError:
		logger = level.Error(h.logger)
	default:
		logger = level.Debug(h.logger)
	}

	pairs := make([]any, 0, 2*record.NumAttrs())
	pairs = append(pairs, "msg", record.Message)
	pairs = append(pairs, h.preformatted...)

	record.Attrs(func(attr slog.Attr) bool {
		pairs = appendPair(pairs, h.group, attr)
		return true
	})

	return logger.Log(pairs...)
}

func (h SlogGoKitHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	pairs := make([]any, 0, 2*len(attrs))
	for _, attr := range attrs {
		pairs = appendPair(pairs, h.group, attr)
	}

	if h.preformatted != nil {
		pairs = append(h.preformatted, pairs...)
	}

	return &SlogGoKitHandler{
		logger:       h.logger,
		preformatted: pairs,
		group:        h.group,
	}
}

func (h SlogGoKitHandler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}

	group := name
	if h.group != "" {
		group = h.group + "." + group
	}

	return &SlogGoKitHandler{
		logger:       h.logger,
		preformatted: h.preformatted,
		group:        group,
	}
}

func appendPair(pairs []any, groupPrefix string, attr slog.Attr) []any {
	if attr.Equal(slog.Attr{}) {
		return pairs
	}

	switch attr.Value.Kind() {
	case slog.KindGroup:
		if attr.Key != "" {
			groupPrefix = groupPrefix + "." + attr.Key
		}
		for _, at := range attr.Value.Group() {
			pairs = appendPair(pairs, groupPrefix, at)
		}
	default:
		key := attr.Key
		if groupPrefix != "" {
			key = groupPrefix + "." + key
		}

		pairs = append(pairs, key, attr.Value)
	}

	return pairs
}
