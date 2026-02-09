package client

import (
	"errors"
	"fmt"
	"math/bits"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/golang/snappy"
	"github.com/grafana/loki/pkg/push"
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
	// req is the push request holding streams and entries to send to Loki.
	req *push.PushRequest
	// createdAt is when the batch was created.
	createdAt time.Time
	// maxSize is the maximum batch size in bytes. At least one entry is always
	// allowed even if it exceeds this limit.
	maxSize int
	// maxStreams is the maximum number of streams in the batch. Zero means no limit.
	maxStreams int
	// entriesTotal is the total number of log entries across all streams in the batch.
	entriesTotal int
	// size is the total serialized size of the push request in bytes.
	size int
	// segmentCounter tracks the amount of entries for each segment present in this batch.
	segmentCounter map[int]int
}

func newBatch(maxStreams, maxSize int) *batch {
	req := &push.PushRequest{}
	return &batch{
		req:            req,
		createdAt:      time.Now(),
		maxSize:        maxSize,
		maxStreams:     maxStreams,
		size:           req.Size(),
		segmentCounter: map[int]int{},
	}
}

// add adds an entry to the batch. It returns errBatchSizeReached when
// entry cannot be added because it would exceed maxSize and
// errMaxStreamsLimitExceeded when adding a new stream would exceed maxStreams.
// segmentNum associates the entry with a WAL segment and is unused for non-WAL clients.
func (b *batch) add(entry loki.Entry, segmentNum int) error {
	labels := labelsMapToString(entry.Labels)

	stream, ok := b.getStream(labels)
	streamLen := len(b.req.Streams)

	// Reject if we would exceed the maxStreams limit.
	if !ok && b.maxStreams > 0 && streamLen >= b.maxStreams {
		return fmt.Errorf("%w, streams: %d exceeds limit: %d, stream: '%s'", errMaxStreamsLimitExceeded, streamLen, b.maxStreams, labels)
	}

	// Append the entry to an already existing stream.
	if ok {
		oldSize := sizeForStream(stream)
		stream.Entries = append(stream.Entries, entry.Entry)
		newSize := sizeForStream(stream)
		// Request size grows by stream delta plus change in the stream's length varint.
		delta := newSize - oldSize
		if b.canAdd(delta) {
			stream.Entries = stream.Entries[:len(stream.Entries)-1]
			return errBatchSizeReached
		}
		b.entriesTotal += 1
		b.size += delta
		b.countForSegment(segmentNum)
		return nil
	}

	stream = &push.Stream{
		Labels:  labels,
		Entries: []push.Entry{entry.Entry},
	}

	// Add the entry to a new stream.
	b.req.Streams = append(b.req.Streams, *stream)

	size := sizeForStream(stream)
	// NOTE: We will always allow to add at least one entry to a batch
	// even if that entry makes the size bigger than maxSize.
	if streamLen != 0 && b.canAdd(size) {
		b.req.Streams = b.req.Streams[:len(b.req.Streams)-1]
		return errBatchSizeReached
	}

	b.entriesTotal += 1
	b.countForSegment(segmentNum)
	b.size += size

	return nil
}

func (b *batch) getStream(labels string) (*push.Stream, bool) {
	i := slices.IndexFunc(b.req.Streams, func(stream push.Stream) bool {
		return stream.Labels == labels
	})

	if i == -1 {
		return nil, false
	}

	return &b.req.Streams[i], true
}

// sizeBytes returns the current batch size in bytes.
func (b *batch) sizeBytes() int {
	return b.size
}

func (b *batch) canAdd(size int) bool {
	return b.sizeBytes()+size > b.maxSize
}

// age of the batch since its creation
func (b *batch) age() time.Duration {
	return time.Since(b.createdAt)
}

// encode marshals the batch to a snappy-compressed push request using the
// given buffers, and returns the encoded bytes, the number of entries, and any error.
// If the batch does not fit in protoBuf or the compressed output does not fit in
// snappyBuf, new buffers are allocated and the caller's buffers are not reused.
// protoBuf and snappyBuf must not overlap.
func (b *batch) encode(protoBuf, snappyBuf []byte) ([]byte, int, error) {
	size := b.sizeBytes()

	// Note: Because we are always allowing at least one
	// entry to be added to a batch eventhough that would
	// exceed that size limit we need to make sure we have
	// enough space in the buffer.
	if size > len(protoBuf) {
		protoBuf = make([]byte, size)
	}

	n, err := b.req.MarshalToSizedBuffer(protoBuf[:size])
	if err != nil {
		return nil, 0, err
	}

	buf := snappy.Encode(snappyBuf, protoBuf[:n])
	return buf, b.entriesTotal, nil
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

// sizeForStream returns the size of the stream in bytes.
func sizeForStream(stream *push.Stream) int {
	streamSize := stream.Size()
	// 1 for the tag, varintSize for the length, and streamSize for the payload
	return 1 + varintSize(uint64(streamSize)) + streamSize
}

// varintSize returns the number of bytes needed to encode x as a protobuf varint.
func varintSize(x uint64) (n int) {
	return (bits.Len64(x|1) + 6) / 7
}
