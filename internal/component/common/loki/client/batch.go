package client

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"

	"github.com/grafana/loki/pkg/push"

	"github.com/grafana/alloy/internal/component/common/loki"
)

var (
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
	// totalBytes holds the total amounts of bytes, across the log lines in this batch.
	totalBytes int
	createdAt  time.Time

	maxStreams int

	// segmentCounter tracks the amount of entries for each segment present in this batch.
	segmentCounter map[int]int
}

func newBatch(maxStreams int, entries ...loki.Entry) *batch {
	b := &batch{
		streams:        map[string]*push.Stream{},
		totalBytes:     0,
		createdAt:      time.Now(),
		maxStreams:     maxStreams,
		segmentCounter: map[int]int{},
	}

	// Add entries to the batch
	for _, entry := range entries {
		//never error here
		_ = b.add(entry)
	}

	return b
}

// add an entry to the batch
func (b *batch) add(entry loki.Entry) error {
	b.totalBytes += entrySize(entry.Entry)

	// Append the entry to an already existing stream (if any)
	labels := labelsMapToString(entry.Labels, ReservedLabelTenantID)
	if stream, ok := b.streams[labels]; ok {
		stream.Entries = append(stream.Entries, entry.Entry)
		return nil
	}

	streams := len(b.streams)
	if b.maxStreams > 0 && streams >= b.maxStreams {
		return fmt.Errorf("%w, streams: %d exceeds limit: %d, stream: '%s'", errMaxStreamsLimitExceeded, streams, b.maxStreams, labels)
	}
	// Add the entry as a new stream
	b.streams[labels] = &push.Stream{
		Labels:  labels,
		Entries: []push.Entry{entry.Entry},
	}
	return nil
}

// addFromWAL adds an entry to the batch, tracking that the data being added comes from segment segmentNum read from the
// WAL.
func (b *batch) addFromWAL(lbs model.LabelSet, entry push.Entry, segmentNum int) error {
	b.totalBytes += len(entry.Line)

	// Append the entry to an already existing stream (if any)
	labels := labelsMapToString(lbs, ReservedLabelTenantID)
	if stream, ok := b.streams[labels]; ok {
		stream.Entries = append(stream.Entries, entry)
		b.countForSegment(segmentNum)
		return nil
	}

	streams := len(b.streams)
	if b.maxStreams > 0 && streams >= b.maxStreams {
		return fmt.Errorf("%w, streams: %d exceeds limit: %d, stream: '%s'", errMaxStreamsLimitExceeded, streams, b.maxStreams, labels)
	}

	// Add the entry as a new stream
	b.streams[labels] = &push.Stream{
		Labels:  labels,
		Entries: []push.Entry{entry},
	}
	b.countForSegment(segmentNum)

	return nil
}

// labelsMapToString encodes an entry's label set as a string, ignoring the without label.
func labelsMapToString(ls model.LabelSet, without model.LabelName) string {
	var b strings.Builder
	totalSize := 2
	lstrs := make([]model.LabelName, 0, len(ls))

	for l, v := range ls {
		if l == without {
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

// sizeBytes returns the current batch size in bytes
func (b *batch) sizeBytes() int {
	return b.totalBytes
}

// sizeBytesAfter returns the size of the batch after the input entry
// will be added to the batch itself
func (b *batch) sizeBytesAfter(entry push.Entry) int {
	return b.totalBytes + entrySize(entry)
}

// age of the batch since its creation
func (b *batch) age() time.Duration {
	return time.Since(b.createdAt)
}

// encode the batch as snappy-compressed push request, and returns
// the encoded bytes and the number of encoded entries
func (b *batch) encode() ([]byte, int, error) {
	req, entriesCount := b.createPushRequest()
	buf, err := proto.Marshal(req)
	if err != nil {
		return nil, 0, err
	}
	buf = snappy.Encode(nil, buf)
	return buf, entriesCount, nil
}

// creates push request and returns it, together with number of entries
func (b *batch) createPushRequest() (*push.PushRequest, int) {
	req := push.PushRequest{
		Streams: make([]push.Stream, 0, len(b.streams)),
	}

	entriesCount := 0
	for _, stream := range b.streams {
		req.Streams = append(req.Streams, *stream)
		entriesCount += len(stream.Entries)
	}
	return &req, entriesCount
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
func (b *batch) reportAsSentData(h SentDataMarkerHandler) {
	for seg, data := range b.segmentCounter {
		h.UpdateSentData(seg, data)
	}
}

func entrySize(entry push.Entry) int {
	structuredMetadataSize := 0
	for _, label := range entry.StructuredMetadata {
		structuredMetadataSize += label.Size()
	}
	return len(entry.Line) + structuredMetadataSize
}
