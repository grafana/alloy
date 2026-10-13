package wal

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/loki/util"
)

func TestWriter(t *testing.T) {
	t.Run("entries are written to wal", func(t *testing.T) {
		var (
			dir    = t.TempDir()
			logger = slog.New(slog.DiscardHandler)
			reg    = prometheus.NewRegistry()
			cfg    = Config{
				MaxSegmentAge: time.Minute,
			}
		)

		wl, err := New(logger, reg, dir)
		require.NoError(t, err)
		defer wl.Close()

		writer := NewWriter(logger, NewWriterMetrics(reg), wl, cfg)
		defer writer.Stop()

		// write entries to wal and sync
		writer.Start()

		lset := model.LabelSet{"testing": "log"}
		entries := []loki.Entry{
			loki.NewEntry(lset, push.Entry{Line: "some line", Timestamp: time.Now()}),
			loki.NewEntry(lset, push.Entry{Line: "some other line", Timestamp: time.Now().Add(1 * time.Second)}),
			loki.NewEntry(lset, push.Entry{Line: "some other other line", Timestamp: time.Now().Add(2 * time.Second)}),
		}

		for _, e := range entries {
			require.NoError(t, writer.WriteEntry(e))
		}

		// accessing the WAL inside, just for testing!
		require.NoError(t, writer.wal.Sync(), "failed to sync wal")

		// assert over WAL entries
		refSeries, refEntries := eventuallyReadWAL(t, len(entries), dir)
		assertEntries(t, refSeries, refEntries, entries)
	})

	t.Run("stopped writer returns error", func(t *testing.T) {
		var (
			dir    = t.TempDir()
			reg    = prometheus.NewRegistry()
			logger = slog.New(slog.DiscardHandler)
			cfg    = Config{
				MaxSegmentAge: time.Minute,
			}
		)

		wl, err := New(logger, reg, dir)
		require.NoError(t, err)
		defer wl.Close()

		writer := NewWriter(logger, NewWriterMetrics(reg), wl, cfg)
		writer.Start()

		require.NoError(t, writer.WriteEntry(loki.Entry{
			Labels: model.LabelSet{"key": "value"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      "lin",
			},
		}))

		writer.Stop()

		require.ErrorIs(t, writer.WriteEntry(loki.Entry{
			Labels: model.LabelSet{"key": "value"},
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      "lin",
			},
		}), loki.ErrConsumerStopped)
	})
}

func TestWriter_WriteBatch(t *testing.T) {
	t.Run("batch are written to wal", func(t *testing.T) {
		var (
			now    = time.Now()
			dir    = t.TempDir()
			reg    = prometheus.NewRegistry()
			logger = slog.New(slog.DiscardHandler)
		)

		wl, err := New(logger, reg, dir)
		require.NoError(t, err)
		defer wl.Close()

		writer := NewWriter(logger, NewWriterMetrics(reg), wl, Config{
			MaxSegmentAge: time.Minute,
		})
		writer.Start()
		defer writer.Stop()

		var (
			streams []loki.Stream
			batch   = loki.NewBatch()
		)

		addStream := func(id string) {
			stream := loki.NewStream(
				model.LabelSet{"stream": model.LabelValue(id)},
				push.Entry{Line: "line 1", Timestamp: now},
				push.Entry{Line: "line 2", Timestamp: now.Add(1 * time.Millisecond)},
				push.Entry{Line: "line 3", Timestamp: now.Add(2 * time.Millisecond)},
				push.Entry{Line: "line 4", Timestamp: now.Add(3 * time.Millisecond)},
			)
			batch.Add(stream)
			streams = append(streams, stream)
		}

		addStream("1")
		addStream("2")
		addStream("3")

		require.NoError(t, writer.WriteBatch(batch))
		require.NoError(t, writer.wal.Sync(), "failed to sync wal")
		refSeries, refEntries := eventuallyReadWAL(t, batch.EntryLen(), dir)
		assertStreams(t, refSeries, refEntries, streams)
	})

	t.Run("stopped writer returns error", func(t *testing.T) {
		var (
			dir    = t.TempDir()
			reg    = prometheus.NewRegistry()
			logger = slog.New(slog.DiscardHandler)
		)

		wl, err := New(logger, reg, dir)
		require.NoError(t, err)

		writer := NewWriter(logger, NewWriterMetrics(reg), wl, Config{
			MaxSegmentAge: time.Minute,
		})
		writer.Start()

		batch := loki.NewBatch()
		batch.Add(loki.NewStream(model.LabelSet{"stream": "1"}, push.Entry{Timestamp: time.Now(), Line: "1"}))

		writer.Stop()
		require.ErrorIs(t, writer.WriteBatch(batch), loki.ErrConsumerStopped)
	})
}

