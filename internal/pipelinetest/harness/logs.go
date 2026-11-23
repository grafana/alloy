package harness

import (
	"strings"
	"sync"
)

// logBuffer collects the log output of the runtime. It is safe for concurrent
// use, because components write logs from many goroutines.
type logBuffer struct {
	mut sync.Mutex
	buf strings.Builder
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mut.Lock()
	defer b.mut.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mut.Lock()
	defer b.mut.Unlock()
	return b.buf.String()
}

// LogsContain returns an Assertion that passes when the runtime logged text.
func LogsContain(text string) Assertion {
	return func(s snapshot) error {
		if strings.Contains(s.logs, text) {
			return nil
		}
		return AssertionError{
			Kind:    "logs.contains",
			Message: "no log line contains " + text,
		}
	}
}
