package testlog

import (
	"os"
	"testing"

	"github.com/go-kit/log"
)

// TestLogger is a copy of util.TestLogger but does not bring hundreds of transitive dependencies
// A little copying is better than a little dependency.
// https://www.youtube.com/watch?v=PAAkCSZUG1c&t=568s
// The Previous attempt to fix this globally stalled: https://github.com/grafana/alloy/pull/4369
// So for now it is in the pyroscope subpackage
func TestLogger(t testing.TB) log.Logger {
	t.Helper()
	l := log.NewSyncLogger(log.NewLogfmtLogger(os.Stderr))
	l = log.WithPrefix(l,
		"test", t.Name(),
		"ts", log.DefaultTimestampUTC,
	)
	return l
}
