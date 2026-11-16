package savepoint

import (
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
)

func TestSegmentTracker(t *testing.T) {
	var (
		logger  = slog.New(slog.DiscardHandler)
		metrics = NewMetrics(prometheus.NewRegistry())
	)

	t.Run("returns last saved segment from file on start", func(t *testing.T) {
		f, err := NewFile(logger, t.TempDir())
		require.NoError(t, err)
		f.StoreSegment("endpoint-a", 10)

		st := NewSegmentTracker(f, "endpoint-a", time.Minute, logger, metrics)
		st.Start()
		defer st.Stop()

		require.Equal(t, 10, st.LastStoredSegment())
	})

	t.Run("last saved segment is updated when sends complete", func(t *testing.T) {
		f, err := NewFile(logger, t.TempDir())
		require.NoError(t, err)
		f.StoreSegment("endpoint-a", 10)

		st := NewSegmentTracker(f, "endpoint-a", time.Minute, logger, metrics)
		st.Start()
		defer st.Stop()

		st.UpdateReceivedData(11, 10)
		// data was read from the next segment, so nothing is written to segment 11 anymore
		st.UpdateReceivedData(12, 1)
		st.UpdateSentData(11, 5)
		st.UpdateSentData(11, 5)

		require.Eventually(t, func() bool {
			return st.LastStoredSegment() == 11
		}, 3*time.Second, time.Millisecond*100, "expected last saved segment to catch up")
		require.Equal(t, 11, f.LastStoredSegment("endpoint-a"))
	})

	t.Run("segment that may still be written to is not saved when its sends complete", func(t *testing.T) {
		f, err := NewFile(logger, t.TempDir())
		require.NoError(t, err)
		f.StoreSegment("endpoint-a", 10)

		st := NewSegmentTracker(f, "endpoint-a", time.Minute, logger, metrics)
		st.Start()

		st.UpdateReceivedData(11, 10)
		st.UpdateSentData(11, 10)

		// stopping applies every update that is waiting, and the ticker has fired by now
		time.Sleep(1200 * time.Millisecond)
		st.Stop()

		require.Equal(t, 10, f.LastStoredSegment("endpoint-a"))
	})

	t.Run("segment that was written to last is saved when the WAL is drained", func(t *testing.T) {
		f, err := NewFile(logger, t.TempDir())
		require.NoError(t, err)
		f.StoreSegment("endpoint-a", 10)

		st := NewSegmentTracker(f, "endpoint-a", time.Minute, logger, metrics)
		st.Start()

		st.UpdateReceivedData(11, 10)
		st.UpdateSentData(11, 10)

		// the last updates are sent while the endpoint stops, and are still waiting when the tracker stops
		st.UpdateReceivedData(12, 2)
		st.UpdateSentData(12, 2)
		st.MarkWALDrained()
		st.Stop()

		require.Equal(t, 12, f.LastStoredSegment("endpoint-a"))
	})

	t.Run("drained WAL does not save a segment with data left to send", func(t *testing.T) {
		f, err := NewFile(logger, t.TempDir())
		require.NoError(t, err)
		f.StoreSegment("endpoint-a", 10)

		st := NewSegmentTracker(f, "endpoint-a", time.Minute, logger, metrics)
		st.Start()

		st.UpdateReceivedData(11, 10)
		st.UpdateSentData(11, 10)
		st.UpdateReceivedData(12, 2)
		st.UpdateSentData(12, 1)
		st.MarkWALDrained()
		st.Stop()

		require.Equal(t, 11, f.LastStoredSegment("endpoint-a"))
	})

	t.Run("last saved segment is updated when segment becomes old", func(t *testing.T) {
		f, err := NewFile(logger, t.TempDir())
		require.NoError(t, err)
		f.StoreSegment("endpoint-a", 10)

		st := NewSegmentTracker(f, "endpoint-a", 2*time.Second, logger, metrics)
		st.Start()
		defer st.Stop()

		// segment 11 has 5 pending data items, and will become old after 2 secs
		st.UpdateReceivedData(11, 10)
		st.UpdateSentData(11, 5)

		// wait until segment becomes old
		time.Sleep(2*time.Second + time.Millisecond*100)

		// send dummy data item to trigger find
		st.UpdateReceivedData(12, 1)

		require.Eventually(t, func() bool {
			return st.LastStoredSegment() == 11
		}, 3*time.Second, time.Millisecond*100, "expected last saved segment to catch up")
		require.Equal(t, 11, f.LastStoredSegment("endpoint-a"))
	})

	t.Run("endpoints sharing a file are tracked independently", func(t *testing.T) {
		f, err := NewFile(logger, t.TempDir())
		require.NoError(t, err)

		a := NewSegmentTracker(f, "endpoint-a", time.Minute, logger, metrics)
		a.Start()
		defer a.Stop()
		b := NewSegmentTracker(f, "endpoint-b", time.Minute, logger, metrics)
		b.Start()
		defer b.Stop()

		a.UpdateReceivedData(11, 1)
		a.UpdateReceivedData(12, 1)
		a.UpdateSentData(11, 1)

		require.Eventually(t, func() bool {
			return a.LastStoredSegment() == 11
		}, 3*time.Second, time.Millisecond*100, "expected endpoint-a to catch up")

		require.Equal(t, 11, a.LastStoredSegment())
		require.Equal(t, noSegment, b.LastStoredSegment())
	})
}

// noOpenSegment is an open segment that is higher than any segment in the tests, so every segment is closed.
const noOpenSegment = math.MaxInt

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
		require.Equal(t, 4, findMarkableSegment(data, time.Minute, noOpenSegment))
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
		require.Equal(t, 4, findMarkableSegment(data, time.Minute, noOpenSegment))
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
		require.Equal(t, 2, findMarkableSegment(data, time.Minute, noOpenSegment))
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
		require.Equal(t, -1, findMarkableSegment(data, time.Minute, noOpenSegment))
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
		require.Equal(t, 11, findMarkableSegment(data, time.Minute, noOpenSegment))
		require.Len(t, data, 0)
	})
}

func TestFindLastMarkableSegmentWithOpenSegment(t *testing.T) {
	t.Run("open segment is not marked even if its count is zero", func(t *testing.T) {
		data := map[int]*countDataItem{
			3: {count: 0, lastUpdate: time.Now()},
		}
		require.Equal(t, -1, findMarkableSegment(data, time.Minute, 3))
		require.Contains(t, data, 3, "open segment must be kept")
	})

	t.Run("open segment is not marked when it is too old", func(t *testing.T) {
		data := map[int]*countDataItem{
			3: {count: 4, lastUpdate: time.Now().Add(-2 * time.Minute)},
		}
		require.Equal(t, -1, findMarkableSegment(data, time.Minute, 3))
		require.Contains(t, data, 3)
	})

	t.Run("segments before the open one are marked", func(t *testing.T) {
		now := time.Now()
		data := map[int]*countDataItem{
			1: {count: 0, lastUpdate: now},
			2: {count: 0, lastUpdate: now},
			3: {count: 0, lastUpdate: now},
		}
		require.Equal(t, 2, findMarkableSegment(data, time.Minute, 3))
		require.NotContains(t, data, 1)
		require.NotContains(t, data, 2)
		require.Contains(t, data, 3)
	})

	t.Run("segments after the open one are not marked", func(t *testing.T) {
		now := time.Now()
		data := map[int]*countDataItem{
			1: {count: 0, lastUpdate: now},
			2: {count: 0, lastUpdate: now},
			3: {count: 0, lastUpdate: now},
		}
		require.Equal(t, 1, findMarkableSegment(data, time.Minute, 2))
	})
}
