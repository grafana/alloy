//go:build linux

package alloycli

import (
	"log/slog"
	"os"

	"github.com/KimMachineGun/automemlimit/memlimit"

	"github.com/grafana/alloy/internal/runtime/logging"
)

func applyAutoMemLimit(l *logging.Logger) error {
	provider := withSystemFallback(memlimit.FromCgroup, os.Getenv("AUTOMEMLIMIT_EXPERIMENT"))
	_, err := memlimit.Set(memlimit.WithProvider(provider), memlimit.WithLogger(slog.New(l.Handler())))
	return err
}
