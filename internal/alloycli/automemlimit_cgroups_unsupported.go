//go:build !linux

package alloycli

import (
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func applyAutoMemLimit(l *logging.Logger) error {
	// For non-linux builds without cgroups, memlimit will always report an error.
	// However, if the system experiment is requested, we can use the system memory limit provider.
	// This logic is similar to https://github.com/KimMachineGun/automemlimit/blob/main/memlimit/experiment.go
	if v, ok := os.LookupEnv("AUTOMEMLIMIT_EXPERIMENT"); ok {
		if slices.Contains(strings.Split(v, ","), "system") {
			_, err := memlimit.SetGoMemLimitWithOpts(memlimit.WithProvider(memlimit.FromSystem), memlimit.WithLogger(slog.New(l.Handler())))
			return err
		}
	}

	return nil
}