func TestWriter_MetricsWorkAfterRecreation(t *testing.T) {
	var (
		dir    = t.TempDir()
		logger = slog.New(slog.DiscardHandler)
		reg    = prometheus.NewRegistry()
		cfg    = Config{
			MaxSegmentAge: time.Minute,
		}
	)

	wl, err := New(logger, reg, dir)
	require.NoError(t, err)
	defer wl.Close()

	writer := NewWriter(logger, NewWriterMetrics(reg), wl, cfg)
	writer.Start()

	entry := loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now(), Line: "line"})
	writer.WriteEntry(entry)

	expected := fmt.Sprintf(`
	# HELP loki_write_wal_writer_last_written_timestamp Latest timestamp that was written to the WAL
	# TYPE loki_write_wal_writer_last_written_timestamp gauge
	loki_write_wal_writer_last_written_timestamp %d
	`, entry.Timestamp.Unix())

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.NoError(c, testutil.GatherAndCompare(reg, strings.NewReader(expected), "loki_write_wal_writer_last_written_timestamp"))
	}, 2*time.Second, 100*time.Millisecond)

	writer.Stop()

	// Reuse the same WAL, the writer no longer owns it.
	writer = NewWriter(logger, NewWriterMetrics(reg), wl, cfg)
	writer.Start()
	defer writer.Stop()

	newEntry := loki.NewEntry(model.LabelSet{"foo": "bar"}, push.Entry{Timestamp: time.Now().Add(1 * time.Second), Line: "line"})
	writer.WriteEntry(newEntry)

	expected = fmt.Sprintf(`
	# HELP loki_write_wal_writer_last_written_timestamp Latest timestamp that was written to the WAL
	# TYPE loki_write_wal_writer_last_written_timestamp gauge
	loki_write_wal_writer_last_written_timestamp %d
	`, newEntry.Timestamp.Unix())

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		require.NoError(c, testutil.GatherAndCompare(reg, strings.NewReader(expected), "loki_write_wal_writer_last_written_timestamp"))
	}, 2*time.Second, 100*time.Millisecond)
}

type notifySegmentsCleanedFunc func(num int)

func (n notifySegmentsCleanedFunc) NotifyWrite() {
}

func (n notifySegmentsCleanedFunc) SeriesReset(segmentNum int) {
	n(segmentNum)
}

