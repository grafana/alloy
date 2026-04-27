package loki

import (
	"context"
	"reflect"
	"sync"
)

type Consumer interface {
	Consume(ctx context.Context, batch Batch) error
	ConsumeEntry(ctx context.Context, entry Entry) error
}

var _ Consumer = (*CollectingConsumer)(nil)

func NewCollectingConsumer() *CollectingConsumer {
	return &CollectingConsumer{}
}

// CollectingConsumer is a Consumer that will collect all received entries
// and batches so it can be inspected later.
// Used in tests.
type CollectingConsumer struct {
	mut     sync.Mutex
	batches []Batch
	entries []Entry
}

func (c *CollectingConsumer) Consume(_ context.Context, batch Batch) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.batches = append(c.batches, batch)

	return nil
}

func (c *CollectingConsumer) ConsumeEntry(_ context.Context, entry Entry) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	c.entries = append(c.entries, entry)
	return nil
}

func (c *CollectingConsumer) Batches() []Batch {
	c.mut.Lock()
	defer c.mut.Unlock()

	return c.batches
}

func (c *CollectingConsumer) Entries() []Entry {
	c.mut.Lock()
	defer c.mut.Unlock()
	return c.entries
}

func requireUpdate[T any](prev, next []T) bool {
	if len(prev) != len(next) {
		return true
	}
	for i := range prev {
		if !reflect.DeepEqual(prev[i], next[i]) {
			return true
		}
	}
	return false
}
