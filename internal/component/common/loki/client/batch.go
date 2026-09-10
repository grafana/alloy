package client

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
)

var (
	errBatchSizeReached        = errors.New("batch size reached")
	errMaxStreamsLimitExceeded = errors.New("streams limit exceeded")
)

// sentDataTracker is a subset of the marker.Tracker interface, that the batch interacts with to report the event that
// all data in the batch has been delivered or a client failed to do so.
type sentDataTracker interface {
	UpdateSentData(segmentId, dataCount int)
}

// batch holds pending log streams waiting to be sent to Loki, and it's used
// to reduce the number of push requests to Loki aggregating multiple log streams
// and entries in a single batch request. In case of multi-tenant Promtail, log
// streams for each tenant are stored in a dedicated batch.
type batch struct {
	streams map[string]*push.Stream
	// created stores per-entry creation timestamps in unix micro seconds for latency observation.
	created []int64
	// createdAt is when the batch was created.
	createdAt time.Time
	// maxSize is the maximum batch size in bytes. At least one entry is always
	// allowed even if it exceeds this limit.
	maxSize int
	// maxStreams is the maximum number of streams in the batch. Zero means no limit.
	maxStreams int
	// size holds the total number of bytes across log lines in this batch.
	size int
	// segmentCounter tracks the amount of entries for each segment present in this batch.
	segmentCounter map[int]int
}

func newBatch(maxStreams, maxSize int) *batch {
	return &batch{
		streams:        make(map[string]*push.Stream),
		createdAt:      time.Now(),
		maxSize:        maxSize,
		maxStreams:     maxStreams,
		segmentCounter: map[int]int{},
	}
}

// add adds an entry to the batch. It returns errBatchSizeReached when
// entry cannot be added because it would exceed maxSize and
// errMaxStreamsLimitExceeded when adding a new stream would exceed maxStreams.
// segmentNum associates the entry with a WAL segment and is unused for non-WAL clients.
func (b *batch) add(entry loki.Entry, segmentNum int) error {
	labels := labelsMapToString(entry.Labels)

	stream, ok := b.streams[labels]
	if ok {
		size := entry.Size()
		if !b.canAdd(size) {
			return errBatchSizeReached
		}

		b.size += size
		b.countForSegment(segmentNum)
		b.created = append(b.created, entry.Created())
		stream.Entries = append(stream.Entries, entry.Entry)
		return nil
	}

	streams := len(b.streams)
	// Reject if we would exceed the maxStreams limit.
	if b.maxStreams > 0 && streams >= b.maxStreams {
		return fmt.Errorf("%w, streams: %d exceeds limit: %d, stream: '%s'", errMaxStreamsLimitExceeded, streams, b.maxStreams, labels)
	}

	size := entry.Size()
	// NOTE: We will always allow to add at least one entry to a batch
	// even if that entry makes the size bigger than maxSize.
	if streams != 0 && !b.canAdd(size) {
		return errBatchSizeReached
	}

	b.size += size
	b.countForSegment(segmentNum)
	b.created = append(b.created, entry.Created())
	b.streams[labels] = &push.Stream{
		Labels:  labels,
		Entries: []push.Entry{entry.Entry},
	}
	return nil
}

// canAdd reports whether adding size bytes would exceed the batch's maxSize.
func (b *batch) canAdd(size int) bool {
	return b.size+size <= b.maxSize
}

// age of the batch since its creation
func (b *batch) age() time.Duration {
	return time.Since(b.createdAt)
}

// request returns a PushRequest and number of entries it contains.
func (b *batch) request() (*push.PushRequest, int) {
	req := &push.PushRequest{Streams: make([]push.Stream, 0, len(b.streams))}

	var entries int
	for _, stream := range b.streams {
		req.Streams = append(req.Streams, *stream)
		entries += len(stream.Entries)
	}
	return req, entries
}

// split divides the batch into two smaller batches, so a batch that Loki
// rejected as too large can be retried in halves.
//
// When the batch holds more than one stream, the streams are divided in half.
// When it holds a single stream, that stream's entries are divided in half
// instead, the first of the two keeping the earlier entries. It reports false
// when the batch cannot be divided any further, that is when it holds a single
// entry.
//
// The halves carry no created timestamps and no segment counters: only the
// batch they were divided from reports sent data and entry latency, so the
// halves must never be passed to reportAsSentData, nor added to.
func (b *batch) split() (*batch, *batch, bool) {
	switch len(b.streams) {
	case 0:
		return nil, nil, false
	case 1:
		return b.splitEntries()
	default:
		return b.splitStreams()
	}
}

// splitStreams divides the batch's streams in half. It always succeeds, since
// more than one stream can always be divided into two. b must hold more than
// one stream.
func (b *batch) splitStreams() (*batch, *batch, bool) {
	// Which streams end up in which half doesn't matter, so take them in map
	// order rather than paying to order them.
	mid := len(b.streams) / 2
	split1, split2 := b.newSplit(mid), b.newSplit(len(b.streams)-mid)

	for labels, stream := range b.streams {
		dst := split2
		if len(split1.streams) < mid {
			dst = split1
		}
		dst.streams[labels] = stream
		dst.size += entriesSize(stream.Entries)
	}

	return split1, split2, true
}

