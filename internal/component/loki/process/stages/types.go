package stages

import (
	"encoding"
	"errors"
	"fmt"
	"sync"

	"go.uber.org/atomic"
)

// Source names a value in the extracted map for stages that read their input
// from it. Unlike SourceType below (which selects the kind of location a
// stage reads from), a Source is the non-empty key itself; rejecting the
// empty string at decode time keeps misconfigurations out of the pipeline.
type Source string

var (
	_ encoding.TextMarshaler   = Source("")
	_ encoding.TextUnmarshaler = (*Source)(nil)
)

// UnmarshalText implements encoding.TextUnmarshaler.
func (s *Source) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		return errors.New("source cannot be empty")
	}
	*s = Source(text)
	return nil
}

// MarshalText implements encoding.TextMarshaler.
func (s Source) MarshalText() (text []byte, err error) {
	return []byte(s), nil
}

type SourceType string

const (
	SourceTypeLine               SourceType = "line"
	SourceTypeLabel              SourceType = "label"
	SourceTypeStructuredMetadata SourceType = "structured_metadata"
	SourceTypeExtractedMap       SourceType = "extracted"
)

var (
	_ encoding.TextMarshaler   = SourceType("")
	_ encoding.TextUnmarshaler = (*SourceType)(nil)
)

// UnmarshalText implements encoding.TextUnmarshaler.
func (t *SourceType) UnmarshalText(text []byte) error {
	str := string(text)
	switch str {
	case string(SourceTypeLine), string(SourceTypeLabel), string(SourceTypeStructuredMetadata), string(SourceTypeExtractedMap):
		*t = SourceType(str)
	default:
		return fmt.Errorf("unknown source_type: %s", str)
	}

	return nil
}

// MarshalText implements encoding.TextMarshaler.
func (t SourceType) MarshalText() (text []byte, err error) {
	return []byte(t), nil
}

const stripedMapStripeCount = 16

// stripedMap is a concurrent map sharded into a fixed number of independently
// locked stripes.
type stripedMap[V any] struct {
	// size is only exact while every stripe lock is held. Other readers treat it
	// as a hint. flushing keeps concurrent callers from all queueing on lockAll.
	size     atomic.Int64
	flushing atomic.Bool

	stripes [stripedMapStripeCount]stripe[V]
}

type stripe[V any] struct {
	mu   sync.Mutex
	data map[uint64]V
}

// newStripedMap creates a stripedMap. sizeHint is the expected total number
// of entries across all stripes.
func newStripedMap[V any](sizeHint int) *stripedMap[V] {
	m := &stripedMap[V]{}
	perStripe := sizeHint/stripedMapStripeCount + 1
	for i := range m.stripes {
		m.stripes[i].data = make(map[uint64]V, perStripe)
	}
	return m
}

func (m *stripedMap[V]) stripe(key uint64) *stripe[V] {
	return &m.stripes[key&(stripedMapStripeCount-1)]
}

// Update runs fn under key's stripe lock with the current value stored at
// key, or the zero value of V if key is not present, and stores fn's result.
func (m *stripedMap[V]) Update(key uint64, fn func(v V) V) {
	s := m.stripe(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	_, existed := s.data[key]
	s.data[key] = fn(s.data[key])
	if !existed {
		m.size.Add(1)
	}
}

// Take removes key and returns its value, or the zero value of V and false
// if key was not present.
func (m *stripedMap[V]) Take(key uint64) (V, bool) {
	s := m.stripe(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	v, ok := s.data[key]
	if ok {
		delete(s.data, key)
		m.size.Add(-1)
	}
	return v, ok
}

// DrainIfAtLeast removes and returns every value in the map if it holds at
// least n entries, otherwise it does nothing and returns nil. If another
// caller is already draining, this returns nil without waiting for it.
func (m *stripedMap[V]) DrainIfAtLeast(n int) []V {
	if int(m.size.Load()) < n || !m.flushing.CompareAndSwap(false, true) {
		return nil
	}
	defer m.flushing.Store(false)

	m.lockAll()
	defer m.unlockAll()

	// Another caller may have drained the map while this one waited for the
	// stripe locks, so re-check the exact size now that everything is held.
	if int(m.size.Load()) < n {
		return nil
	}
	return m.drainLocked()
}

// DrainAll removes and returns every value in the map.
func (m *stripedMap[V]) DrainAll() []V {
	m.lockAll()
	defer m.unlockAll()

	return m.drainLocked()
}

func (m *stripedMap[V]) lockAll() {
	for i := range m.stripes {
		m.stripes[i].mu.Lock()
	}
}

func (m *stripedMap[V]) unlockAll() {
	for i := len(m.stripes) - 1; i >= 0; i-- {
		m.stripes[i].mu.Unlock()
	}
}

// drainLocked returns every value in the map and clears it. Callers must
// hold every stripe lock.
func (m *stripedMap[V]) drainLocked() []V {
	buf := make([]V, 0, m.size.Load())
	for i := range m.stripes {
		for _, v := range m.stripes[i].data {
			buf = append(buf, v)
		}
		clear(m.stripes[i].data)
	}
	m.size.Store(0)
	return buf
}
