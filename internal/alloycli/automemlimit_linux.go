//go:build linux

package alloycli

import (
	"log/slog"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func applyAutoMemLimit(l *logging.Logger) error {
	_, err := memlimit.SetGoMemLimitWithOpts(memlimit.WithLogger(slog.New(l.Handler())))
	return err
}
