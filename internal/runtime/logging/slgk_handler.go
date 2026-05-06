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
	// Some libraries check Enabled() to decide whether to emit output via
	// non-slog paths (e.g. fmt.Fprintln to stderr). See:
	// https://github.com/percona/mongodb_exporter/blob/8290ba50eeb73d6380885d2546619afc878a6016/exporter/debug.go#L26-L42
	if ea, ok := h.logger.(EnabledAware); ok {
		return ea.Enabled(ctx, level)
	}
	return true
}

// levelAwareLogger wraps a go-kit logger and implements EnabledAware by
// delegating to a separate EnabledAware instance. This preserves level
// awareness through go-kit's log.With() wrapping.
type levelAwareLogger struct {
	log.Logger
	ea EnabledAware
}

func (l *levelAwareLogger) Enabled(ctx context.Context, level slog.Level) bool {
	return l.ea.Enabled(ctx, level)
}

// NewLevelAwareLogger returns a go-kit logger that also implements EnabledAware,
// delegating level checks to ea. Use this when wrapping a logger with log.With()
// to preserve the ability to check log levels via the EnabledAware interface.
func NewLevelAwareLogger(logger log.Logger, ea EnabledAware) log.Logger {
	return &levelAwareLogger{Logger: logger, ea: ea}
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
