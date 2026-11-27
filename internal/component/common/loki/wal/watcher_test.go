package wal

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"log/slog"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/prometheus/prometheus/tsdb/wlog"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/loki/util"
	autil "github.com/grafana/alloy/internal/util"
)

type testWriteTo struct {
	ReadEntries         *util.SyncSlice[loki.Entry]
	series              map[uint64]model.LabelSet
	logger              *slog.Logger
	ReceivedSeriesReset []int
}

func (t *testWriteTo) StoreSeries(series []record.RefSeries, _ int) {
	for _, seriesRec := range series {
		t.series[uint64(seriesRec.Ref)] = util.MapToModelLabelSet(seriesRec.Labels.Map())
	}
}

func (t *testWriteTo) SeriesReset(segmentNum int) {
	t.logger.Debug("received series reset", "segment", segmentNum)
	t.ReceivedSeriesReset = append(t.ReceivedSeriesReset, segmentNum)
}

func (t *testWriteTo) AppendEntries(_ context.Context, entries RefEntries, _ int) error {
	var entry loki.Entry
	if l, ok := t.series[uint64(entries.Ref)]; ok {
		entry.Labels = l
		for _, e := range entries.Entries {
			entry.Entry = e
			t.ReadEntries.Append(entry)
		}
	} else {
		t.logger.Debug("series for entry not found")
	}
	return nil
}

func (t *testWriteTo) AssertContainsLines(tst *testing.T, lines ...string) {
	seen := map[string]bool{}
	for _, l := range lines {
		seen[l] = false
	}
	for _, e := range t.ReadEntries.StartIterate() {
		if _, ok := seen[e.Line]; ok {
			seen[e.Line] = true
		}
	}
	t.ReadEntries.DoneIterate()

	for line, wasSeen := range seen {
		require.True(tst, wasSeen, "expected to have received line: %s", line)
	}
}

// watcherTestResources contains all resources necessary to test an individual Watcher functionality
type watcherTestResources struct {
	writeEntry             func(entry loki.Entry)
	notifyWrite            func()
	startWatcher           func()
	syncWAL                func() error
	nextWALSegment         func() error
	writeTo                *testWriteTo
	notifySegmentReclaimed func(segmentNum int)
}

type watcherTest func(t *testing.T, res *watcherTestResources)

