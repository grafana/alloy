package oci

import (
	"context"
	"log/slog"

	"github.com/grafana/alloy/internal/runtime/logging"
)

// slogHandler wraps the internal logging adapter with support for debug field toggling.
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
