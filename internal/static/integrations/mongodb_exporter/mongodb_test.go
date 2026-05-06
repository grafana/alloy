package mongodb_exporter

import (
	"context"
	"log/slog"
	"testing"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/static/config"
)

func TestConfig_SecretMongoDB(t *testing.T) {
	stringCfg := `
prometheus:
  wal_directory: /tmp/agent
integrations:
  mongodb_exporter:
    enabled: true
    mongodb_uri: secret_password_in_uri
`
	config.CheckSecret(t, stringCfg, "secret_password_in_uri")
}

func TestLevelAwareLogger_Enabled(t *testing.T) {
	nop := log.NewNopLogger()

	tests := []struct {
		name     string
		minLevel slog.Level
		check    slog.Level
		want     bool
	}{
		{"info blocks debug", slog.LevelInfo, slog.LevelDebug, false},
		{"info allows info", slog.LevelInfo, slog.LevelInfo, true},
		{"info allows warn", slog.LevelInfo, slog.LevelWarn, true},
		{"info allows error", slog.LevelInfo, slog.LevelError, true},
		{"debug allows debug", slog.LevelDebug, slog.LevelDebug, true},
		{"error blocks warn", slog.LevelError, slog.LevelWarn, false},
		{"error blocks info", slog.LevelError, slog.LevelInfo, false},
		{"error allows error", slog.LevelError, slog.LevelError, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			l := &levelAwareLogger{Logger: nop, minLevel: tt.minLevel}
			got := l.Enabled(context.Background(), tt.check)
			require.Equal(t, tt.want, got)
		})
	}
}
