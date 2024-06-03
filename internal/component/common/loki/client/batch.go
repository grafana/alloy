package client

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/golang/snappy"
	"github.com/prometheus/common/model"
	"golang.org/x/exp/slices"

	"github.com/grafana/loki/v3/pkg/logproto"

	"github.com/grafana/alloy/internal/component/common/loki"
)

const (
	errMaxStreamsLimitExceeded = "streams limit exceeded, streams: %d exceeds limit: %d, stream: '%s'"
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
	streams map[string]*logproto.Stream
	// totalBytes holds the total amounts of bytes, across the log lines in this batch.
	totalBytes int
	createdAt  time.Time

	maxStreams int

	// segmentCounter tracks the amount of entries for each segment present in this batch.
	segmentCounter map[int]int
}

func newBatch(maxStreams int, entries ...loki.Entry) *batch {
	b := &batch{
		streams:        map[string]*logproto.Stream{},
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
		return fmt.Errorf(errMaxStreamsLimitExceeded, streams, b.maxStreams, labels)
	}
	// Add the entry as a new stream
	b.streams[labels] = &logproto.Stream{
		Labels:  labels,
		Entries: []logproto.Entry{entry.Entry},
	}
	return nil
}

// addFromWAL adds an entry to the batch, tracking that the data being added comes from segment segmentNum read from the
// WAL.
func (b *batch) addFromWAL(lbs model.LabelSet, entry logproto.Entry, segmentNum int) error {
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
		return fmt.Errorf(errMaxStreamsLimitExceeded, streams, b.maxStreams, labels)
	}

	// Add the entry as a new stream
	b.streams[labels] = &logproto.Stream{
		Labels:  labels,
		Entries: []logproto.Entry{entry},
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
func (b *batch) sizeBytesAfter(entry logproto.Entry) int {
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
func (b *batch) createPushRequest() (*logproto.PushRequest, int) {
	req := logproto.PushRequest{
		Streams: make([]logproto.Stream, 0, len(b.streams)),
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

func entrySize(entry logproto.Entry) int {
	structuredMetadataSize := 0
	for _, label := range entry.StructuredMetadata {
		structuredMetadataSize += label.Size()
	}
	return len(entry.Line) + structuredMetadataSize
}
