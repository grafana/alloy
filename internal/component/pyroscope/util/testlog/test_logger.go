package testlog

import (
	"os"
	"testing"

	"github.com/go-kit/log"
)

func TestLogger(t testing.TB) log.Logger {
	t.Helper()

	l := log.NewSyncLogger(log.NewLogfmtLogger(os.Stderr))
	l = log.WithPrefix(l,
		"test", t.Name(),
		"ts", log.DefaultTimestampUTC,
	)

	return l
}
