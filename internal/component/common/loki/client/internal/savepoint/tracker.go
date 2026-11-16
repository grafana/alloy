package savepoint

import (
	"log/slog"
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki/wal"
)

type Tracker interface {
	wal.Savepoint

	// Start loads the last marked segment from disk and begins the async processing of
	// receive/send dataUpdate updates.
	Start()

	// UpdateReceivedData sends an update event to the tracker, that informs that some dataUpdate, coming from a particular WAL
	// segment, has been read out of the WAL and enqueued for sending.
	UpdateReceivedData(segmentId, dataCount int)

	// UpdateSentData sends an update event to the tracker, informing that some dataUpdate, coming from a particular WAL
	// segment, has been delivered, or the sender has given up on it.
	UpdateSentData(segmentId, dataCount int) // Data which was sent or given up on sending

	// MarkWALDrained tells the tracker that nothing is written to the WAL anymore and that the watcher has read all of
	// it. It must be called before Stop. The segment that was last written to is then complete and Stop saves it as
	// consumed too, once its data is delivered, so that a restart does not replay it.
	MarkWALDrained()

	// Stop stops the tracker, and its async processing of receive/send dataUpdate updates.
	Stop()
}

// SegmentTracker implements Tracker, processing data update events in an asynchronous manner, and tracking the last
// consumed segment of a single endpoint in the savepoint file.
type SegmentTracker struct {
	dataIOUpdate     chan dataUpdate
	lastSavedSegment int
	logger           *slog.Logger
	file             *File
	key              string
	maxSegmentAge    time.Duration
	metrics          *Metrics
	quit             chan struct{}
	runFindTicker    *time.Ticker
	walDrained       atomic.Bool
	wg               sync.WaitGroup
}

// dataUpdate is an update event that some amount of data has been read out of the WAL and enqueued, delivered or dropped.
type dataUpdate struct {
	segmentId int
	dataCount int
	// received is true for data that was read out of the WAL, false for data that was delivered or given up on.
	received bool
}

var _ Tracker = (*SegmentTracker)(nil)

// NewSegmentTracker creates a new SegmentTracker, storing the progress of key in file.
func NewSegmentTracker(file *File, key string, maxSegmentAge time.Duration, logger *slog.Logger, metrics *Metrics) *SegmentTracker {
	return &SegmentTracker{
		lastSavedSegment: noSegment, // Segment ID last marked on disk.
		file:             file,
		key:              key,
		//TODO: What is a good size for the channel?
		dataIOUpdate:  make(chan dataUpdate, 100),
		quit:          make(chan struct{}),
		logger:        logger,
		metrics:       metrics.curryWithId(key),
		maxSegmentAge: maxSegmentAge,
		// runFindTicker will force the execution of the find markable segment routine every second
		runFindTicker: time.NewTicker(time.Second),
	}
}

func (t *SegmentTracker) Start() {
	// Load the last saved segment from disk (if it exists).
	if lastSegment := t.file.LastStoredSegment(t.key); lastSegment >= 0 {
		t.lastSavedSegment = lastSegment
	}

	t.wg.Go(t.runUpdatePendingData)
}

func (t *SegmentTracker) LastStoredSegment() int {
	return t.file.LastStoredSegment(t.key)
}

func (t *SegmentTracker) UpdateReceivedData(segmentId, dataCount int) {
	t.dataIOUpdate <- dataUpdate{
		segmentId: segmentId,
		dataCount: dataCount,
		received:  true,
	}
}

func (t *SegmentTracker) UpdateSentData(segmentId, dataCount int) {
	t.dataIOUpdate <- dataUpdate{
		segmentId: segmentId,
		dataCount: -1 * dataCount,
	}
}

func (t *SegmentTracker) MarkWALDrained() {
	t.walDrained.Store(true)
}

func (t *SegmentTracker) Stop() {
	t.runFindTicker.Stop()
	close(t.quit)
	t.wg.Wait()
}

// countDataItem tracks inside a map the count of in-flight log entries, and the last update received, for a given segment.
type countDataItem struct {
	count      int
	lastUpdate time.Time
}

// processDataItem is a version of countDataItem, with the segment number the information corresponds to included.
type processDataItem struct {
	segment    int
	count      int
	lastUpdate time.Time
}

