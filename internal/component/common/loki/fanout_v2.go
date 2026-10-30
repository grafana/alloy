package loki

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

var _ Consumer = (*FanoutV2)(nil)

// NewFanoutV2 creates a FanoutV2 that passes batches to children. It panics
// when a receiver does not support Loki pipeline V2.
func NewFanoutV2(children []LogsReceiver) *FanoutV2 {
	return &FanoutV2{
		consumers: consumersFrom(children),
	}
}

// FanoutV2 passes batches to multiple LogsReceivers through Loki pipeline V2.
type FanoutV2 struct {
	mut       sync.RWMutex
	consumers []Consumer
}

func (f *FanoutV2) Consume(ctx context.Context, batch Batch) error {
	// NOTE: We snapshot the consumer list under a brief read lock and release it
	// before delivering, rather than holding the lock for the whole fan-out.
	//
	// This is safe because delivery is a plain function call: a consumer that has
	// stopped returns ErrConsumerStopped instead of blocking, so delivering against
	// a stale snapshot can never deadlock. A consumer removed concurrently with
	// delivery returns ErrConsumerStopped, and Consume passes it on so the caller
	// retries against the new snapshot. The snapshot stays valid because
	// UpdateChildren replaces the slice rather than mutating it in place.
	// Releasing the lock early also keeps a slow consumer from blocking
	// reconfiguration.

	f.mut.RLock()
	consumers := f.consumers
	f.mut.RUnlock()

	var errs []error

	for i, consumer := range consumers {
		if i == len(consumers)-1 {
			if err := consumer.Consume(ctx, batch); err != nil {
				errs = append(errs, err)
			}
			continue
		}

		if err := consumer.Consume(ctx, batch.Clone()); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// UpdateChildren updates the receivers that batches are passed to. It panics
// when a receiver does not support Loki pipeline V2.
func (f *FanoutV2) UpdateChildren(children []LogsReceiver) {
	consumers := consumersFrom(children)

	f.mut.RLock()
	if !slices.Equal(f.consumers, consumers) {
		// Upgrade lock to write.
		f.mut.RUnlock()
		f.mut.Lock()
		f.consumers = consumers
		f.mut.Unlock()
	} else {
		f.mut.RUnlock()
	}
}

func consumersFrom(children []LogsReceiver) []Consumer {
	consumers := make([]Consumer, 0, len(children))
	for _, child := range children {
		if child == nil {
			continue
		}

		c, ok := child.(Consumer)
		if !ok {
			panic(fmt.Sprintf("fatal error in experimental Loki pipeline V2: %v does not support Loki pipeline V2", child))
		}
		consumers = append(consumers, c)
	}
	return consumers
}
