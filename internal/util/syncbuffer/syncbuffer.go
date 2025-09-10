package syncbuffer

import (
	"bytes"
	"sync"
)

// Buffer wraps around a bytes.Buffer and makes it safe to use from
// multiple goroutines.
type Buffer struct {
	mut sync.RWMutex
	buf bytes.Buffer
}

func (sb *Buffer) Bytes() []byte {
	sb.mut.RLock()
	defer sb.mut.RUnlock()

	return sb.buf.Bytes()
}

func (sb *Buffer) Write(p []byte) (n int, err error) {
	sb.mut.Lock()
	defer sb.mut.Unlock()

	return sb.buf.Write(p)
}

func (sb *Buffer) String() string {
	sb.mut.RLock()
	defer sb.mut.RUnlock()

	return sb.buf.String()
}

func (sb *Buffer) Reset() {
	sb.mut.Lock()
	defer sb.mut.Unlock()
	sb.buf.Reset()
}
