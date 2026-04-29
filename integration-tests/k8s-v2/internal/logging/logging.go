package logging

import (
	"io"
	"log/slog"
	"os"
)

var logger = slog.New(slog.NewTextHandler(io.Discard, nil))

func Configure(debug bool) {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})).With("suite", "k8s-v2")
}

func Logger() *slog.Logger {
	return logger
}
