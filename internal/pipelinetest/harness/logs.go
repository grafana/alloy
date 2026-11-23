package harness

import (
	"fmt"
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

// LogsContain returns an Assertion that passes when the log output of the
// runtime contains text. The text can span lines.
func LogsContain(text string) Assertion {
	return func(s snapshot) error {
		if strings.Contains(s.logs, text) {
			return nil
		}
		return AssertionError{
			Kind:    "logs.contains",
			Message: "log output does not contain " + text,
		}
	}
}

// maxRenderedLogLines is how many of the last log lines a failure shows.
const maxRenderedLogLines = 40

// renderLogs renders the end of the captured logs for a failure message.
func renderLogs(logs string) string {
	lines := strings.Split(strings.TrimSpace(logs), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return "logs: none"
	}
	prefix := "logs:\n"
	if len(lines) > maxRenderedLogLines {
		lines = lines[len(lines)-maxRenderedLogLines:]
		prefix = fmt.Sprintf("logs (last %d lines):\n", maxRenderedLogLines)
	}
	return prefix + strings.Join(lines, "\n")
}