// runUpdatePendingData is assumed to run in a separate routine, asynchronously keeping track of how much data each WAL
// segment the Watcher reads from, has left to send. When a segment reaches zero, it means that is has been consumed,
// and a procedure is triggered to find the "last consumed segment", implemented by FindMarkableSegment. Since this
// last procedure could be expensive, it's execution is run at most if a segment has reached count zero, of when a timer
// is fired (once per second).
//
// The highest segment that data has been read from is never marked, since it may be the segment the WAL is still
// writing to. Entries appended to a segment after it was marked would be skipped by a restart, which continues with
// the segment after the marked one. Once the WAL is drained nothing is written anymore and that segment is marked too.
func (t *SegmentTracker) runUpdatePendingData() {
	var (
		segmentDataCount = make(map[int]*countDataItem)
		// openSegment is the highest segment that data has been read from.
		openSegment = noSegment
	)

	// update applies an update and returns true if a segment reached a count of zero.
	update := func(u dataUpdate) bool {
		if u.received && u.segmentId > openSegment {
			openSegment = u.segmentId
		}

		di, ok := segmentDataCount[u.segmentId]
		if !ok {
			segmentDataCount[u.segmentId] = &countDataItem{
				count:      u.dataCount,
				lastUpdate: time.Now(),
			}
			return false
		}

		di.lastUpdate = time.Now()
		di.count += u.dataCount
		return di.count == 0
	}

	for {
		// shouldRunFind will be true if a markable segment should be found after the update, that is if one reached a count
		// of zero, or a ticker fired
		shouldRunFind := false
		select {
		case <-t.quit:
			t.finish(segmentDataCount, update)
			return
		case u := <-t.dataIOUpdate:
			// if a segment reached zero, run find routine because a segment might be ready to be marked
			shouldRunFind = update(u)
		}

		// if ticker fired, force run find
		select {
		case <-t.runFindTicker.C:
			shouldRunFind = true
		default:
		}

		if !shouldRunFind {
			continue
		}

		t.mark(findMarkableSegment(segmentDataCount, t.maxSegmentAge, openSegment))
	}
}

// finish saves the last segment once the WAL is drained. By then every update an endpoint reported while it stopped is
// waiting in the channel, so those are applied first.
func (t *SegmentTracker) finish(segmentDataCount map[int]*countDataItem, update func(dataUpdate) bool) {
	if !t.walDrained.Load() {
		return
	}

	for {
		select {
		case u := <-t.dataIOUpdate:
			update(u)
		default:
			t.mark(findMarkableSegment(segmentDataCount, t.maxSegmentAge, math.MaxInt))
			return
		}
	}
}

// mark saves segment as the last consumed one, unless it is not newer than the one that is already saved.
func (t *SegmentTracker) mark(segment int) {
	t.logger.Debug("found markable segment", "segment", segment)
	if segment > t.lastSavedSegment {
		t.file.StoreSegment(t.key, segment)
		t.lastSavedSegment = segment
		t.metrics.lastSavedSegment.WithLabelValues().Set(float64(segment))
	}
}

// findMarkableSegment finds, given the summary of data updates received, and a threshold on how much time can pass for
// a segment that hasn't received updates to be considered as "live", the segment that should be marked as last consumed.
// The algorithm will find the highest numbered segment that is considered as "consumed", with its all predecessors
// "consumed" as well.
//
// A consumed segment is one with data count of zero, meaning that there's no data left in flight for it, or it hasn't
// received any updates for tooOldThreshold time.
//
// openSegment and every segment after it are never consumed, since the WAL may still write to them.
//
// Also, while reviewing the data items in segmentDataCount, those who are consumed will be deleted to clean up space.
//
// This algorithm runs in O(N log N), being N the size of segmentDataCount, and allocates O(N) memory.
func findMarkableSegment(segmentDataCount map[int]*countDataItem, tooOldThreshold time.Duration, openSegment int) int {
	// N = len(segmentDataCount)
	// alloc slice, N
	orderedSegmentCounts := make([]processDataItem, 0, len(segmentDataCount))

	// convert map into slice, which already has expected capacity, N
	for seg, item := range segmentDataCount {
		orderedSegmentCounts = append(orderedSegmentCounts, processDataItem{
			segment:    seg,
			count:      item.count,
			lastUpdate: item.lastUpdate,
		})
	}

	// sort orderedSegmentCounts, N log N
	sort.Slice(orderedSegmentCounts, func(i, j int) bool {
		return orderedSegmentCounts[i].segment < orderedSegmentCounts[j].segment
	})

	var lastZero = -1
	for _, item := range orderedSegmentCounts {
		// the segment may still receive data
		if item.segment >= openSegment {
			break
		}

		// we consider a segment as "consumed if it's data count is zero, or the lastUpdate is too old
		if item.count == 0 || time.Since(item.lastUpdate) > tooOldThreshold {
			lastZero = item.segment
			// since the segment has been consumed, clear from map
			delete(segmentDataCount, item.segment)
		} else {
			// if we find a "non consumed" segment, we exit
			break
		}
	}

	return lastZero
}

// NewNopTracker creates a new no-op Tracker.
// This is useful when savepoint tracking
// is not needed or disabled.
func NewNopTracker() *NopTracker {
	return &NopTracker{}
}

var _ Tracker = (*NopTracker)(nil)

// NopTracker is a no-op implementation of Tracker. All methods
// are implemented as empty functions, making it suitable for use when savepoint
// tracking functionality is not required.
type NopTracker struct{}

func (n *NopTracker) LastStoredSegment() int { return noSegment }

func (n *NopTracker) UpdateReceivedData(segmentId int, dataCount int) {}

func (n *NopTracker) UpdateSentData(segmentId int, dataCount int) {}

func (n *NopTracker) MarkWALDrained() {}

func (n *NopTracker) Start() {}

func (n *NopTracker) Stop() {}
