package harness

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"sync"
)

// maxProblemLogs caps how many log lines a failed test shows.
const maxProblemLogs = 10

var (
	// Alloy writes ts first and level second.
	problemLevel = regexp.MustCompile(`^(ts=\S+\s+)?level=(warn|error)(\s|$)`)
	logTimestamp = regexp.MustCompile(`^ts=\S+\s+`)
)

// problemLog keeps the first distinct warning and error lines that Alloy logs.
// These often name the cause of a failure, such as a file that doesn't exist.
// It drops the timestamp, so a component that logs the same error again and
// again takes one line.
type problemLog struct {
	mut     sync.Mutex
	max     int
	partial []byte
	kept    []string
	dropped int
}

func newProblemLog(maxLines int) *problemLog {
	return &problemLog{max: maxLines}
}

// Write takes logfmt output. A line can arrive in more than one write.
func (p *problemLog) Write(b []byte) (int, error) {
	p.mut.Lock()
	defer p.mut.Unlock()

	p.partial = append(p.partial, b...)
	for {
		i := bytes.IndexByte(p.partial, '\n')
		if i < 0 {
			break
		}
		p.add(string(p.partial[:i]))
		p.partial = p.partial[i+1:]
	}
	return len(b), nil
}

func (p *problemLog) add(line string) {
	if !problemLevel.MatchString(line) {
		return
	}
	line = logTimestamp.ReplaceAllString(line, "")
	if slices.Contains(p.kept, line) {
		return
	}
	if len(p.kept) >= p.max {
		p.dropped++
		return
	}
	p.kept = append(p.kept, line)
}

func (p *problemLog) lines() []string {
	p.mut.Lock()
	defer p.mut.Unlock()

	out := slices.Clone(p.kept)
	if p.dropped > 0 {
		out = append(out, fmt.Sprintf("... and %d more", p.dropped))
	}
	return out
}
