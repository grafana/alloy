package slogadapter

import (
	"log/slog"

	"go.uber.org/zap"

	"github.com/grafana/alloy/internal/util/zapadapter"
)

// Zap returns a [zap.Logger] that writes to the provided slog.Logger.
func Zap(l *slog.Logger) *zap.Logger {
	// FIXME(kalleep): This is a hack, we use the go-kit logger adapter and pass
	// it to zapadapter. We should implement proper zap to slog adapter.
	return zapadapter.New(GoKit(l.Handler()))
}
