package testlog

import (
	"log/slog"
	"os"
	"testing"
)

// TestLogger is a copy of util.TestLogger but does not bring hundreds of transitive dependencies
// A little copying is better than a little dependency.
// https://www.youtube.com/watch?v=PAAkCSZUG1c&t=568s
// The Previous attempt to fix this globally stalled: https://github.com/grafana/alloy/pull/4369
// So for now it is in the pyroscope subpackage
func TestLogger(t testing.TB) *slog.Logger {
	t.Helper()
	l := slog.New(slog.NewTextHandler(os.Stderr, nil))
	l = l.With("test", t.Name())
	return l
}
