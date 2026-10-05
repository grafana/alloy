package logging

import (
	"strings"
	"sync"
)

// Buffer retains the most recent formatted log output up to a fixed size.
type Buffer struct {
	mut           sync.RWMutex
	maxBytes      int
	data          []byte
	startsMidLine bool
}

// NewBuffer creates a log buffer that retains at most maxBytes of output.
func NewBuffer(maxBytes int) *Buffer {
	return &Buffer{
		maxBytes: max(maxBytes, 0),
	}
}

// Write appends formatted log output to the buffer.
func (b *Buffer) Write(p []byte) (int, error) {
	b.mut.Lock()
	defer b.mut.Unlock()

	if b.maxBytes == 0 {
		b.data = nil
		b.startsMidLine = false
		return len(p), nil
	}

	overflow := len(b.data) + len(p) - b.maxBytes
	switch {
	case overflow <= 0:
		b.data = append(b.data, p...)
	case overflow < len(b.data):
		b.startsMidLine = b.data[overflow-1] != '\n'
		b.data = append(b.data[:0], b.data[overflow:]...)
		b.data = append(b.data, p...)
	default:
		start := overflow - len(b.data)
		b.startsMidLine = start > 0 && p[start-1] != '\n'
		b.data = append(b.data[:0], p[start:]...)
	}

	return len(p), nil
}

// Lines returns complete log lines in the requested page.
func (b *Buffer) Lines(offset, limit int) []string {
	b.mut.RLock()
	data := string(b.data)
	startsMidLine := b.startsMidLine
	b.mut.RUnlock()

	if limit == 0 {
		return []string{}
	}

	lines := strings.Split(data, "\n")
	if startsMidLine {
		lines = lines[1:]
	}
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	} else if len(lines) > 0 {
		lines = lines[:len(lines)-1]
	}

	if offset >= len(lines) {
		return []string{}
	}
	lines = lines[offset:]
	if len(lines) > limit {
		lines = lines[:limit]
	}
	return lines
}
