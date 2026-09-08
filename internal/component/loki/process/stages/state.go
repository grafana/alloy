package stages

import (
	"sync"

	"go.uber.org/atomic"
)

func newSharedState[T any](sizeHint int, concurrentSafe bool) sharedState[T] {
	if concurrentSafe {
		return newStripedMapState[T](sizeHint)
	}
	return newMapState[T](sizeHint)
}

type sharedState[T any] interface {
	Take(key uint64) (T, bool)
	Update(key uint64, fn func(v T) T)
	DrainAll() []T
	DrainIfAtLeast(n int) []T
}

var _ sharedState[any] = (*mapState[any])(nil)

func newMapState[T any](sizeHint int) *mapState[T] {
	return &mapState[T]{
		m: make(map[uint64]T, sizeHint),
	}
}

type mapState[T any] struct {
	m map[uint64]T
}

func (u *mapState[T]) Take(key uint64) (T, bool) {
	v, ok := u.m[key]
	if ok {
		delete(u.m, key)
	}
	return v, ok
}

func (u *mapState[T]) Update(key uint64, fn func(v T) T) {
	u.m[key] = fn(u.m[key])
}

func (u *mapState[T]) DrainAll() []T {
	buf := make([]T, 0, len(u.m))
	for _, v := range u.m {
		buf = append(buf, v)
	}
	clear(u.m)
	return buf
}

func (u *mapState[T]) DrainIfAtLeast(n int) []T {
	if len(u.m) < n {
		return nil
	}
	return u.DrainAll()
}

const stripeCount = 16

var _ sharedState[any] = (*stripedMapState[any])(nil)

// stripedMapState is a concurrent map sharded into a fixed number of independently
// locked stripes.
type stripedMapState[T any] struct {
	// size is only exact while every stripe lock is held. Other readers treat it
	// as a hint. flushing keeps concurrent callers from all queueing on lockAll.
	size     atomic.Int64
	flushing atomic.Bool

	stripes [stripeCount]stripe[T]
}

type stripe[V any] struct {
	mu   sync.Mutex
	data map[uint64]V
}

// newStripedMapState creates a stripedMapState. sizeHint is the expected total number
// of entries across all stripes.
func newStripedMapState[T any](sizeHint int) *stripedMapState[T] {
	m := &stripedMapState[T]{}
	perStripe := sizeHint/stripeCount + 1
	for i := range m.stripes {
		m.stripes[i].data = make(map[uint64]T, perStripe)
	}
	return m
}

func (m *stripedMapState[T]) stripe(key uint64) *stripe[T] {
	return &m.stripes[key&(stripeCount-1)]
}

// Take removes key and returns its value, or the zero value of V and false
// if key was not present.
func (m *stripedMapState[T]) Take(key uint64) (T, bool) {
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

// Update runs fn under key's stripe lock with the current value stored at
// key, or the zero value of V if key is not present, and stores fn's result.
func (m *stripedMapState[T]) Update(key uint64, fn func(v T) T) {
	s := m.stripe(key)
	s.mu.Lock()
	defer s.mu.Unlock()

	_, existed := s.data[key]
	s.data[key] = fn(s.data[key])
	if !existed {
		m.size.Add(1)
	}
}

// DrainAll removes and returns every value in the map.
func (m *stripedMapState[T]) DrainAll() []T {
	m.lockAll()
	defer m.unlockAll()

	return m.drainLocked()
}

// DrainIfAtLeast removes and returns every value in the map if it holds at
// least n entries, otherwise it does nothing and returns nil. If another
// caller is already draining, this returns nil without waiting for it.
func (m *stripedMapState[T]) DrainIfAtLeast(n int) []T {
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

func (m *stripedMapState[T]) lockAll() {
	for i := range m.stripes {
		m.stripes[i].mu.Lock()
	}
}

func (m *stripedMapState[T]) unlockAll() {
	for i := len(m.stripes) - 1; i >= 0; i-- {
		m.stripes[i].mu.Unlock()
	}
}

// drainLocked returns every value in the map and clears it. Callers must
// hold every stripe lock.
func (m *stripedMapState[T]) drainLocked() []T {
	buf := make([]T, 0, m.size.Load())
	for i := range m.stripes {
		for _, v := range m.stripes[i].data {
			buf = append(buf, v)
		}
		clear(m.stripes[i].data)
	}
	m.size.Store(0)
	return buf
}
