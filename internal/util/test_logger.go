package util

import (
	"os"
	"testing"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging"
	slimtestlog "github.com/grafana/alloy/internal/slim/testlog"
	"github.com/stretchr/testify/require"
)

// TestLogger generates a logger for a test.
// todo: inline
func TestLogger(t testing.TB) log.Logger {
	t.Helper()
	return slimtestlog.TestLogger(t)
}

// TestAlloyLogger generates an Alloy-compatible logger for a test.
func TestAlloyLogger(t require.TestingT) *logging.Logger {
	if t, ok := t.(*testing.T); ok {
		t.Helper()
	}

	l, err := logging.New(os.Stderr, logging.Options{
		Level:  logging.LevelDebug,
		Format: logging.FormatLogfmt,
	})
	require.NoError(t, err)
	return l
}
