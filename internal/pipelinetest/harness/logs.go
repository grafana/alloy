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

// logTail is the end of the captured logs that a failure message shows.
type logTail struct {
	lines     []string
	truncated bool
}

// tailLogs keeps the last maxRenderedLogLines lines of logs.
func tailLogs(logs string) logTail {
	logs = strings.TrimSpace(logs)
	if logs == "" {
		return logTail{}
	}
	lines := strings.Split(logs, "\n")
	if len(lines) <= maxRenderedLogLines {
		return logTail{lines: lines}
	}
	return logTail{lines: lines[len(lines)-maxRenderedLogLines:], truncated: true}
}

func (t logTail) String() string {
	out := strings.Join(t.lines, "\n")
	if t.truncated {
		return "...\n" + out
	}
	return out
}
