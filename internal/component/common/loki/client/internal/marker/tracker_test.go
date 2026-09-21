package marker

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/util"
)

const (
	// testFindInterval drives the tracker's find markable segment routine far
	// faster than defaultFindInterval, so the tests don't have to wait a second
	// for a forced run.
	testFindInterval = 10 * time.Millisecond
	// testMaxSegmentAge is short for the same reason: it's how long a segment
	// with data still in flight has to go without updates to count as consumed.
	testMaxSegmentAge = 100 * time.Millisecond
	// eventuallyWait still leaves generous headroom over the intervals above, so
	// a loaded CI runner doesn't fail the assertion.
	eventuallyWait = 5 * time.Second
)

func TestTracker(t *testing.T) {
	logger := util.TestAlloyLogger(t).Slog()
	// drive-by test: if metrics don't have the id curried, it panics when emitting them
	metrics := NewMetrics(nil).CurryWithId("test")
	t.Run("returns last marked segment from file handler on start", func(t *testing.T) {
		f, err := NewFile(logger, t.TempDir())
		require.NoError(t, err)
		require.NoError(t, f.MarkSegment(10))

		st := NewSegmentTracker(f, time.Minute, logger, metrics)
		defer st.Stop()

		require.Equal(t, 10, st.LastMarkedSegment())
	})

	t.Run("last marked segment is updated when sends complete", func(t *testing.T) {
		f, err := NewFile(logger, t.TempDir())
		require.NoError(t, err)
		require.NoError(t, f.MarkSegment(10))

		st := newSegmentTracker(f, time.Minute, testFindInterval, logger, metrics)
		defer st.Stop()

		st.UpdateReceivedData(11, 10)
		st.UpdateSentData(11, 5)
		st.UpdateSentData(11, 5)

		require.Eventually(t, func() bool {
			return st.LastMarkedSegment() == 11
		}, eventuallyWait, testFindInterval, "expected last marked segment to catch up")
		require.Equal(t, 11, f.LastMarkedSegment())
	})

	t.Run("last marked segment is updated when segment becomes old", func(t *testing.T) {
		f, err := NewFile(logger, t.TempDir())
		require.NoError(t, err)
		require.NoError(t, f.MarkSegment(10))

		st := newSegmentTracker(f, testMaxSegmentAge, testFindInterval, logger, metrics)
		defer st.Stop()

		// segment 11 has 5 pending data items, and will become old after
		// testMaxSegmentAge
		st.UpdateReceivedData(11, 10)
		st.UpdateSentData(11, 5)

		// wait until segment becomes old
		time.Sleep(testMaxSegmentAge + testFindInterval)

		// send a dummy data item, so the find also runs on a data update and not
		// only on runFindTicker
		st.UpdateReceivedData(12, 1)

		require.Eventually(t, func() bool {
			return st.LastMarkedSegment() == 11
		}, eventuallyWait, testFindInterval, "expected last marked segment to catch up")
		require.Equal(t, 11, f.LastMarkedSegment())
	})

	t.Run("retries marking a segment after a failed write", func(t *testing.T) {
		dir := t.TempDir()
		f, err := NewFile(logger, dir)
		require.NoError(t, err)
		require.NoError(t, f.MarkSegment(10))

		st := newSegmentTracker(f, time.Minute, testFindInterval, logger, metrics)
		defer st.Stop()

		// Replace the marker folder with a regular file, so the atomic write can't
		// create its temporary file and MarkSegment fails. Unlike permission bits,
		// this behaves the same on every platform.
		markerDir := filepath.Join(dir, markerFolderName)
		require.NoError(t, os.RemoveAll(markerDir))
		require.NoError(t, os.WriteFile(markerDir, []byte("not a directory"), 0o600))
		require.Error(t, f.MarkSegment(11), "expected marking to fail while the folder is a file")

		st.UpdateReceivedData(11, 10)
		st.UpdateSentData(11, 10)

		// The tracker must not report a segment it failed to persist.
		time.Sleep(5 * testFindInterval)
		require.NotEqual(t, 11, st.LastMarkedSegment())

		// Once writing is possible again, the pending segment must still be marked.
		require.NoError(t, os.Remove(markerDir))
		require.NoError(t, os.MkdirAll(markerDir, markerFolderMode))

		require.Eventually(t, func() bool {
			return st.LastMarkedSegment() == 11
		}, eventuallyWait, testFindInterval, "expected the tracker to retry marking segment 11")
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
		require.Equal(t, 4, findMarkableSegment(data, time.Minute))
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
		require.Equal(t, 4, findMarkableSegment(data, time.Minute))
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
		require.Equal(t, 2, findMarkableSegment(data, time.Minute))
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
		require.Equal(t, -1, findMarkableSegment(data, time.Minute))
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
		require.Equal(t, 11, findMarkableSegment(data, time.Minute))
		require.Len(t, data, 0)
	})
}
