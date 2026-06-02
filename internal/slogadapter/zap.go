package slogadapter

import (
	"log/slog"

	"go.uber.org/zap"
)

// FIXME(kalleep): zap adapter for slog
func Zap(l *slog.Logger) *zap.Logger {
	return nil
}
