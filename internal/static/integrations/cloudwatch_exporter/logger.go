package cloudwatch_exporter

import (
	"context"
	"log/slog"

	"github.com/grafana/alloy/internal/runtime/logging"
)

// slogHandler is wrapping our internal logging adapter with support for cloudwatch debug field.
// cloudwatch exporter will inspect check for debug logging level and pass that to aws sdk that perform
// it's own logging without going through the logger we pass.
type slogHandler struct {
	debug bool
	*logging.SlogGoKitHandler
}

func newSlogHandler(logger *logging.SlogGoKitHandler, debug bool) *slogHandler {
	return &slogHandler{
		debug:            debug,
		SlogGoKitHandler: logger,
	}
}

func (s *slogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	if level == slog.LevelDebug {
		return s.debug
	}
	return s.SlogGoKitHandler.Enabled(ctx, level)
}