func TestWriter_OldSegmentsAreCleanedUp(t *testing.T) {
	var (
		dir           = t.TempDir()
		logger        = slog.New(slog.DiscardHandler)
		reg           = prometheus.NewRegistry()
		maxSegmentAge = time.Second * 2
		cfg           = Config{
			MaxSegmentAge: maxSegmentAge,
		}
		subscriber1 = []int{}
		subscriber2 = []int{}
	)

	wl, err := New(logger, reg, dir)
	require.NoError(t, err)
	defer wl.Close()

	writer := NewWriter(logger, NewWriterMetrics(reg), wl, cfg)
	defer writer.Stop()
	writer.Start()

	notificationMutex := sync.Mutex{}
	// add writer events subscriber. Add multiple to test fanout
	writer.SubscribeCleanup(notifySegmentsCleanedFunc(func(num int) {
		notificationMutex.Lock()
		defer notificationMutex.Unlock()
		subscriber1 = append(subscriber1, num)
	}))
	writer.SubscribeCleanup(notifySegmentsCleanedFunc(func(num int) {
		notificationMutex.Lock()
		defer notificationMutex.Unlock()
		subscriber2 = append(subscriber2, num)
	}))

	lset := model.LabelSet{"testing": "log"}

	entries := []loki.Entry{
		loki.NewEntry(lset, push.Entry{Line: "some line", Timestamp: time.Now()}),
		loki.NewEntry(lset, push.Entry{Line: "some other line", Timestamp: time.Now().Add(1 * time.Second)}),
		loki.NewEntry(lset, push.Entry{Line: "some other other line", Timestamp: time.Now().Add(2 * time.Second)}),
	}

	for _, e := range entries {
		require.NoError(t, writer.WriteEntry(e))
	}

	// accessing the WAL inside, just for testing!
	require.NoError(t, writer.wal.Sync(), "failed to sync wal")

	// assert over WAL entries
	refSeries, refEntries := eventuallyReadWAL(t, len(entries), dir)
	assertEntries(t, refSeries, refEntries, entries)

	// check segment is there
	fileInfo, err := os.Stat(filepath.Join(dir, "00000000"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, fileInfo.Size(), int64(0), "first segment size should be >= 0")

	// force close segment, so that one is eventually cleaned up
	_, err = writer.wal.NextSegment()
	require.NoError(t, err, "error closing current segment")

	// wait for segment to be cleaned
	time.Sleep(maxSegmentAge * 2)

	_, err = os.Stat(filepath.Join(dir, "00000000"))
	require.Error(t, err)
	require.ErrorIs(t, err, os.ErrNotExist, "expected file not exists error")

	notificationMutex.Lock()
	// assert all subscribers were notified
	require.Len(t, subscriber1, 1, "expected one segment reclaimed notification in subscriber1")
	require.Equal(t, 0, subscriber1[0])

	require.Len(t, subscriber2, 1, "expected one segment reclaimed notification in subscriber2")
	require.Equal(t, 0, subscriber2[0])
	notificationMutex.Unlock()

	// Expect last, or "head" segment to still be alive
	_, err = os.Stat(filepath.Join(dir, "00000001"))
	require.NoError(t, err)
}

func TestWriter_NoSegmentIsCleanedUpIfTheresOnlyOne(t *testing.T) {
	var (
		dir           = t.TempDir()
		logger        = slog.New(slog.DiscardHandler)
		reg           = prometheus.NewRegistry()
		maxSegmentAge = 2 * time.Second
		cfg           = Config{
			MaxSegmentAge: maxSegmentAge,
		}
		segmentsReclaimedNotificationsReceived = []int{}
	)

	wl, err := New(logger, reg, dir)
	require.NoError(t, err)
	defer wl.Close()

	writer := NewWriter(logger, NewWriterMetrics(reg), wl, cfg)
	writer.Start()
	defer writer.Stop()

	// add writer events subscriber
	writer.SubscribeCleanup(notifySegmentsCleanedFunc(func(num int) {
		segmentsReclaimedNotificationsReceived = append(segmentsReclaimedNotificationsReceived, num)
	}))

	lset := model.LabelSet{"testing": "log"}

	entries := []loki.Entry{
		loki.NewEntry(lset, push.Entry{Line: "some line", Timestamp: time.Now()}),
		loki.NewEntry(lset, push.Entry{Line: "some line 2", Timestamp: time.Now().Add(1 * time.Second)}),
	}

	// write entries to wal and sync
	for _, e := range entries {
		require.NoError(t, writer.WriteEntry(e))
	}

	// accessing the WAL inside, just for testing!
	require.NoError(t, writer.wal.Sync(), "failed to sync wal")

	// assert over WAL entries
	refSeries, refEntries := eventuallyReadWAL(t, len(entries), dir)
	assertEntries(t, refSeries, refEntries, entries)

	// check segment is there
	fileInfo, err := os.Stat(filepath.Join(dir, "00000000"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, fileInfo.Size(), int64(0), "first segment size should be >= 0")

	// wait for segment to be cleaned
	time.Sleep(maxSegmentAge * 2)

	_, err = os.Stat(filepath.Join(dir, "00000000"))
	require.NoError(t, err)
	require.Len(t, segmentsReclaimedNotificationsReceived, 0, "expected no notification")
}

func assertEntries(t *testing.T, refSeries []record.RefSeries, refEntries []RefEntries, expected []loki.Entry) {
	t.Helper()

	var entries []loki.Entry
	for _, group := range refEntries {
		si := slices.IndexFunc(refSeries, func(s record.RefSeries) bool {
			return s.Ref == group.Ref
		})
		require.GreaterOrEqual(t, si, 0)

		lset := util.MapToModelLabelSet(refSeries[si].Labels.Map())
		for i := range group.Entries {
			entries = append(entries, group.EntryAt(lset, i))
		}
	}
	require.Lenf(t, entries, len(expected), "expected: %v\nread from wal: %v", expected, entries)

	for _, e := range expected {
		i := slices.IndexFunc(entries, func(entry loki.Entry) bool {
			return entriesEqual(e, entry)
		})
		require.GreaterOrEqualf(t, i, 0, "entry not found in wal: %v\nread from wal: %v", e, entries)
		entries = slices.Delete(entries, i, i+1)
	}
}

func entriesEqual(a, b loki.Entry) bool {
	return a.Labels.Equal(b.Labels) &&
		a.Created() == b.Created() &&
		pushEntriesEqual(a.Entry, b.Entry)
}

func pushEntriesEqual(a, b push.Entry) bool {
	return a.Line == b.Line &&
		a.Timestamp.Equal(b.Timestamp) &&
		slices.Equal(a.StructuredMetadata, b.StructuredMetadata)
}

func assertStreams(t *testing.T, refSeries []record.RefSeries, refEntries []RefEntries, expected []loki.Stream) {
	t.Helper()

	var streams []loki.Stream

	for _, entries := range refEntries {
		si := slices.IndexFunc(refSeries, func(s record.RefSeries) bool {
			return s.Ref == entries.Ref
		})
		require.GreaterOrEqual(t, si, 0)

		lset := util.MapToModelLabelSet(refSeries[si].Labels.Map())
		stream := entries.Stream(lset)

		i := slices.IndexFunc(streams, func(s loki.Stream) bool {
			return s.Labels.Equal(lset)
		})
		if i < 0 {
			streams = append(streams, stream)
			continue
		}
		streams[i].Entries = append(streams[i].Entries, stream.Entries...)
	}
	require.Lenf(t, streams, len(expected), "expected: %v\nread from wal: %v", expected, streams)

	for _, e := range expected {
		i := slices.IndexFunc(streams, func(s loki.Stream) bool {
			return streamsEqual(e, s)
		})
		require.GreaterOrEqualf(t, i, 0, "stream not found in wal: %v\nread from wal: %v", e, streams)
		streams = slices.Delete(streams, i, i+1)
	}
}

func streamsEqual(a, b loki.Stream) bool {
	if !a.Labels.Equal(b.Labels) || a.Created() != b.Created() || len(a.Entries) != len(b.Entries) {
		return false
	}
	for i := range a.Entries {
		if !pushEntriesEqual(a.Entries[i], b.Entries[i]) {
			return false
		}
	}
	return true
}

func eventuallyReadWAL(t *testing.T, expectedEntries int, dir string) ([]record.RefSeries, []RefEntries) {
	var (
		refSeries  []record.RefSeries
		refEntries []RefEntries
	)

	require.EventuallyWithT(t, func(c *assert.CollectT) {
		refSeries, refEntries = readWAL(c, dir)

		var count int
		for _, re := range refEntries {
			count += len(re.Entries)
		}
		require.Equal(c, count, expectedEntries)
	}, time.Second*5, time.Second, "timed out waiting for WAL")
	return refSeries, refEntries
}

func readWAL(t require.TestingT, dir string) ([]record.RefSeries, []RefEntries) {
	reader, closeFn, err := newWalReader(dir, -1)
	require.NoError(t, err)
	defer closeFn.Close()

	var (
		refSeries  []record.RefSeries
		refEntries []RefEntries
	)

	for reader.Next() {
		var walRec = Record{}
		bytes := reader.Record()
		require.NoError(t, DecodeRecord(bytes, &walRec))

		refSeries = append(refSeries, walRec.Series...)
		refEntries = append(refEntries, walRec.RefEntries...)
	}

	return refSeries, refEntries
}

func BenchmarkWriter_WriteEntries(b *testing.B) {
	type testCase struct {
		lines          int
		labelSetsCount int
	}
	var cases = []testCase{
		{
			lines:          1000,
			labelSetsCount: 1,
		},
		{
			lines:          1000,
			labelSetsCount: 4,
		},
		{
			lines:          1e6,
			labelSetsCount: 1,
		},
		{
			lines:          1e6,
			labelSetsCount: 100,
		},
		{
			lines:          1e7,
			labelSetsCount: 1,
		},
		{
			lines:          1e7,
			labelSetsCount: 1e3,
		},
	}
	for _, testCase := range cases {
		b.Run(fmt.Sprintf("%d lines, %d different label sets", testCase.lines, testCase.labelSetsCount), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				benchWriteEntries(b, testCase.lines, testCase.labelSetsCount)
			}
		})
	}
}

func benchWriteEntries(b *testing.B, lines, labelSetCount int) {
	var (
		dir    = b.TempDir()
		logger = slog.New(slog.DiscardHandler)
		reg    = prometheus.NewRegistry()
		cfg    = Config{
			MaxSegmentAge: time.Minute,
		}
	)

	wl, err := New(logger, reg, dir)
	require.NoError(b, err)
	defer wl.Close()

	writer := NewWriter(logger, NewWriterMetrics(reg), wl, cfg)
	writer.Start()
	defer writer.Stop()

	for i := 0; i < lines; i++ {
		require.NoError(b, writer.WriteEntry(
			loki.Entry{
				Labels: model.LabelSet{
					"someLabel": model.LabelValue(fmt.Sprint(i % labelSetCount)),
				},
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      fmt.Sprintf("some line being written %d", i),
				},
			},
		))
	}
}
