package ring

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

// SignalType identifies the OTLP signal carried by an entry.
type SignalType string

const (
	SignalTypeTraces  SignalType = "traces"
	SignalTypeLogs    SignalType = "logs"
	SignalTypeMetrics SignalType = "metrics"
)

var ErrEntryTooLarge = errors.New("entry exceeds max_bytes")

// Entry is a buffered serialized OTLP payload.
type Entry struct {
	Payload    []byte
	Size       int
	InsertedAt time.Time
	SignalType SignalType
}

// Utilization reflects the current memory footprint of the ring buffer.
type Utilization struct {
	TotalBytes int
	EntryCount int
}

// Buffer is a thread-safe FIFO ring buffer with byte-capacity and age eviction.
type Buffer struct {
	mut sync.Mutex

	entries []Entry
	head    int
	count   int

	totalBytes int
	maxBytes   int
	maxAge     time.Duration
	now        func() time.Time
}

// NewBuffer creates a new buffer.
func NewBuffer(maxBytes int, maxAge time.Duration) (*Buffer, error) {
	return newBuffer(maxBytes, maxAge, time.Now)
}

func newBuffer(maxBytes int, maxAge time.Duration, now func() time.Time) (*Buffer, error) {
	if maxBytes <= 0 {
		return nil, fmt.Errorf("max_bytes must be greater than zero")
	}
	if maxAge <= 0 {
		return nil, fmt.Errorf("max_age must be greater than zero")
	}
	if now == nil {
		return nil, fmt.Errorf("now function must not be nil")
	}

	return &Buffer{
		maxBytes: maxBytes,
		maxAge:   maxAge,
		now:      now,
	}, nil
}

// Insert adds a serialized OTLP payload and evicts oldest entries as needed.
func (b *Buffer) Insert(payload []byte, signalType SignalType) error {
	size := len(payload)
	if size > b.maxBytes {
		return fmt.Errorf("%w: size=%d max_bytes=%d", ErrEntryTooLarge, size, b.maxBytes)
	}

	b.mut.Lock()
	defer b.mut.Unlock()

	now := b.now()
	b.evictExpiredLocked(now)

	for b.count > 0 && b.totalBytes+size > b.maxBytes {
		b.evictOldestLocked()
	}

	if b.totalBytes+size > b.maxBytes {
		return fmt.Errorf("%w: size=%d max_bytes=%d", ErrEntryTooLarge, size, b.maxBytes)
	}

	b.ensureSpaceLocked()

	idx := (b.head + b.count) % len(b.entries)
	b.entries[idx] = Entry{
		Payload:    append([]byte(nil), payload...),
		Size:       size,
		InsertedAt: now,
		SignalType: signalType,
	}
	b.count++
	b.totalBytes += size
	return nil
}

// EvictExpired removes entries older than max_age.
func (b *Buffer) EvictExpired() {
	b.mut.Lock()
	defer b.mut.Unlock()
	b.evictExpiredLocked(b.now())
}

// Utilization returns up-to-date aggregate usage.
func (b *Buffer) Utilization() Utilization {
	b.mut.Lock()
	defer b.mut.Unlock()

	return Utilization{
		TotalBytes: b.totalBytes,
		EntryCount: b.count,
	}
}

// Snapshot returns entries in FIFO order from oldest to newest.
func (b *Buffer) Snapshot() []Entry {
	b.mut.Lock()
	defer b.mut.Unlock()

	out := make([]Entry, 0, b.count)
	for i := 0; i < b.count; i++ {
		entry := b.entries[(b.head+i)%len(b.entries)]
		entry.Payload = append([]byte(nil), entry.Payload...)
		out = append(out, entry)
	}
	return out
}

func (b *Buffer) ensureSpaceLocked() {
	if len(b.entries) == 0 {
		b.entries = make([]Entry, 1)
		return
	}
	if b.count < len(b.entries) {
		return
	}

	newEntries := make([]Entry, len(b.entries)*2)
	for i := 0; i < b.count; i++ {
		newEntries[i] = b.entries[(b.head+i)%len(b.entries)]
	}
	b.entries = newEntries
	b.head = 0
}

func (b *Buffer) evictExpiredLocked(now time.Time) {
	for b.count > 0 {
		oldest := b.entries[b.head]
		if now.Sub(oldest.InsertedAt) <= b.maxAge {
			return
		}
		b.evictOldestLocked()
	}
}

func (b *Buffer) evictOldestLocked() {
	if b.count == 0 {
		return
	}

	oldest := b.entries[b.head]
	b.totalBytes -= oldest.Size
	b.entries[b.head] = Entry{}
	b.head = (b.head + 1) % len(b.entries)
	b.count--

	if b.count == 0 {
		b.head = 0
	}
}
