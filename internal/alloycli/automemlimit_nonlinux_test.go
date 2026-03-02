//go:build !linux

package alloycli

import (
	"bytes"
	"log/slog"
	"testing"

	"github.com/KimMachineGun/automemlimit/memlimit"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/stretchr/testify/require"
)

func TestNoMemlimitErrorLogs(t *testing.T) {
	buffer := bytes.NewBuffer(nil)

	l, err := logging.New(buffer, logging.DefaultOptions)
	require.NoError(t, err)

	applyAutoMemLimit(l)

	require.Equal(t, "", buffer.String())

	// Linux behavior, to confirm error is logged
	memlimit.SetGoMemLimitWithOpts(memlimit.WithLogger(slog.New(l.Handler())))

	require.Contains(t, buffer.String(), "cgroups is not supported on this system")
}
