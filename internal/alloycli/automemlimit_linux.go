//go:build linux
// +build linux

package alloycli

import (
	"log/slog"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/grafana/alloy/internal/service/logging"
)

func applyAutoMemLimit(l *logging.Logger) {
	memlimit.SetGoMemLimitWithOpts(memlimit.WithLogger(slog.New(l.Handler())))
}