// cases defines the watcher test cases
var cases = map[string]watcherTest{
	"read entries from WAL": func(t *testing.T, res *watcherTestResources) {
		res.startWatcher()

		lines := []string{
			"holis",
			"holus",
			"chau",
		}
		testLabels := model.LabelSet{
			"test": "watcher_read",
		}

		for _, line := range lines {
			res.writeEntry(loki.Entry{
				Labels: testLabels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			})
		}
		require.NoError(t, res.syncWAL())

		// notify watcher that entries have been written
		res.notifyWrite()

		require.Eventually(t, func() bool {
			return res.writeTo.ReadEntries.Length() == 3
		}, time.Second*10, time.Second, "expected watcher to catch up with written entries")
		defer res.writeTo.ReadEntries.DoneIterate()
		for _, readEntry := range res.writeTo.ReadEntries.StartIterate() {
			require.Contains(t, lines, readEntry.Line, "not expected log line")
		}
	},

	"read entries from WAL, just using backup timer to trigger reads": func(t *testing.T, res *watcherTestResources) {
		res.startWatcher()

		lines := []string{
			"holis",
			"holus",
			"chau",
		}
		testLabels := model.LabelSet{
			"test": "watcher_read",
		}

		for _, line := range lines {
			res.writeEntry(loki.Entry{
				Labels: testLabels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			})
		}
		require.NoError(t, res.syncWAL())

		// do not notify, let the backup timer trigger the watcher reads

		require.Eventually(t, func() bool {
			return res.writeTo.ReadEntries.Length() == 3
		}, time.Second*10, time.Second, "expected watcher to catch up with written entries")
		defer res.writeTo.ReadEntries.DoneIterate()
		for _, readEntry := range res.writeTo.ReadEntries.StartIterate() {
			require.Contains(t, lines, readEntry.Line, "not expected log line")
		}
	},

	"continue reading entries in next segment after initial segment is closed": func(t *testing.T, res *watcherTestResources) {
		res.startWatcher()
		lines := []string{
			"holis",
			"holus",
			"chau",
		}
		linesAfter := []string{
			"holis2",
			"holus2",
			"chau2",
		}
		testLabels := model.LabelSet{
			"test": "watcher_read",
		}

		for _, line := range lines {
			res.writeEntry(loki.Entry{
				Labels: testLabels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			})
		}
		require.NoError(t, res.syncWAL())

		res.notifyWrite()

		require.Eventually(t, func() bool {
			return res.writeTo.ReadEntries.Length() == 3
		}, time.Second*10, time.Second, "expected watcher to catch up with written entries")
		for _, readEntry := range res.writeTo.ReadEntries.StartIterate() {
			require.Contains(t, lines, readEntry.Line, "not expected log line")
		}
		res.writeTo.ReadEntries.DoneIterate()

		err := res.nextWALSegment()
		require.NoError(t, err, "expected no error when moving to next wal segment")

		for _, line := range linesAfter {
			res.writeEntry(loki.Entry{
				Labels: testLabels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			})
		}
		require.NoError(t, res.syncWAL())
		res.notifyWrite()

		require.Eventually(t, func() bool {
			return res.writeTo.ReadEntries.Length() == 6
		}, time.Second*10, time.Second, "expected watcher to catch up after new wal segment is cut")
		// assert over second half of entries
		defer res.writeTo.ReadEntries.DoneIterate()
		for _, readEntry := range res.writeTo.ReadEntries.StartIterate()[3:] {
			require.Contains(t, linesAfter, readEntry.Line, "not expected log line")
		}
	},

	"start reading from last segment": func(t *testing.T, res *watcherTestResources) {
		linesAfter := []string{
			"holis2",
			"holus2",
			"chau2",
		}
		testLabels := model.LabelSet{
			"test": "watcher_read",
		}

		// write something to first segment
		res.writeEntry(loki.Entry{
			Labels: testLabels,
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      "this shouldn't be read",
			},
		})

		require.NoError(t, res.syncWAL())

		err := res.nextWALSegment()
		require.NoError(t, err, "expected no error when moving to next wal segment")

		res.startWatcher()

		for _, line := range linesAfter {
			res.writeEntry(loki.Entry{
				Labels: testLabels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			})
		}
		require.NoError(t, res.syncWAL())

		res.notifyWrite()

		require.Eventually(t, func() bool {
			return res.writeTo.ReadEntries.Length() == 3
		}, time.Second*10, time.Second, "expected watcher to catch up after new wal segment is cut")
		// assert over second half of entries
		defer res.writeTo.ReadEntries.DoneIterate()
		for _, readEntry := range res.writeTo.ReadEntries.StartIterate()[3:] {
			require.Contains(t, linesAfter, readEntry.Line, "not expected log line")
		}
	},

	"watcher receives segments reclaimed notifications correctly": func(t *testing.T, res *watcherTestResources) {
		res.startWatcher()
		testLabels := model.LabelSet{
			"test": "watcher_read",
		}

		writeAndWaitForWatcherToCatchUp := func(line string, expectedReadEntries int) {
			res.writeEntry(loki.Entry{
				Labels: testLabels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			})
			require.NoError(t, res.syncWAL())
			res.notifyWrite()
			require.Eventually(t, func() bool {
				return res.writeTo.ReadEntries.Length() == expectedReadEntries
			}, time.Second*10, time.Second, "expected watcher to catch up with written entries")
		}

		// writing segment 0
		writeAndWaitForWatcherToCatchUp("segment 0", 1)

		// moving to segment 1
		require.NoError(t, res.nextWALSegment(), "expected no error when moving to next wal segment")

		// moving on to segment 1
		writeAndWaitForWatcherToCatchUp("segment 1", 2)

		// collecting segment 0
		res.notifySegmentReclaimed(0)
		require.Eventually(t, func() bool {
			return len(res.writeTo.ReceivedSeriesReset) == 1 && res.writeTo.ReceivedSeriesReset[0] == 0
		}, time.Second*10, time.Second, "timed out waiting to receive series reset")

		// moving and writing to segment 2
		require.NoError(t, res.nextWALSegment(), "expected no error when moving to next wal segment")
		writeAndWaitForWatcherToCatchUp("segment 2", 3)
		time.Sleep(time.Millisecond)
		// moving and writing to segment 3
		require.NoError(t, res.nextWALSegment(), "expected no error when moving to next wal segment")
		writeAndWaitForWatcherToCatchUp("segment 3", 4)

		// collecting all segments up to 2
		res.notifySegmentReclaimed(2)
		// Expect second SeriesReset call to have the highest numbered deleted segment, 2
		require.Eventually(t, func() bool {
			t.Logf("received series reset: %v", res.writeTo.ReceivedSeriesReset)
			return len(res.writeTo.ReceivedSeriesReset) == 2 && res.writeTo.ReceivedSeriesReset[1] == 2
		}, time.Second*10, time.Second, "timed out waiting to receive series reset")
	},
}

