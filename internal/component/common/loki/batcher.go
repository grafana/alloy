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

var _ StreamBatcher = (*ChannelStreamBatcher)(nil)

// ChannelStreamBatcher holds the last added entry and sends it through a
// Fanout on Flush.
type ChannelStreamBatcher struct {
	fanout  *Fanout
	labels  model.LabelSet
	entry   push.Entry
	pending bool
}

// NewChannelStreamBatcher creates a ChannelStreamBatcher that sends entries
// with labels through fanout.
func NewChannelStreamBatcher(fanout *Fanout, labels model.LabelSet) *ChannelStreamBatcher {
	return &ChannelStreamBatcher{fanout: fanout, labels: labels}
}

func (b *ChannelStreamBatcher) Add(entry push.Entry) bool {
	b.entry = entry
	b.pending = true
	return true
}

func (b *ChannelStreamBatcher) Flush(ctx context.Context) error {
	if !b.pending {
		return nil
	}

	b.pending = false
	return b.fanout.Send(ctx, NewEntry(b.labels, b.entry))
}

var _ StreamBatcher = (*ConsumerStreamBatcher)(nil)

// ConsumerStreamBatcher collects entries into one stream and passes it to a
// Consumer on Flush.
type ConsumerStreamBatcher struct {
	consumer  Consumer
	labels    model.LabelSet
	flushSize int

	created int64
	batch   Batch
	entries []push.Entry
}

// NewConsumerStreamBatcher creates a StreamBatcher that collects entries
// with labels and passes them to consumer on Flush. Add asks for a Flush
// once flushSize entries are added.
func NewConsumerStreamBatcher(consumer Consumer, labels model.LabelSet, flushSize int) *ConsumerStreamBatcher {
	return &ConsumerStreamBatcher{
		consumer:  consumer,
		labels:    labels,
		flushSize: flushSize,
	}
}

func (b *ConsumerStreamBatcher) Add(entry push.Entry) bool {
	if len(b.entries) == 0 {
		b.created = time.Now().UnixMicro()
	}
	b.entries = append(b.entries, entry)
	return len(b.entries) >= b.flushSize
}

func (b *ConsumerStreamBatcher) Flush(ctx context.Context) error {
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
