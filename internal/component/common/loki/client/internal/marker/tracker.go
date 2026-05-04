package marker

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/common/loki/wal"
	"github.com/grafana/alloy/internal/runtime/logging/level"
)

type Tracker interface {
	wal.Marker

	// UpdateReceivedData sends an update event to the handler, that informs that some dataUpdate, coming from a particular WAL
	// segment, has been read out of the WAL and enqueued for sending.
	UpdateReceivedData(segmentId, dataCount int)

	// UpdateSentData sends an update event to the handler, informing that some dataUpdate, coming from a particular WAL
	// segment, has been delivered, or the sender has given up on it.
	UpdateSentData(segmentId, dataCount int) // Data which was sent or given up on sending

	// Stop stops the handler, and it's async processing of receive/send dataUpdate updates.
	Stop()
}

// SegmentTracker implements Tracker, processing data update events in an asynchronous manner, and tracking the last
// consumed segment in a file.
type SegmentTracker struct {
	dataIOUpdate      chan dataUpdate
	lastMarkedSegment int
	logger            log.Logger
	file              *File
	maxSegmentAge     time.Duration
	metrics           *Metrics
	quit              chan struct{}
	runFindTicker     *time.Ticker
	wg                sync.WaitGroup
}

// dataUpdate is an update event that some amount of data has been read out of the WAL and enqueued, delivered or dropped.
type dataUpdate struct {
	segmentId int
	dataCount int
}

var _ Tracker = (*SegmentTracker)(nil)

// NewTracker creates a new SegmentTracker.
func NewTracker(file *File, maxSegmentAge time.Duration, logger log.Logger, metrics *Metrics) *SegmentTracker {
	t := &SegmentTracker{
		lastMarkedSegment: -1, // Segment ID last marked on disk.
		file:              file,
		//TODO: What is a good size for the channel?
		dataIOUpdate: make(chan dataUpdate, 100),
		quit:         make(chan struct{}),
		logger:       logger,
		metrics:      metrics,

		maxSegmentAge: maxSegmentAge,
		// runFindTicker will force the execution of the find markable segment routine every second
		runFindTicker: time.NewTicker(time.Second),
	}

	// Load the last marked segment from disk (if it exists).
	if lastSegment := t.file.LastMarkedSegment(); lastSegment >= 0 {
		t.lastMarkedSegment = lastSegment
	}

	t.wg.Go(t.runUpdatePendingData)

	return t
}

func (mh *SegmentTracker) LastMarkedSegment() int {
	return mh.file.LastMarkedSegment()
}

func (mh *SegmentTracker) UpdateReceivedData(segmentId, dataCount int) {
	mh.dataIOUpdate <- dataUpdate{
		segmentId: segmentId,
		dataCount: dataCount,
	}
}

func (mh *SegmentTracker) UpdateSentData(segmentId, dataCount int) {
	mh.dataIOUpdate <- dataUpdate{
		segmentId: segmentId,
		dataCount: -1 * dataCount,
	}
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
func (mh *SegmentTracker) runUpdatePendingData() {
	segmentDataCount := make(map[int]*countDataItem)

	for {
		// shouldRunFind will be true if a markable segment should be found after the update, that is if one reached a count
		// of zero, or a ticker fired
		shouldRunFind := false
		select {
		case <-mh.quit:
			return
		case update := <-mh.dataIOUpdate:
			if di, ok := segmentDataCount[update.segmentId]; ok {
				di.lastUpdate = time.Now()
				resultingCount := di.count + update.dataCount
				di.count = resultingCount
				// if a segment reached zero, run find routine because a segment might be ready to be marked
				shouldRunFind = resultingCount == 0
			} else {
				segmentDataCount[update.segmentId] = &countDataItem{
					count:      update.dataCount,
					lastUpdate: time.Now(),
				}
			}
		}

		// if ticker fired, force run find
		select {
		case <-mh.runFindTicker.C:
			shouldRunFind = true
		default:
		}

		if !shouldRunFind {
			continue
		}

		markableSegment := FindMarkableSegment(segmentDataCount, mh.maxSegmentAge)
		level.Debug(mh.logger).Log("msg", fmt.Sprintf("found as markable segment %d", markableSegment))
		if markableSegment > mh.lastMarkedSegment {
			mh.file.MarkSegment(markableSegment)
			mh.lastMarkedSegment = markableSegment
			mh.metrics.lastMarkedSegment.WithLabelValues().Set(float64(markableSegment))
		}
	}
}

func (mh *SegmentTracker) Stop() {
	mh.runFindTicker.Stop()
	mh.quit <- struct{}{}
	mh.wg.Wait()
}

// FindMarkableSegment finds, given the summary of data updates received, and a threshold on how much time can pass for
// a segment that hasn't received updates to be considered as "live", the segment that should be marked as last consumed.
// The algorithm will find the highest numbered segment that is considered as "consumed", with its all predecessors
// "consumed" as well.
//
// A consumed segment is one with data count of zero, meaning that there's no data left in flight for it, or it hasn't
// received any updates for tooOldThreshold time.
//
// Also, while reviewing the data items in segmentDataCount, those who are consumed will be deleted to clean up space.
//
// This algorithm runs in O(N log N), being N the size of segmentDataCount, and allocates O(N) memory.
func FindMarkableSegment(segmentDataCount map[int]*countDataItem, tooOldThreshold time.Duration) int {
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
// This is useful when marker tracking
// is not needed or disabled.
func NewNopTracker() *NopTracker {
	return &NopTracker{}
}

var _ Tracker = (*NopTracker)(nil)

// NopTracker is a no-op implementation of Tracker. All methods
// are implemented as empty functions, making it suitable for use when marker
// tracking functionality is not required.
type NopTracker struct{}

func (n *NopTracker) LastMarkedSegment() int { return -1 }

func (n *NopTracker) UpdateReceivedData(segmentId int, dataCount int) {}

func (n *NopTracker) UpdateSentData(segmentId int, dataCount int) {}

func (n *NopTracker) Stop() {}