type noSavepoint struct{}

func (n noSavepoint) LastStoredSegment() int {
	return -1
}

// TestWatcher is the main test function, that works as framework to test different scenarios of the Watcher. It bootstraps
// necessary test components.
func TestWatcher(t *testing.T) {
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			// start test global resources
			reg := prometheus.NewRegistry()
			logger := autil.TestAlloyLogger(t).Slog()
			dir := t.TempDir()
			metrics := NewWatcherMetrics(reg)
			writeTo := &testWriteTo{
				series:      map[uint64]model.LabelSet{},
				logger:      logger,
				ReadEntries: util.NewSyncSlice[loki.Entry](),
			}
			// create new watcher, and defer stop
			watcher := NewWatcher(dir, "test", metrics, writeTo, logger, DefaultWatchConfig, noSavepoint{})
			defer watcher.Stop()

			wl, err := New(logger, reg, dir)
			require.NoError(t, err)
			defer wl.Close()

			writer := NewWriter(logger, NewWriterMetrics(reg), wl, Config{})
			require.NoError(t, err)
			defer writer.Stop()

			// run test case injecting resources
			testCase(
				t,
				&watcherTestResources{
					writeEntry: func(entry loki.Entry) {
						_ = writer.WriteEntry(entry)
					},
					notifyWrite: func() {
						watcher.NotifyWrite()
					},
					startWatcher: func() {
						watcher.Start()
					},
					syncWAL: func() error {
						return writer.wal.Sync()
					},
					nextWALSegment: func() error {
						_, err := writer.wal.NextSegment()
						return err
					},
					writeTo: writeTo,
					notifySegmentReclaimed: func(segmentNum int) {
						writeTo.SeriesReset(segmentNum)
					},
				},
			)
		})
	}
}

type mockSavepoint struct {
	LastStoredSegmentFunc func() int
}

func (m mockSavepoint) LastStoredSegment() int {
	return m.LastStoredSegmentFunc()
}

