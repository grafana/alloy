//go:build linux

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
	opts := []memlimit.Option{memlimit.WithLogger(slog.New(l.Handler()))}
	// automemlimit v1 no longer reads AUTOMEMLIMIT_EXPERIMENT, so keep the
	// documented "system" fallback to the system memory limit here.
	if v, ok := os.LookupEnv("AUTOMEMLIMIT_EXPERIMENT"); ok && slices.Contains(strings.Split(v, ","), "system") {
		opts = append(opts, memlimit.WithProvider(memlimit.ApplyFallback(memlimit.FromCgroup, memlimit.FromSystem)))
	}
	_, err := memlimit.Set(opts...)
	return err
}
