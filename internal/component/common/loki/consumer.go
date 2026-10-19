package loki

import (
	"context"
	"errors"
	"slices"
	"sync"
)

// ErrConsumerStopped is returned by Consumer when an entry is
// submitted after the consumer has been stopped.
var ErrConsumerStopped = errors.New("consumer stopped")

// Consumer consumes batches of log entries.
type Consumer interface {
	// Consume consumes batch and may modify it. The caller may reuse batch once
	// Consume returns, so Consume must not retain any references to the batch itself
	// after it returns. Labels and individual entries read from batch may be kept.
	Consume(ctx context.Context, batch Batch) error
}

var _ Consumer = (*CollectingConsumer)(nil)

func NewCollectingConsumer() *CollectingConsumer {
	return &CollectingConsumer{}
}

// CollectingConsumer is a Consumer that will collect all received batches
// so it can be inspected later. Used in tests.
type CollectingConsumer struct {
	mut     sync.Mutex
	batches []Batch
}

func (c *CollectingConsumer) Consume(_ context.Context, batch Batch) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.batches = append(c.batches, batch.Clone())
	return nil
}

func (c *CollectingConsumer) Batches() []Batch {
	c.mut.Lock()
	defer c.mut.Unlock()

	return slices.Clone(c.batches)
}

func (c *CollectingConsumer) Entries() []Entry {
	c.mut.Lock()
	defer c.mut.Unlock()

	var entries []Entry
	for _, b := range c.batches {
		for _, s := range b.Streams() {
			for _, e := range s.Entries {
				entries = append(entries, NewEntryWithCreatedUnixMicro(s.Labels, s.Created(), e))
			}
		}
	}
	return entries
}

var _ Consumer = (*NopConsumer)(nil)

func NewNopConsumer() *NopConsumer {
	return &NopConsumer{}
}

type NopConsumer struct{}

func (n *NopConsumer) Consume(_ context.Context, _ Batch) error {
	return nil
}
