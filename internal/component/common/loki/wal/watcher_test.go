package wal

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/loki/ingester/wal"
	"github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/utils"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

type testWriteTo struct {
	ReadEntries         *utils.SyncSlice[loki.Entry]
	series              map[uint64]model.LabelSet
	logger              log.Logger
	ReceivedSeriesReset []int
}

func (t *testWriteTo) StoreSeries(series []record.RefSeries, _ int) {
	for _, seriesRec := range series {
		t.series[uint64(seriesRec.Ref)] = util.MapToModelLabelSet(seriesRec.Labels.Map())
	}
}

func (t *testWriteTo) SeriesReset(segmentNum int) {
	level.Debug(t.logger).Log("msg", fmt.Sprintf("received series reset with %d", segmentNum))
	t.ReceivedSeriesReset = append(t.ReceivedSeriesReset, segmentNum)
}

func (t *testWriteTo) AppendEntries(entries wal.RefEntries, _ int) error {
	var entry loki.Entry
	if l, ok := t.series[uint64(entries.Ref)]; ok {
		entry.Labels = l
		for _, e := range entries.Entries {
			entry.Entry = e
			t.ReadEntries.Append(entry)
		}
	} else {
		level.Debug(t.logger).Log("series for entry not found")
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

type noMarker struct{}

func (n noMarker) LastMarkedSegment() int {
	return -1
}

// TestWatcher is the main test function, that works as framework to test different scenarios of the Watcher. It bootstraps
// necessary test components.
func TestWatcher(t *testing.T) {
	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			// start test global resources
			reg := prometheus.NewRegistry()
			logger := level.NewFilter(log.NewLogfmtLogger(os.Stdout), level.AllowDebug())
			dir := t.TempDir()
			metrics := NewWatcherMetrics(reg)
			writeTo := &testWriteTo{
				series:      map[uint64]model.LabelSet{},
				logger:      logger,
				ReadEntries: utils.NewSyncSlice[loki.Entry](),
			}
			// create new watcher, and defer stop
			watcher := NewWatcher(dir, "test", metrics, writeTo, logger, DefaultWatchConfig, noMarker{})
			defer watcher.Stop()
			wl, err := New(Config{
				Enabled: true,
				Dir:     dir,
			}, logger, reg)
			require.NoError(t, err)
			defer wl.Close()
			ew := newEntryWriter()
			// run test case injecting resources
			testCase(
				t,
				&watcherTestResources{
					writeEntry: func(entry loki.Entry) {
						_ = ew.WriteEntry(entry, wl, logger)
					},
					notifyWrite: func() {
						watcher.NotifyWrite()
					},
					startWatcher: func() {
						watcher.Start()
					},
					syncWAL: func() error {
						return wl.Sync()
					},
					nextWALSegment: func() error {
						_, err := wl.NextSegment()
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

type mockMarker struct {
	LastMarkedSegmentFunc func() int
}

func (m mockMarker) LastMarkedSegment() int {
	return m.LastMarkedSegmentFunc()
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

	t.Run("replay from marked segment if marker is not invalid", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		logger := level.NewFilter(log.NewLogfmtLogger(os.Stdout), level.AllowDebug())
		dir := t.TempDir()
		metrics := NewWatcherMetrics(reg)
		writeTo := &testWriteTo{
			series:      map[uint64]model.LabelSet{},
			logger:      logger,
			ReadEntries: utils.NewSyncSlice[loki.Entry](),
		}
		// create new watcher, and defer stop
		watcher := NewWatcher(dir, "test", metrics, writeTo, logger, DefaultWatchConfig, mockMarker{
			LastMarkedSegmentFunc: func() int {
				// when starting watcher, read from segment 0
				return 0
			},
		})
		defer watcher.Stop()
		wl, err := New(Config{
			Enabled: true,
			Dir:     dir,
		}, logger, reg)
		require.NoError(t, err)
		defer wl.Close()

		ew := newEntryWriter()

		// First, write to segment 0. This will be the last "marked" segment
		err = ew.WriteEntry(loki.Entry{
			Labels: labels,
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      "this line should appear in received entries",
			},
		}, wl, logger)
		require.NoError(t, err)

		// cut segment and sync
		_, err = wl.NextSegment()
		require.NoError(t, err)

		// Now, write to segment 1, this will be a segment not marked, hence replayed
		for _, line := range segment1Lines {
			err = ew.WriteEntry(loki.Entry{
				Labels: labels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			}, wl, logger)
			require.NoError(t, err)
		}

		// cut segment and sync
		_, err = wl.NextSegment()
		require.NoError(t, err)

		// Finally, write some data to the last segment, this will be the write head
		for _, line := range segment2Lines {
			err = ew.WriteEntry(loki.Entry{
				Labels: labels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			}, wl, logger)
			require.NoError(t, err)
		}

		// sync wal, and start watcher
		require.NoError(t, wl.Sync())

		// start watcher
		watcher.Start()

		require.Eventually(t, func() bool {
			return writeTo.ReadEntries.Length() == 6 // wait for watcher to catch up with both segments
		}, time.Second*10, time.Second, "timed out waiting for watcher to catch up")
		writeTo.AssertContainsLines(t, segment1Lines...)
		writeTo.AssertContainsLines(t, segment2Lines...)
	})

	t.Run("do not replay at all if invalid marker", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		logger := level.NewFilter(log.NewLogfmtLogger(os.Stdout), level.AllowDebug())
		dir := t.TempDir()
		metrics := NewWatcherMetrics(reg)
		writeTo := &testWriteTo{
			series:      map[uint64]model.LabelSet{},
			logger:      logger,
			ReadEntries: utils.NewSyncSlice[loki.Entry](),
		}
		// create new watcher, and defer stop
		watcher := NewWatcher(dir, "test", metrics, writeTo, logger, DefaultWatchConfig, mockMarker{
			LastMarkedSegmentFunc: func() int {
				// when starting watcher, read from segment 0
				return -1
			},
		})
		defer watcher.Stop()
		wl, err := New(Config{
			Enabled: true,
			Dir:     dir,
		}, logger, reg)
		require.NoError(t, err)
		defer wl.Close()

		ew := newEntryWriter()

		// First, write to segment 0. This will be the last "marked" segment
		err = ew.WriteEntry(loki.Entry{
			Labels: labels,
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      "this line should appear in received entries",
			},
		}, wl, logger)
		require.NoError(t, err)

		// cut segment and sync
		_, err = wl.NextSegment()
		require.NoError(t, err)

		// Now, write to segment 1, this will be a segment not marked, hence replayed
		for _, line := range segment1Lines {
			err = ew.WriteEntry(loki.Entry{
				Labels: labels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			}, wl, logger)
			require.NoError(t, err)
		}

		// cut segment and sync
		_, err = wl.NextSegment()
		require.NoError(t, err)

		// sync wal, and start watcher
		require.NoError(t, wl.Sync())

		// start watcher
		watcher.Start()

		// Write something after watcher started
		for _, line := range segment2Lines {
			err = ew.WriteEntry(loki.Entry{
				Labels: labels,
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      line,
				},
			}, wl, logger)
			require.NoError(t, err)
		}

		// sync wal, and start watcher
		require.NoError(t, wl.Sync())

		require.Eventually(t, func() bool {
			return writeTo.ReadEntries.Length() == 3 // wait for watcher to catch up with both segments
		}, time.Second*10, time.Second, "timed out waiting for watcher to catch up")
		writeTo.AssertContainsLines(t, segment2Lines...)
	})
}

// slowWriteTo mimics the combination of a WriteTo and a slow remote write client. This will allow us to have a writer
// that moves faster than the WAL watcher, and therefore, test the draining procedure.
type slowWriteTo struct {
	t                       *testing.T
	entriesReceived         atomic.Uint64
	sleepAfterAppendEntries time.Duration
}

func (s *slowWriteTo) SeriesReset(segmentNum int) {
}

func (s *slowWriteTo) StoreSeries(series []record.RefSeries, segmentNum int) {
}

func (s *slowWriteTo) AppendEntries(entries wal.RefEntries, segmentNum int) error {
	// only log on development debug flag
	if debug {
		var allLines strings.Builder
		for _, e := range entries.Entries {
			allLines.WriteString(e.Line)
			allLines.WriteString("/")
		}
		s.t.Logf("AppendEntries called from segment %d - %s", segmentNum, allLines.String())
	}

	s.entriesReceived.Add(uint64(len(entries.Entries)))
	time.Sleep(s.sleepAfterAppendEntries)
	return nil
}

func TestWatcher_StopAndDrainWAL(t *testing.T) {
	labels := model.LabelSet{
		"app": "test",
	}
	logger := level.NewFilter(log.NewLogfmtLogger(os.Stdout), level.AllowDebug())

	// newTestingResources is a helper for bootstrapping all required testing resources
	newTestingResources := func(t *testing.T, cfg WatchConfig) (*slowWriteTo, *Watcher, WAL) {
		reg := prometheus.NewRegistry()
		dir := t.TempDir()
		metrics := NewWatcherMetrics(reg)

		// the slow write to will take one second on each AppendEntries operation
		writeTo := &slowWriteTo{
			t:                       t,
			sleepAfterAppendEntries: time.Second,
		}

		watcher := NewWatcher(dir, "test", metrics, writeTo, logger, cfg, mockMarker{
			LastMarkedSegmentFunc: func() int {
				// Ignore marker to read from last segment, which is none
				return -1
			},
		})

		// start watcher, and burn through WAL as we write to it
		watcher.Start()

		wl, err := New(Config{
			Enabled: true,
			Dir:     dir,
		}, logger, reg)
		require.NoError(t, err)
		return writeTo, watcher, wl
	}

	t.Run("watcher drains WAL just in time", func(t *testing.T) {
		cfg := DefaultWatchConfig
		// considering the slow write to has a 1 second delay when Appending an entry, and before the draining begins,
		// the watcher would have consumed only 5 entries, this timeout will give the Watcher just enough time to fully
		// drain the WAL.
		cfg.DrainTimeout = time.Second * 16
		writeTo, watcher, wl := newTestingResources(t, cfg)
		defer wl.Close()

		ew := newEntryWriter()

		// helper to add context to each written line
		var lineCounter atomic.Int64
		writeNLines := func(t *testing.T, n int) {
			for i := 0; i < n; i++ {
				// First, write to segment 0. This will be the last "marked" segment
				err := ew.WriteEntry(loki.Entry{
					Labels: labels,
					Entry: push.Entry{
						Timestamp: time.Now(),
						Line:      fmt.Sprintf("test line %d", lineCounter.Load()),
					},
				}, wl, logger)
				lineCounter.Add(1)
				require.NoError(t, err)
			}
		}

		// The test will write the WAL while the Watcher is running. First, 10 lines will be written to a segment, and the test
		// will wait for the Watcher to have read 5 lines. After, a new segment will be cut, 10 other lines written, and the
		// Watcher stopped with drain. The test will expect all 20 lines in total to have been received.

		writeNLines(t, 10)

		require.Eventually(t, func() bool {
			return writeTo.entriesReceived.Load() >= 5
		}, time.Second*11, time.Millisecond*500, "expected the write to catch up to half of the first segment")

		_, err := wl.NextSegment()
		require.NoError(t, err)
		writeNLines(t, 10)
		require.NoError(t, wl.Sync())

		// Upon calling Stop drain, the Watcher should finish burning through segment 0, and also consume segment 1
		now := time.Now()
		watcher.Drain()
		watcher.Stop()

		// expecting 15s (missing 15 entries * 1 sec delay in AppendEntries) +/- 2.0s (taking into account the drain timeout
		// has one extra second.
		require.InDelta(t, time.Second*15, time.Since(now), float64(time.Millisecond*2000), "expected the drain procedure to take around 15s")
		require.Equal(t, int(writeTo.entriesReceived.Load()), 20, "expected the watcher to fully drain the WAL")
	})

	t.Run("watcher should exit promptly after draining completely", func(t *testing.T) {
		cfg := DefaultWatchConfig
		// the drain timeout will be too long, for the amount of data remaining in the WAL (~15 entries more)
		cfg.DrainTimeout = time.Second * 30
		writeTo, watcher, wl := newTestingResources(t, cfg)
		defer wl.Close()

		ew := newEntryWriter()

		// helper to add context to each written line
		var lineCounter atomic.Int64
		writeNLines := func(t *testing.T, n int) {
			for i := 0; i < n; i++ {
				// First, write to segment 0. This will be the last "marked" segment
				err := ew.WriteEntry(loki.Entry{
					Labels: labels,
					Entry: push.Entry{
						Timestamp: time.Now(),
						Line:      fmt.Sprintf("test line %d", lineCounter.Load()),
					},
				}, wl, logger)
				lineCounter.Add(1)
				require.NoError(t, err)
			}
		}

		// The test will write the WAL while the Watcher is running. First, 10 lines will be written to a segment, and the test
		// will wait for the Watcher to have read 5 lines. After, a new segment will be cut, 10 other lines written, and the
		// Watcher stopped with drain. The test will expect all 20 lines in total to have been received.

		writeNLines(t, 10)

		require.Eventually(t, func() bool {
			return writeTo.entriesReceived.Load() >= 5
		}, time.Second*11, time.Millisecond*500, "expected the write to catch up to half of the first segment")

		_, err := wl.NextSegment()
		require.NoError(t, err)
		writeNLines(t, 10)
		require.NoError(t, wl.Sync())

		// Upon calling Stop drain, the Watcher should finish burning through segment 0, and also consume segment 1
		now := time.Now()
		watcher.Drain()
		watcher.Stop()

		// expecting 15s (missing 15 entries * 1 sec delay in AppendEntries) +/- 2.0s (taking into account the drain timeout
		// has one extra second.
		require.InDelta(t, time.Second*15, time.Since(now), float64(time.Millisecond*2000), "expected the drain procedure to take around 15s")
		require.Equal(t, int(writeTo.entriesReceived.Load()), 20, "expected the watcher to fully drain the WAL")
	})

	t.Run("watcher drain timeout too short, should exit promptly", func(t *testing.T) {
		cfg := DefaultWatchConfig
		// having a 10 seconds timeout should give the watcher enough time to only consume ~10 entries, and be missing ~5
		// from the last segment
		cfg.DrainTimeout = time.Second * 10
		writeTo, watcher, wl := newTestingResources(t, cfg)
		defer wl.Close()

		ew := newEntryWriter()

		// helper to add context to each written line
		var lineCounter atomic.Int64
		writeNLines := func(t *testing.T, n int) {
			for i := 0; i < n; i++ {
				// First, write to segment 0. This will be the last "marked" segment
				err := ew.WriteEntry(loki.Entry{
					Labels: labels,
					Entry: push.Entry{
						Timestamp: time.Now(),
						Line:      fmt.Sprintf("test line %d", lineCounter.Load()),
					},
				}, wl, logger)
				lineCounter.Add(1)
				require.NoError(t, err)
			}
		}

		// The test will write the WAL while the Watcher is running. First, 10 lines will be written to a segment, and the test
		// will wait for the Watcher to have read 5 lines. After, a new segment will be cut, 10 other lines written, and the
		// Watcher stopped with drain. The test will expect all 20 lines in total to have been received.

		writeNLines(t, 10)

		require.Eventually(t, func() bool {
			return writeTo.entriesReceived.Load() >= 5
		}, time.Second*11, time.Millisecond*500, "expected the write to catch up to half of the first segment")

		_, err := wl.NextSegment()
		require.NoError(t, err)
		writeNLines(t, 10)
		require.NoError(t, wl.Sync())

		// Upon calling Stop drain, the Watcher should finish burning through segment 0, and also consume segment 1
		now := time.Now()
		watcher.Drain()
		watcher.Stop()

		require.InDelta(t, time.Second*10, time.Since(now), float64(time.Millisecond*2000), "expected the drain procedure to take around 15s")
		require.Less(t, int(writeTo.entriesReceived.Load()), 20, "expected watcher to have not consumed WAL fully")
		require.InDelta(t, 15, int(writeTo.entriesReceived.Load()), 1.0, "expected Watcher to consume at most +/- 1 entry from the WAL")
	})
}
