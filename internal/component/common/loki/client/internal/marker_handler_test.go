package internal

import (
	"os"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

type mockMarkerFileHandler struct {
	lastMarkedSegment atomic.Int64
}

func newMockMarkerFileHandler(seg int) *mockMarkerFileHandler {
	mh := &mockMarkerFileHandler{}
	mh.MarkSegment(seg)
	return mh
}

func (m *mockMarkerFileHandler) LastMarkedSegment() int {
	return int(m.lastMarkedSegment.Load())
}

func (m *mockMarkerFileHandler) MarkSegment(segment int) {
	m.lastMarkedSegment.Store(int64(segment))
}

func TestMarkerHandler(t *testing.T) {
	logger := log.NewLogfmtLogger(os.Stdout)
	// drive-by test: if metrics don't have the id curried, it panics when emitting them
	metrics := NewMarkerMetrics(nil).WithCurriedId("test")
	t.Run("returns last marked segment from file handler on start", func(t *testing.T) {
		mockMFH := newMockMarkerFileHandler(10)
		mh := NewMarkerHandler(mockMFH, time.Minute, logger, metrics)
		defer mh.Stop()

		require.Equal(t, 10, mh.LastMarkedSegment())
	})

	t.Run("last marked segment is updated when sends complete", func(t *testing.T) {
		mockMFH := newMockMarkerFileHandler(10)
		mh := NewMarkerHandler(mockMFH, time.Minute, logger, metrics)
		defer mh.Stop()

		mh.UpdateReceivedData(11, 10)
		mh.UpdateSentData(11, 5)
		mh.UpdateSentData(11, 5)

		require.Eventually(t, func() bool {
			return mh.LastMarkedSegment() == 11
		}, time.Second, time.Millisecond*100, "expected last marked segment to catch up")
		require.Equal(t, 11, mockMFH.LastMarkedSegment())
	})

	t.Run("last marked segment is updated when segment becomes old", func(t *testing.T) {
		mockMFH := newMockMarkerFileHandler(10)
		mh := NewMarkerHandler(mockMFH, 2*time.Second, logger, metrics)
		defer mh.Stop()

		// segment 11 has 5 pending data items, and will become old after 2 secs
		mh.UpdateReceivedData(11, 10)
		mh.UpdateSentData(11, 5)

		// wait until segment becomes old
		time.Sleep(2*time.Second + time.Millisecond*100)

		// send dummy data item to trigger find
		mh.UpdateReceivedData(12, 1)

		require.Eventually(t, func() bool {
			return mh.LastMarkedSegment() == 11
		}, 3*time.Second, time.Millisecond*100, "expected last marked segment to catch up")
		require.Equal(t, 11, mockMFH.LastMarkedSegment())
	})
}

func TestFindLastMarkableSegment(t *testing.T) {
	t.Run("all segments with count zero, highest numbered should be marked", func(t *testing.T) {
		now := time.Now()
		data := map[int]*countDataItem{
			1: {
				count:      0,
				lastUpdate: now,
			},
			2: {
				count:      0,
				lastUpdate: now,
			},
			3: {
				count:      0,
				lastUpdate: now,
			},
			4: {
				count:      0,
				lastUpdate: now,
			},
		}
		require.Equal(t, 4, FindMarkableSegment(data, time.Minute))
	})

	t.Run("all segments with count zero, and one too old, highest numbered should be marked", func(t *testing.T) {
		now := time.Now()
		data := map[int]*countDataItem{
			1: {
				count:      0,
				lastUpdate: now,
			},
			2: {
				count:      0,
				lastUpdate: now,
			},
			3: {
				count:      10,
				lastUpdate: now.Add(-2 * time.Minute),
			},
			4: {
				count:      0,
				lastUpdate: now,
			},
		}
		require.Equal(t, 4, FindMarkableSegment(data, time.Minute))
		// items that should have been cleanup up
		require.Len(t, data, 0)
	})
	t.Run("should find the zeroed segment before the last non-zero", func(t *testing.T) {
		now := time.Now()
		data := map[int]*countDataItem{
			1: {
				count:      0,
				lastUpdate: now,
			},
			2: {
				count:      0,
				lastUpdate: now,
			},
			3: {
				count:      10,
				lastUpdate: now,
			},
			4: {
				count:      0,
				lastUpdate: now,
			},
		}
		require.Equal(t, 2, FindMarkableSegment(data, time.Minute))
		require.NotContains(t, data, 1)
		require.NotContains(t, data, 2)
	})
	t.Run("should return -1 when no segment is markable", func(t *testing.T) {
		now := time.Now()
		data := map[int]*countDataItem{
			1: {
				count:      11,
				lastUpdate: now,
			},
			2: {
				count:      5,
				lastUpdate: now,
			},
			3: {
				count:      10,
				lastUpdate: now,
			},
			4: {
				count:      2,
				lastUpdate: now,
			},
		}
		lenBefore := len(data)
		require.Equal(t, -1, FindMarkableSegment(data, time.Minute))
		require.Len(t, data, lenBefore, "none key should have been deleted")
	})
	t.Run("should find only item with zero, and clean it up", func(t *testing.T) {
		now := time.Now()
		data := map[int]*countDataItem{
			11: {
				count:      0,
				lastUpdate: now,
			},
		}
		require.Equal(t, 11, FindMarkableSegment(data, time.Minute))
		require.Len(t, data, 0)
	})
}
