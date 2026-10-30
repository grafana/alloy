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
	// Consume returns one of:
	//   - nil: the batch has been accepted and the caller can commit it.
	//   - A context error: ctx was cancelled and the batch may not have been
	//     delivered. The caller should not commit it.
	//   - ErrConsumerStopped: a consumer in the pipeline has stopped, usually because
	//     of a config reload, and the batch may not have been delivered. The caller
	//     should not commit it and should retry with a copy of the original batch
	//     until Consume succeeds or ctx is cancelled.
	//   - Any other error: unexpected, most likely a bug.
	//
	// When Consume returns an error, check ctx.Err() to tell a context error apart
	// instead of inspecting the returned error. The entries may already have reached
	// some consumers, so sending the batch again can produce duplicates.
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

var _ Consumer = (*NopConsumer)(nil)

func NewNopConsumer() *NopConsumer {
	return &NopConsumer{}
}

type NopConsumer struct{}

func (n *NopConsumer) Consume(_ context.Context, _ Batch) error {
	return nil
}
