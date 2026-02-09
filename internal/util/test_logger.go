package util

import (
	"os"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/stretchr/testify/require"
)

// TestLogger generates a logger for a test.
func TestLogger(t testing.TB) log.Logger {
	t.Helper()

	l := log.NewSyncLogger(log.NewLogfmtLogger(os.Stderr))
	l = log.WithPrefix(l,
		"test", t.Name(),
		"ts", log.Valuer(testTimestamp),
	)

	return l
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

// testTimestamp is a log.Valuer that returns the timestamp
// without the date or timezone, reducing the noise in the test.
func testTimestamp() any {
	t := time.Now().UTC()
	return t.Format("15:04:05.000")
}
