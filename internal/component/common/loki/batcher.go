package loki

import (
	"context"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
)

// StreamBatcher collects entries of a single stream from one producer and
// delivers them on Flush. A StreamBatcher can be reused after
// Flush and is not safe for concurrent use.
type StreamBatcher interface {
	// Add adds an entry to the stream and reports whether the caller must
	// Flush before the next Add.
	Add(entry push.Entry) bool
	// Flush delivers all entries added since the last Flush. If delivery
	// fails the entries are dropped, possibly after reaching some targets, and
	// a later Flush does not resend them.
	Flush(ctx context.Context) error
}

var _ StreamBatcher = (*StreamBatcherV1)(nil)

// StreamBatcherV1 holds the last added entry and sends it to a LogsReceiver
// on Flush.
type StreamBatcherV1 struct {
	recv    LogsReceiver
	labels  model.LabelSet
	entry   push.Entry
	pending bool
}

// NewStreamBatcherV1 creates a StreamBatcherV1 that sends entries with labels
// to recv.
func NewStreamBatcherV1(recv LogsReceiver, labels model.LabelSet) *StreamBatcherV1 {
	return &StreamBatcherV1{recv: recv, labels: labels}
}

func (b *StreamBatcherV1) Add(entry push.Entry) bool {
	b.entry = entry
	b.pending = true
	return true
}

func (b *StreamBatcherV1) Flush(ctx context.Context) error {
	if !b.pending {
		return nil
	}

	b.pending = false
	select {
	case <-ctx.Done():
		return ctx.Err()
	case b.recv.Chan() <- NewEntry(b.labels, b.entry):
		return nil
	}
}

var _ StreamBatcher = (*StreamBatcherV2)(nil)

// StreamBatcherV2 collects entries into one stream and passes it to a
// Consumer on Flush.
type StreamBatcherV2 struct {
	consumer  Consumer
	labels    model.LabelSet
	flushSize int

	created int64
	batch   Batch
	entries []push.Entry
}

// NewStreamBatcherV2 creates a StreamBatcherV2 that collects entries
// with labels and passes them to consumer on Flush. Add asks for a Flush
// once flushSize entries are added.
func NewStreamBatcherV2(consumer Consumer, labels model.LabelSet, flushSize int) *StreamBatcherV2 {
	return &StreamBatcherV2{
		consumer:  consumer,
		labels:    labels,
		flushSize: flushSize,
	}
}

func (b *StreamBatcherV2) Add(entry push.Entry) bool {
	if len(b.entries) == 0 {
		b.created = time.Now().UnixMicro()
	}
	b.entries = append(b.entries, entry)
	return len(b.entries) >= b.flushSize
}

func (b *StreamBatcherV2) Flush(ctx context.Context) error {
	if len(b.entries) == 0 {
		return nil
	}

	defer func() {
		b.batch.Reset()
		b.entries = b.entries[:0]
	}()

	b.batch.Add(NewStreamWithCreatedUnixMicro(b.labels, b.created, b.entries...))
	return b.consumer.Consume(ctx, b.batch)
}
