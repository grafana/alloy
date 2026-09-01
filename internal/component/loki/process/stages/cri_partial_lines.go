package stages

import (
	"slices"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"
)

type partialLineStore struct {
	mut   sync.Mutex
	lines map[model.Fingerprint]Entry
	size  atomic.Int64

	cfg            CRIConfig
	linesTruncated prometheus.Counter
}

func newPartialLineStore(cfg CRIConfig, linesTruncated prometheus.Counter) *partialLineStore {
	return &partialLineStore{
		lines:          make(map[model.Fingerprint]Entry, cfg.MaxPartialLines),
		cfg:            cfg,
		linesTruncated: linesTruncated,
	}
}

func (s *partialLineStore) Size() int {
	return int(s.size.Load())
}

func (s *partialLineStore) Append(fp model.Fingerprint, e Entry) {
	s.mut.Lock()
	defer s.mut.Unlock()

	if prev, ok := s.lines[fp]; ok {
		e.Line = prev.Line + e.Line
	}
	truncatePartialLine(&e, s.cfg, s.linesTruncated)
	s.lines[fp] = e
	s.size.Store(int64(len(s.lines)))
}

func (s *partialLineStore) Take(fp model.Fingerprint) (Entry, bool) {
	s.mut.Lock()
	defer s.mut.Unlock()

	e, ok := s.lines[fp]
	if !ok {
		return Entry{}, false
	}
	delete(s.lines, fp)
	s.size.Store(int64(len(s.lines)))
	return e, true
}

func (s *partialLineStore) DrainIfAtLeast(n int, buf []Entry) []Entry {
	if s.Size() < n {
		return buf
	}

	s.mut.Lock()
	defer s.mut.Unlock()

	if len(s.lines) < n {
		return buf
	}
	return s.drainLocked(buf)
}

func (s *partialLineStore) DrainAll(buf []Entry) []Entry {
	s.mut.Lock()
	defer s.mut.Unlock()

	return s.drainLocked(buf)
}

func (s *partialLineStore) Reset() {
	s.mut.Lock()
	defer s.mut.Unlock()

	clear(s.lines)
	s.size.Store(0)
}

func (s *partialLineStore) drainLocked(buf []Entry) []Entry {
	buf = slices.Grow(buf, len(s.lines))
	for _, e := range s.lines {
		buf = append(buf, e)
	}
	clear(s.lines)
	s.size.Store(0)
	return buf
}
