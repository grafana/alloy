package gather

import "sync"

// logRing is a fixed-size byte ring buffer. It always retains the most recent
// bytes written to it, up to its size, evicting the oldest. A single mutex
// guards it. A benchmark (logs_bench_test.go) showed a lock-free rewrite does
// not meaningfully beat this under contention, because a single ordered append
// point is inherently one synchronization point.
type logRing struct {
	mu   sync.Mutex
	buf  []byte
	pos  int  // next write index
	full bool // the ring has wrapped at least once
}

func newLogRing(size int) *logRing { return &logRing{buf: make([]byte, size)} }

func (r *logRing) Write(p []byte) (int, error) {
	n := len(p)
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.buf) == 0 {
		return n, nil
	}
	// A line at least as large as the ring: keep only its tail.
	if len(p) >= len(r.buf) {
		copy(r.buf, p[len(p)-len(r.buf):])
		r.pos = 0
		r.full = true
		return n, nil
	}
	end := r.pos + len(p)
	if end <= len(r.buf) {
		copy(r.buf[r.pos:], p)
	} else {
		first := len(r.buf) - r.pos
		copy(r.buf[r.pos:], p[:first])
		copy(r.buf, p[first:])
	}
	if end >= len(r.buf) {
		r.full = true
	}
	r.pos = end % len(r.buf)
	return n, nil
}

// snapshot returns the retained bytes in chronological order and reports whether
// the ring had wrapped (so the oldest bytes were evicted and the first line may
// be partial).
func (r *logRing) snapshot() (data []byte, evicted bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.full {
		out := make([]byte, r.pos)
		copy(out, r.buf[:r.pos])
		return out, false
	}
	out := make([]byte, len(r.buf))
	n := copy(out, r.buf[r.pos:])
	copy(out[n:], r.buf[:r.pos])
	return out, true
}
