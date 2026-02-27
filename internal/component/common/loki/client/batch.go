package client

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
)

var (
	errBatchSizeReached        = errors.New("batch size reached")
	errMaxStreamsLimitExceeded = errors.New("streams limit exceeded")
)

// SentDataMarkerHandler is a slice of the MarkerHandler interface, that the batch interacts with to report the event that
// all data in the batch has been delivered or a client failed to do so.
type SentDataMarkerHandler interface {
	UpdateSentData(segmentId, dataCount int)
}

// batch holds pending log streams waiting to be sent to Loki, and it's used
// to reduce the number of push requests to Loki aggregating multiple log streams
// and entries in a single batch request. In case of multi-tenant Promtail, log
// streams for each tenant are stored in a dedicated batch.
type batch struct {
	streams map[string]*push.Stream
	// FIXME(kalleep): this is bad
	created []time.Time
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

// countForSegment tracks that one data item has been read from a certain WAL segment.
func (b *batch) countForSegment(segmentNum int) {
	if curr, ok := b.segmentCounter[segmentNum]; ok {
		b.segmentCounter[segmentNum] = curr + 1
		return
	}
	b.segmentCounter[segmentNum] = 1
}

// reportAsSentData will report for all segments whose data is part of this batch, the amount of that data as sent to
// the provided SentDataMarkerHandler
func (b *batch) reportAsSentData(h SentDataMarkerHandler, obs prometheus.Observer) {
	for seg, data := range b.segmentCounter {
		h.UpdateSentData(seg, data)
	}

	now := time.Now()
	for _, t := range b.created {
		obs.Observe(float64(now.Sub(t).Seconds()))
	}
}

// labelsMapToString encodes an entry's label set as a string, ignoring internal labels
func labelsMapToString(ls model.LabelSet) string {
	var b strings.Builder
	totalSize := 2
	lstrs := make([]model.LabelName, 0, len(ls))

	for l, v := range ls {
		// skip internal labels
		if strings.HasPrefix(string(l), "__") {
			continue
		}

		lstrs = append(lstrs, l)
		// guess size increase: 2 for `, ` between labels and 3 for the `=` and quotes around label value
		totalSize += len(l) + 2 + len(v) + 3
	}

	b.Grow(totalSize)
	b.WriteByte('{')
	slices.Sort(lstrs)
	for i, l := range lstrs {
		if i > 0 {
			b.WriteString(", ")
		}

		b.WriteString(string(l))
		b.WriteString(`=`)
		b.WriteString(strconv.Quote(string(ls[l])))
	}
	b.WriteByte('}')

	return b.String()
}
