package util

import (
	"os"
	"testing"

	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/stretchr/testify/require"
)

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