// splitEntries divides the entries of the batch's only stream in half, the
// first of the two keeping the earlier entries. It reports false when that
// stream holds a single entry, which cannot be divided any further. b must hold
// exactly one stream.
func (b *batch) splitEntries() (*batch, *batch, bool) {
	var stream *push.Stream
	for _, s := range b.streams {
		stream = s
	}

	if len(stream.Entries) < 2 {
		return nil, nil, false
	}

	mid := len(stream.Entries) / 2
	entries1, entries2 := stream.Entries[:mid], stream.Entries[mid:]

	split1, split2 := b.newSplit(1), b.newSplit(1)
	split1.streams[stream.Labels] = &push.Stream{Labels: stream.Labels, Entries: entries1}
	split1.size = entriesSize(entries1)
	split2.streams[stream.Labels] = &push.Stream{Labels: stream.Labels, Entries: entries2}
	split2.size = entriesSize(entries2)

	return split1, split2, true
}

// newSplit returns an empty batch sized for n streams, sharing this batch's
// limits and creation time. See split for why created is left empty.
func (b *batch) newSplit(n int) *batch {
	return &batch{
		streams:        make(map[string]*push.Stream, n),
		createdAt:      b.createdAt,
		maxSize:        b.maxSize,
		maxStreams:     b.maxStreams,
		segmentCounter: map[int]int{},
	}
}

// entriesSize is the number of bytes across the entries' log lines, matching
// how batch.size is accumulated in add.
func entriesSize(entries []push.Entry) int {
	var size int
	for _, e := range entries {
		entry := loki.Entry{Entry: e}
		size += entry.Size()
	}
	return size
}

// countForSegment tracks that one data item has been read from a certain WAL segment.
func (b *batch) countForSegment(segmentNum int) {
	if curr, ok := b.segmentCounter[segmentNum]; ok {
		b.segmentCounter[segmentNum] = curr + 1
		return
	}
	b.segmentCounter[segmentNum] = 1
}

// reportAsSentData reports sent data counts per segment and observes per-entry propagation latency.
func (b *batch) reportAsSentData(t sentDataTracker, obs prometheus.Observer) {
	for seg, data := range b.segmentCounter {
		t.UpdateSentData(seg, data)
	}

	now := time.Now().UnixMicro()
	for _, created := range b.created {
		// NOTE: Some WAL entries may not have a created timestamp, so we ignore 0.
		// We also only record entries where created <= now. Since created is stored as
		// Unix microseconds, monotonic time is lost. If wall clock adjustments make
		// created appear in the future, we skip that sample.
		if created != 0 && created <= now {
			// Track entry propagation latency in seconds.
			obs.Observe(float64(now-created) / 1e6)
		}
	}
}

// 15 matches Loki's default maximum for indexed labels.
const maxPooledLabelNamesCapacity = 15

var labelNamesPool = sync.Pool{
	New: func() any {
		s := make([]model.LabelName, 0, maxPooledLabelNamesCapacity)
		return &s
	},
}

// labelsMapToString encodes an entry's label set as a string, ignoring internal labels
func labelsMapToString(ls model.LabelSet) string {
	var (
		totalSize = 2
		pooled    = labelNamesPool.Get().(*[]model.LabelName)
		lstrs     = *pooled
	)

	defer func() {
		// Only return slices that stayed within the pooled capacity, this avoids
		// retaining large one-off backing arrays.
		if cap(lstrs) <= maxPooledLabelNamesCapacity {
			// append may have updated the slice header so write it back before
			// returning the slice to the pool.
			// This is not necessary with the current cap check, but we keep it so
			// the pooled slice state stays correct if that ever changes.
			*pooled = lstrs[:0]
			labelNamesPool.Put(pooled)
		}
	}()

	for l, v := range ls {
		// skip internal labels
		if strings.HasPrefix(string(l), "__") {
			continue
		}

		lstrs = append(lstrs, l)
		// guess size increase: 2 for `, ` between labels and 3 for the `=` and quotes around label value
		totalSize += len(l) + 2 + len(v) + 3
	}

	slices.Sort(lstrs)

	// Build into a local byte slice so strconv.AppendQuote can write directly
	// into the final buffer. With strings.Builder we would need strconv.Quote,
	// which creates an intermediate quoted string for each label value.
	buf := make([]byte, 0, totalSize)
	buf = append(buf, '{')
	for i, l := range lstrs {
		if i > 0 {
			buf = append(buf, ',', ' ')
		}

		buf = append(buf, string(l)...)
		buf = append(buf, '=')
		buf = strconv.AppendQuote(buf, string(ls[l]))
	}
	buf = append(buf, '}')

	// #nosec G103 nosemgrep: use-of-unsafe-block
	// Safety: buf is local to this call and is never mutated again after converting
	// it to a string so the returned strings backing bytes remain immutable.
	return unsafe.String(unsafe.SliceData(buf), len(buf))
}