func TestWatcher_Replay(t *testing.T) {
	labels := model.LabelSet{
		"app": "test",
	}
	segment1Lines := []string{
		"before 1",
		"before 2",
		"before 3",
	}
	segment2Lines := []string{
		"after 1",
		"after 2",
		"after 3",
	}

	t.Run("replay from stored segment if savepoint is not invalid", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		logger := autil.TestAlloyLogger(t).Slog()
		dir := t.TempDir()
		metrics := NewWatcherMetrics(reg)
		writeTo := &testWriteTo{
			series:      map[uint64]model.LabelSet{},
			logger:      logger,
			ReadEntries: util.NewSyncSlice[loki.Entry](),
		}
		// create new watcher, and defer stop
		watcher := NewWatcher(dir, "test", metrics, writeTo, logger, DefaultWatchConfig, mockSavepoint{
			LastStoredSegmentFunc: func() int {
				// when starting watcher, read from segment 0
				return 0
			},
		})
		defer watcher.Stop()

		wl, err := New(logger, reg, dir)
		require.NoError(t, err)
		defer wl.Close()

		writer := NewWriter(logger, NewWriterMetrics(reg), wl, Config{})
		require.NoError(t, err)
		defer writer.Stop()

		// First, write to segment 0. This will be the last "marked" segment
		err = writer.WriteEntry(loki.Entry{
			Labels: labels,
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      "this line should appear in received entries",
			},
		})
		require.NoError(t, err)

		// cut segment and sync
		_, err = writer.wal.NextSegment()
		require.NoError(t, err)

		// Now, write to segment 1, this will be a segment not marked, hence replayed
		for _, line := range segment1Lines {
			err = writer.WriteEntry(loki.Entry{
				Labels: labels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			})
			require.NoError(t, err)
		}

		// cut segment and sync
		_, err = writer.wal.NextSegment()
		require.NoError(t, err)

		// Finally, write some data to the last segment, this will be the write head
		for _, line := range segment2Lines {
			err = writer.WriteEntry(loki.Entry{
				Labels: labels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			})
			require.NoError(t, err)
		}

		// sync wal, and start watcher
		require.NoError(t, writer.wal.Sync())

		// start watcher
		watcher.Start()

		require.Eventually(t, func() bool {
			return writeTo.ReadEntries.Length() == 6 // wait for watcher to catch up with both segments
		}, time.Second*10, time.Second, "timed out waiting for watcher to catch up")
		writeTo.AssertContainsLines(t, segment1Lines...)
		writeTo.AssertContainsLines(t, segment2Lines...)
	})

	t.Run("do not replay at all if invalid savepoint", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		logger := autil.TestAlloyLogger(t).Slog()
		dir := t.TempDir()
		metrics := NewWatcherMetrics(reg)
		writeTo := &testWriteTo{
			series:      map[uint64]model.LabelSet{},
			logger:      logger,
			ReadEntries: util.NewSyncSlice[loki.Entry](),
		}
		// create new watcher, and defer stop
		watcher := NewWatcher(dir, "test", metrics, writeTo, logger, DefaultWatchConfig, mockSavepoint{
			LastStoredSegmentFunc: func() int {
				// when starting watcher, read from segment 0
				return -1
			},
		})
		defer watcher.Stop()

		wl, err := New(logger, reg, dir)
		require.NoError(t, err)
		defer wl.Close()

		writer := NewWriter(logger, NewWriterMetrics(reg), wl, Config{})
		require.NoError(t, err)
		defer writer.Stop()

		// First, write to segment 0. This will be the last "marked" segment
		err = writer.WriteEntry(loki.Entry{
			Labels: labels,
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      "this line should appear in received entries",
			},
		})
		require.NoError(t, err)

		// cut segment and sync
		_, err = writer.wal.NextSegment()
		require.NoError(t, err)

		// Now, write to segment 1, this will be a segment not marked, hence replayed
		for _, line := range segment1Lines {
			err = writer.WriteEntry(loki.Entry{
				Labels: labels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			})
			require.NoError(t, err)
		}

		// cut segment and sync
		_, err = writer.wal.NextSegment()
		require.NoError(t, err)

		// sync wal, and start watcher
		require.NoError(t, writer.wal.Sync())

		// start watcher
		watcher.Start()

		// Write something after watcher started
		for _, line := range segment2Lines {
			err = writer.WriteEntry(loki.Entry{
				Labels: labels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			})
			require.NoError(t, err)
		}

		// sync wal, and start watcher
		require.NoError(t, writer.wal.Sync())

		require.Eventually(t, func() bool {
			return writeTo.ReadEntries.Length() == 3 // wait for watcher to catch up with both segments
		}, time.Second*10, time.Second, "timed out waiting for watcher to catch up")
		writeTo.AssertContainsLines(t, segment2Lines...)
	})
}

// slowWriteTo mimics the combination of a WriteTo and a slow remote write client. This will allow us to have a writer
// that moves faster than the WAL watcher, and therefore, test the draining procedure.
type slowWriteTo struct {
	entriesReceived         atomic.Uint64
	sleepAfterAppendEntries time.Duration

	// pauseAfter makes AppendEntries block once that many entries are received, until resume is closed. This pins how
	// much of the WAL the Watcher has consumed when the test starts draining.
	pauseAfter uint64
	resume     chan struct{}
}

func (s *slowWriteTo) SeriesReset(segmentNum int) {
}

func (s *slowWriteTo) StoreSeries(series []record.RefSeries, segmentNum int) {
}

