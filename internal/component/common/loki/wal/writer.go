package wal

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/loki/util"
)

const (
	minimumCleanSegmentsEvery = time.Second
)

// CleanupEventSubscriber is an interface that objects that want to receive events from the wal Writer can implement. After
// they can subscribe to events by adding themselves as subscribers on the Writer with writer.SubscribeCleanup.
type CleanupEventSubscriber interface {
	WriteCleanup
}

// WriteEventSubscriber is an interface that objects that want to receive an event when Writer writes to the WAL can
// implement, and later subscribe to the Writer via writer.SubscribeWrite.
type WriteEventSubscriber interface {
	// NotifyWrite allows others to be notifier when Writer writes to the underlying WAL.
	NotifyWrite()
}

// Writer writes log entries to a write-ahead log. It is also responsible for the WAL's segments,
// removing those older than the configured maximum age and notifying subscribers when it does.
type Writer struct {
	logger *slog.Logger
	cfg    Config
	wg     sync.WaitGroup
	wal    WAL

	writeMut    sync.Mutex
	stopped     bool
	entryWriter *entryWriter

	cleanupSubscribersLock sync.RWMutex
	cleanupSubscribers     []CleanupEventSubscriber

	writeSubscribersLock sync.RWMutex
	writeSubscribers     []WriteEventSubscriber

	metrics *WriterMetrics

	done chan struct{}
}

// NewWriter creates a new Writer.
func NewWriter(logger *slog.Logger, metrics *WriterMetrics, wl WAL, cfg Config) *Writer {
	return &Writer{
		logger:      logger,
		entryWriter: newEntryWriter(),
		wg:          sync.WaitGroup{},
		cfg:         cfg,
		wal:         wl,
		done:        make(chan struct{}),
		metrics:     metrics,
	}
}

func (wrt *Writer) Start() {
	// WAL cleanup routine that cleans old segments
	wrt.wg.Go(func() {
		// By cleaning every 10th of the configured threshold for considering a segment old, we are allowing a maximum slip
		// of 10%. If the configured time is 1 hour, that'd be 6 minutes.
		triggerEvery := wrt.cfg.MaxSegmentAge / 10
		if triggerEvery < minimumCleanSegmentsEvery {
			triggerEvery = minimumCleanSegmentsEvery
		}
		trigger := time.NewTicker(triggerEvery)
		defer trigger.Stop()
		for {
			select {
			case <-trigger.C:
				wrt.logger.Debug("Running wal old segments cleanup")
				if err := wrt.cleanSegments(wrt.cfg.MaxSegmentAge); err != nil {
					wrt.logger.Error("Error cleaning old segments", "err", err)
				}
			case <-wrt.done:
				return
			}
		}
	})
}

func (wrt *Writer) WriteEntry(entry loki.Entry) error {
	wrt.writeMut.Lock()
	defer wrt.writeMut.Unlock()

	if wrt.stopped {
		return loki.ErrConsumerStopped
	}

	if err := wrt.entryWriter.writeEntry(entry, wrt.wal); err != nil {
		wrt.logger.Error("failed to write entry", "err", err)
		return err
	}

	// emit metric with latest written timestamp, to be able to track delay from writer to watcher
	wrt.metrics.lastWrittenTimestamp.WithLabelValues().Set(float64(entry.Timestamp.Unix()))

	wrt.writeSubscribersLock.RLock()
	for _, s := range wrt.writeSubscribers {
		s.NotifyWrite()
	}
	wrt.writeSubscribersLock.RUnlock()

	return nil
}

func (wrt *Writer) Stop() {
	wrt.writeMut.Lock()
	defer wrt.writeMut.Unlock()
	wrt.stopped = true

	// Stop cleaner routine and wait for it to stop.
	close(wrt.done)
	wrt.wg.Wait()
}

