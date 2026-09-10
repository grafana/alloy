package client

import (
	"fmt"
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/loki/pkg/push"
)

// newTestBatch builds a batch holding streams streams of entriesPerStream
// entries each. Lines are unique so entries can be told apart.
func newTestBatch(t *testing.T, streams, entriesPerStream int) *batch {
	t.Helper()

	b := newBatch(0, int(1*units.MiB))
	for s := range streams {
		for e := range entriesPerStream {
			entry := loki.NewEntry(
				model.LabelSet{"app": model.LabelValue(fmt.Sprintf("app-%d", s))},
				push.Entry{
					Timestamp: time.Unix(int64(e), 0).UTC(),
					Line:      fmt.Sprintf("stream-%d line-%d", s, e),
				},
			)
			require.NoError(t, b.add(entry, 0))
		}
	}
	return b
}

// lines returns every log line in the batch, keyed by stream labels.
func lines(b *batch) map[string][]string {
	out := make(map[string][]string, len(b.streams))
	for labels, stream := range b.streams {
		for _, e := range stream.Entries {
			out[labels] = append(out[labels], e.Line)
		}
	}
	return out
}

func entryCount(b *batch) int {
	var n int
	for _, stream := range b.streams {
		n += len(stream.Entries)
	}
	return n
}

func TestBatch_split_byStreams(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		streams         int
		expectedStreams [2]int
	}{
		{name: "two streams", streams: 2, expectedStreams: [2]int{1, 1}},
		{name: "three streams", streams: 3, expectedStreams: [2]int{1, 2}},
		{name: "four streams", streams: 4, expectedStreams: [2]int{2, 2}},
		{name: "nine streams", streams: 9, expectedStreams: [2]int{4, 5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := newTestBatch(t, tt.streams, 2)
			original := lines(b)

			split1, split2, ok := b.split()
			require.True(t, ok)

			require.Len(t, split1.streams, tt.expectedStreams[0])
			require.Len(t, split2.streams, tt.expectedStreams[1])

			// Which stream ends up in which half depends on map iteration
			// order, so only assert the halves partition the original: every
			// stream appears exactly once, with all of its entries intact.
			combined := lines(split1)
			for labels, l := range lines(split2) {
				require.NotContains(t, combined, labels, "stream in both halves")
				combined[labels] = l
			}
			require.Equal(t, original, combined)

			require.Equal(t, entryCount(b), entryCount(split1)+entryCount(split2))
			require.Equal(t, b.size, split1.size+split2.size)
		})
	}
}

func TestBatch_split_byEntries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		entries         int
		expectedEntries [2]int
	}{
		{name: "two entries", entries: 2, expectedEntries: [2]int{1, 1}},
		{name: "three entries", entries: 3, expectedEntries: [2]int{1, 2}},
		{name: "four entries", entries: 4, expectedEntries: [2]int{2, 2}},
		{name: "nine entries", entries: 9, expectedEntries: [2]int{4, 5}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			b := newTestBatch(t, 1, tt.entries)
			labels := `{app="app-0"}`

			split1, split2, ok := b.split()
			require.True(t, ok)

			// Both halves keep the single stream, and its entries are divided
			// in order: the first half holds the earlier entries.
			require.Len(t, split1.streams, 1)
			require.Len(t, split2.streams, 1)

			mid := tt.expectedEntries[0]
			require.Equal(t, lines(b)[labels][:mid], lines(split1)[labels])
			require.Equal(t, lines(b)[labels][mid:], lines(split2)[labels])

			require.Equal(t, b.size, split1.size+split2.size)
		})
	}
}

func TestBatch_split_indivisible(t *testing.T) {
	t.Parallel()

	t.Run("single entry", func(t *testing.T) {
		t.Parallel()

		split1, split2, ok := newTestBatch(t, 1, 1).split()
		require.False(t, ok, "a batch holding one entry cannot be divided")
		require.Nil(t, split1)
		require.Nil(t, split2)
	})

	t.Run("empty batch", func(t *testing.T) {
		t.Parallel()

		split1, split2, ok := newBatch(0, int(1*units.MiB)).split()
		require.False(t, ok)
		require.Nil(t, split1)
		require.Nil(t, split2)
	})
}

func TestBatch_split_carriesLimitsButNotReporting(t *testing.T) {
	t.Parallel()

	b := newBatch(7, 4096)
	entry := loki.NewEntry(model.LabelSet{"app": "app-1"}, push.Entry{Line: "line"})
	require.NoError(t, b.add(entry, 3))
	require.NoError(t, b.add(entry, 4))

	for _, half := range []*batch{mustSplit1(t, b), mustSplit2(t, b)} {
		require.Equal(t, b.maxStreams, half.maxStreams)
		require.Equal(t, b.maxSize, half.maxSize)
		require.Equal(t, b.createdAt, half.createdAt)

		// Only the batch the halves were divided from reports sent data and
		// entry latency, so the halves must carry neither. reportAsSentData on
		// a half has to be a no-op, otherwise splitting would double-count
		// WAL segments and latency samples.
		require.Empty(t, half.created)
		require.Empty(t, half.segmentCounter)

		tracker := &fakeSentDataTracker{}
		half.reportAsSentData(tracker, discardObserver{})
		require.Empty(t, tracker.calls)
	}
}

func mustSplit1(t *testing.T, b *batch) *batch {
	t.Helper()
	split1, _, ok := b.split()
	require.True(t, ok)
	return split1
}

func mustSplit2(t *testing.T, b *batch) *batch {
	t.Helper()
	_, split2, ok := b.split()
	require.True(t, ok)
	return split2
}

type fakeSentDataTracker struct {
	calls []int
}

func (f *fakeSentDataTracker) UpdateSentData(segmentID, dataCount int) {
	f.calls = append(f.calls, segmentID, dataCount)
}

type discardObserver struct{}

func (discardObserver) Observe(float64) {}
