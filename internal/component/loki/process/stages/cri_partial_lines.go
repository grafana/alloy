package stages

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"go.uber.org/atomic"
)

const partialLineStripeCount = 16

type partialLineStripe struct {
	mut   sync.Mutex
	lines map[model.Fingerprint]Entry
}

type partialLineStore struct {
	// size is only exact while every stripe lock is held. Other readers treat it
	// as a hint. flushing keeps concurrent callers from all queueing on lockAll.
	size     atomic.Int64
	flushing atomic.Bool

	stripes [partialLineStripeCount]partialLineStripe

	cfg            CRIConfig
	linesTruncated prometheus.Counter
}

func newPartialLineStore(cfg CRIConfig, linesTruncated prometheus.Counter) *partialLineStore {
	s := &partialLineStore{
		cfg:            cfg,
		linesTruncated: linesTruncated,
	}
	perStripe := cfg.MaxPartialLines/partialLineStripeCount + 1
	for i := range s.stripes {
		s.stripes[i].lines = make(map[model.Fingerprint]Entry, perStripe)
	}
	return s
}

func (s *partialLineStore) stripe(fp model.Fingerprint) *partialLineStripe {
	return &s.stripes[uint64(fp)&(partialLineStripeCount-1)]
}

func (s *partialLineStore) Size() int {
	return int(s.size.Load())
}

func (s *partialLineStore) Append(fp model.Fingerprint, e Entry) {
	st := s.stripe(fp)
	st.mut.Lock()
	defer st.mut.Unlock()

	prev, existed := st.lines[fp]
	if existed {
		e.Line = prev.Line + e.Line
	}
	truncatePartialLine(&e, s.cfg, s.linesTruncated)
	st.lines[fp] = e
	if !existed {
		s.size.Add(1)
	}
}

func (s *partialLineStore) Take(fp model.Fingerprint) (Entry, bool) {
	st := s.stripe(fp)
	st.mut.Lock()
	defer st.mut.Unlock()

	e, ok := st.lines[fp]
	if !ok {
		return Entry{}, false
	}
	delete(st.lines, fp)
	s.size.Add(-1)
	return e, true
}

func (s *partialLineStore) DrainIfAtLeast(n int) []Entry {
	if s.Size() < n {
		return nil
	}
	if !s.flushing.CompareAndSwap(false, true) {
		return nil
	}
	defer s.flushing.Store(false)

	s.lockAll()
	defer s.unlockAll()

	// Another caller may already have drained the store while this one waited.
	if int(s.size.Load()) < n {
		return nil
	}
	return s.drainLocked()
}

func (s *partialLineStore) DrainAll() []Entry {
	s.lockAll()
	defer s.unlockAll()

	return s.drainLocked()
}

func (s *partialLineStore) Reset() {
	s.lockAll()
	defer s.unlockAll()

	for i := range s.stripes {
		clear(s.stripes[i].lines)
	}
	s.size.Store(0)
}

func (s *partialLineStore) lockAll() {
	for i := range s.stripes {
		s.stripes[i].mut.Lock()
	}
}

func (s *partialLineStore) unlockAll() {
	for i := len(s.stripes) - 1; i >= 0; i-- {
		s.stripes[i].mut.Unlock()
	}
}

func (s *partialLineStore) drainLocked() []Entry {
	buf := make([]Entry, 0, s.size.Load())
	for i := range s.stripes {
		for _, e := range s.stripes[i].lines {
			buf = append(buf, e)
		}
		clear(s.stripes[i].lines)
	}
	s.size.Store(0)
	return buf
}