// cleanSegments will remove segments older than maxAge from the WAL directory. If there's just one segment, none will be
// deleted since it's likely there's active readers on it. In case there's multiple segments, each will be deleted if:
// - It's not the last (highest numbered) segment
// - It's last modified date is older than the max allowed age
func (wrt *Writer) cleanSegments(maxAge time.Duration) error {
	maxModifiedAt := time.Now().Add(-maxAge)
	walDir := wrt.wal.Dir()
	segments, err := listSegments(walDir)
	if err != nil {
		return fmt.Errorf("error reading segments in wal directory: %w", err)
	}
	// Only clean if there's more than one segment
	if len(segments) <= 1 {
		return nil
	}
	// find the most recent, or head segment to avoid cleaning it up
	lastSegment := -1
	maxReclaimed := -1
	for _, segment := range segments {
		if lastSegment < segment.number {
			lastSegment = segment.number
		}
	}
	for _, segment := range segments {
		if segment.lastModified.Before(maxModifiedAt) && segment.number != lastSegment {
			// segment is older than allowed age, cleaning up
			if err := os.Remove(filepath.Join(walDir, segment.name)); err != nil {
				wrt.logger.Error("Error old wal segment", "err", err, "segmentNum", segment.number)
			}
			wrt.logger.Debug("Deleted old wal segment", "segmentNum", segment.number)
			wrt.metrics.reclaimedOldSegmentsSpaceCounter.WithLabelValues().Add(float64(segment.size))
			// keep track of the largest segment number reclaimed
			if segment.number > maxReclaimed {
				maxReclaimed = segment.number
			}
		}
	}
	// if we reclaimed at least one segment, notify all subscribers
	if maxReclaimed != -1 {
		wrt.cleanupSubscribersLock.RLock()
		defer wrt.cleanupSubscribersLock.RUnlock()
		for _, subscriber := range wrt.cleanupSubscribers {
			subscriber.SeriesReset(maxReclaimed)
		}
		wrt.metrics.lastReclaimedSegment.WithLabelValues().Set(float64(maxReclaimed))
	}
	return nil
}

// SubscribeCleanup adds a new CleanupEventSubscriber that will receive cleanup events.
func (wrt *Writer) SubscribeCleanup(subscriber CleanupEventSubscriber) {
	wrt.cleanupSubscribersLock.Lock()
	defer wrt.cleanupSubscribersLock.Unlock()
	wrt.cleanupSubscribers = append(wrt.cleanupSubscribers, subscriber)
}

// SubscribeWrite adds a new WriteEventSubscriber that will receive write events.
func (wrt *Writer) SubscribeWrite(subscriber WriteEventSubscriber) {
	wrt.writeSubscribersLock.Lock()
	defer wrt.writeSubscribersLock.Unlock()
	wrt.writeSubscribers = append(wrt.writeSubscribers, subscriber)
}

// entryWriter writes loki.Entry to a WAL, keeping in memory a single Record object that's reused
// across every write.
type entryWriter struct {
	reusableWALRecord *Record
}

// newEntryWriter creates a new entryWriter.
func newEntryWriter() *entryWriter {
	return &entryWriter{
		reusableWALRecord: &Record{
			RefEntries: make([]RefEntries, 0, 1),
			Series:     make([]record.RefSeries, 0, 1),
		},
	}
}

func (ew *entryWriter) writeEntry(entry loki.Entry, wl WAL) error {
	defer ew.reusableWALRecord.Reset()

	var fp uint64
	lbs := labels.FromMap(util.ModelLabelSetToMap(entry.Labels))
	fp, _ = lbs.HashWithoutLabels(nil, []string(nil)...)
	ref := chunks.HeadSeriesRef(fp)

	ew.reusableWALRecord.Series = append(ew.reusableWALRecord.Series, record.RefSeries{
		Ref:    ref,
		Labels: lbs,
	})

	// Append the entry to an already existing stream (if any)
	ew.reusableWALRecord.RefEntries = append(ew.reusableWALRecord.RefEntries, RefEntries{
		Ref: ref,
		Entries: []push.Entry{
			entry.Entry,
		},
		Created: entry.Created(),
	})

	return wl.Log(ew.reusableWALRecord)
}

type segmentRef struct {
	name         string
	number       int
	size         int64
	lastModified time.Time
}

// listSegments list wal segments under the given directory, alongside with some file system information for each.
func listSegments(dir string) (refs []segmentRef, err error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	// the following will attempt to get segments info in a best effort manner, omitting file if error
	for _, f := range files {
		fn := f.Name()
		k, err := strconv.Atoi(fn)
		if err != nil {
			continue
		}
		fileInfo, err := f.Info()
		if err != nil {
			continue
		}
		refs = append(refs, segmentRef{
			name:         fn,
			number:       k,
			lastModified: fileInfo.ModTime(),
			size:         fileInfo.Size(),
		})
	}
	sort.Slice(refs, func(i, j int) bool {
		return refs[i].number < refs[j].number
	})
	for i := 0; i < len(refs)-1; i++ {
		if refs[i].number+1 != refs[i+1].number {
			return nil, fmt.Errorf("segments are not sequential")
		}
	}
	return refs, nil
}
