package stages

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func partialLineStoreCount(s *partialLineStore) int {
	s.lockAll()
	defer s.unlockAll()

	n := 0
	for i := range s.stripes {
		n += len(s.stripes[i].lines)
	}
	return n
}

func partialLineTestEntry(fp int, line string) (model.Fingerprint, Entry) {
	labels := model.LabelSet{
		"pod":     model.LabelValue(fmt.Sprintf("pod-%d", fp)),
		criStream: "stdout",
	}
	return labels.Fingerprint(), newEntry(make(map[string]any, 4), labels, line, time.Now())
}

func TestPartialLineStore_SizeTracksContents(t *testing.T) {
	s := newPartialLineStore(CRIConfig{MaxPartialLines: 100}, nil)

	fpA, entryA := partialLineTestEntry(1, "a ")
	fpB, entryB := partialLineTestEntry(2, "b ")

	require.Zero(t, s.Size())

	s.Append(fpA, entryA)
	require.Equal(t, 1, s.Size())

	s.Append(fpA, entryA)
	require.Equal(t, 1, s.Size(), "merging into an existing stream must not grow the size")

	s.Append(fpB, entryB)
	require.Equal(t, 2, s.Size())

	got, ok := s.Take(fpA)
	require.True(t, ok)
	require.Equal(t, "a a ", got.Line, "Take must return the merged line")
	require.Equal(t, 1, s.Size())

	_, ok = s.Take(fpA)
	require.False(t, ok, "a second Take of the same stream must find nothing")
	require.Equal(t, 1, s.Size())

	drained := s.DrainAll()
	require.Len(t, drained, 1)
	require.Zero(t, s.Size())
	require.Zero(t, partialLineStoreCount(s))
}

func TestPartialLineStore_SizeMatchesContentsUnderConcurrency(t *testing.T) {
	const (
		goroutines = 8
		iterations = 3000
		streams    = 6
	)

	s := newPartialLineStore(CRIConfig{MaxPartialLines: 4}, nil)

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()

			ops := make([]struct {
				fp model.Fingerprint
				e  Entry
			}, streams)
			for i := range ops {
				ops[i].fp, ops[i].e = partialLineTestEntry(g*streams+i, fmt.Sprintf("worker %d ", g))
			}

			for range iterations {
				for _, op := range ops {
					s.Append(op.fp, op.e)
				}
				for _, op := range ops {
					s.Take(op.fp)
				}
				s.DrainIfAtLeast(4)
			}
		}(g)
	}
	wg.Wait()

	require.Equal(t, partialLineStoreCount(s), s.Size(),
		"size must still match the entries actually held after concurrent appends, takes and drains")
}

func TestPartialLineStore_DrainEmitsEachEntryOnce(t *testing.T) {
	const (
		goroutines = 8
		streams    = 8
	)

	s := newPartialLineStore(CRIConfig{MaxPartialLines: 4}, nil)

	var (
		mut     sync.Mutex
		emitted []Entry
	)
	collect := func(entries []Entry) {
		mut.Lock()
		defer mut.Unlock()
		emitted = append(emitted, entries...)
	}

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()

			for i := range streams {
				fp, e := partialLineTestEntry(g*streams+i, fmt.Sprintf("w%d-s%d ", g, i))
				collect(s.DrainIfAtLeast(4))
				s.Append(fp, e)
			}
			for i := range streams {
				fp, _ := partialLineTestEntry(g*streams+i, "")
				if taken, ok := s.Take(fp); ok {
					collect([]Entry{taken})
				}
			}
		}(g)
	}
	wg.Wait()

	collect(s.DrainAll())

	seen := make(map[string]int, len(emitted))
	for _, e := range emitted {
		seen[e.Line]++
	}
	for line, count := range seen {
		require.Equal(t, 1, count, "line %q was emitted %d times, every entry must leave the store exactly once", line, count)
	}
	require.Len(t, emitted, goroutines*streams, "every appended stream must be emitted")
	require.Zero(t, s.Size())
}