func (s *slowWriteTo) AppendEntries(ctx context.Context, entries RefEntries, segmentNum int) error {
	n := uint64(len(entries.Entries))
	received := s.entriesReceived.Add(n)
	select {
	case <-time.After(s.sleepAfterAppendEntries):
	case <-ctx.Done():
		return ctx.Err()
	}
	if received-n < s.pauseAfter && received >= s.pauseAfter {
		select {
		case <-s.resume:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func TestWatcher_StopAndDrainWAL(t *testing.T) {
	labels := model.LabelSet{
		"app": "test",
	}
	logger := autil.TestAlloyLogger(t).Slog()

	// drain writes 10 lines to a segment and waits for the Watcher to consume 5 of them. Then it cuts a new segment,
	// writes 10 more lines and drains the Watcher, which has 15 entries left to consume, each taking one second.
	// It returns how many entries the Watcher consumed in total, and how long Drain and Stop took.
	drain := func(t *testing.T, drainTimeout time.Duration) (uint64, time.Duration) {
		reg := prometheus.NewRegistry()
		dir := t.TempDir()

		writeTo := &slowWriteTo{
			sleepAfterAppendEntries: time.Second,
			pauseAfter:              5,
			resume:                  make(chan struct{}),
		}

		wl, err := New(logger, reg, dir)
		require.NoError(t, err)
		defer wl.Close()
		writer := NewWriter(logger, NewWriterMetrics(reg), wl, Config{})
		defer writer.Stop()

		cfg := DefaultWatchConfig
		cfg.DrainTimeout = drainTimeout
		watcher := NewWatcher(dir, "test", NewWatcherMetrics(reg), writeTo, logger, cfg, mockSavepoint{
			LastStoredSegmentFunc: func() int {
				// Ignore savepoint to read from last segment, which is none
				return -1
			},
		})
		// Start the Watcher after the WAL has its first segment. Otherwise, the Watcher finds no segment and retries
		// after 5 seconds.
		watcher.Start()

		var lineCounter int
		writeNLines := func(n int) {
			for range n {
				err := writer.WriteEntry(loki.Entry{
					Labels: labels,
					Entry: push.Entry{
						Timestamp: time.Now(),
						Line:      fmt.Sprintf("test line %d", lineCounter),
					},
				})
				lineCounter++
				require.NoError(t, err)
			}
		}

		writeNLines(10)
		require.Eventually(t, func() bool {
			return writeTo.entriesReceived.Load() >= 5
		}, time.Second*11, time.Millisecond*100, "expected the write to catch up to half of the first segment")

		_, err = writer.wal.NextSegment()
		require.NoError(t, err)
		writeNLines(10)
		require.NoError(t, writer.wal.Sync())
		require.Equal(t, uint64(5), writeTo.entriesReceived.Load(), "expected the Watcher to be paused after 5 entries")

		// Upon calling Drain, the Watcher should finish burning through segment 0, and also consume segment 1
		start := time.Now()
		close(writeTo.resume)
		watcher.Drain()
		watcher.Stop()
		return writeTo.entriesReceived.Load(), time.Since(start)
	}

	t.Run("watcher drains WAL just in time", func(t *testing.T) {
		// The Watcher needs 15s to consume the 15 remaining entries, so this timeout leaves it just enough time.
		received, took := drain(t, time.Second*18)

		require.Equal(t, uint64(20), received, "expected the watcher to fully drain the WAL")
		require.GreaterOrEqual(t, took, time.Second*15, "expected the drain procedure to wait for the remaining entries")
	})

	t.Run("watcher should exit promptly after draining completely", func(t *testing.T) {
		// The drain timeout is much longer than the 15s needed to consume the remaining entries.
		received, took := drain(t, time.Second*30)

		require.Equal(t, uint64(20), received, "expected the watcher to fully drain the WAL")
		require.Less(t, took, time.Second*20, "expected the drain procedure to exit well before the timeout")
	})

	t.Run("watcher drain timeout too short, should exit promptly", func(t *testing.T) {
		// In 10 seconds the Watcher consumes ~10 of the 15 remaining entries.
		received, took := drain(t, time.Second*10)

		require.InDelta(t, time.Second*10, took, float64(time.Second*2), "expected the drain procedure to take around 10s")
		require.InDelta(t, 15, int(received), 1.0, "expected Watcher to consume at most +/- 1 entry from the WAL")
	})
}

// Regression test: reading a record that spans a page boundary must not panic.
func TestLiveReaderRecordSpanningPage(t *testing.T) {
	const recFull = byte(1)

	payload := make([]byte, 10)
	crc := crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli))

	var page bytes.Buffer
	page.WriteByte(recFull)
	binary.Write(&page, binary.BigEndian, uint16(len(payload)))
	binary.Write(&page, binary.BigEndian, crc)
	page.Write(payload)
	page.WriteByte(recFull)
	binary.Write(&page, binary.BigEndian, uint16(32750))
	binary.Write(&page, binary.BigEndian, uint32(0))

	logger := autil.TestAlloyLogger(t).Slog()
	reader := wlog.NewLiveReader(logger, wlog.NewLiveReaderMetrics(nil), bytes.NewReader(page.Bytes()))

	require.NotPanics(t, func() {
		for reader.Next() {
		}
	})
}
