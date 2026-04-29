package loki

import (
	"context"
	"errors"
	"sync"
)

var _ Consumer = (*FanoutConsumer)(nil)

func NewFanoutConsumer(consumers []Consumer) *FanoutConsumer {
	return &FanoutConsumer{
		consumers: consumers,
	}
}

type FanoutConsumer struct {
	mut       sync.RWMutex
	consumers []Consumer
}

func (f *FanoutConsumer) Consume(ctx context.Context, batch Batch) error {
	// NOTE: It's important that we hold a read lock for the duration of Consume
	// rather than making a copy of consumers and releasing the lock early.
	//
	// When config is updated, the loader evaluates all components and updates
	// them while they continue running. The scheduler only stops removed components
	// after all updates complete. During this window, Send may execute concurrently
	// with receiver list updates. By holding the read lock for the entire Send
	// operation, receiver list updates (which require a write lock) will block
	// until all in-flight Send calls complete. This prevents sending entries to
	// receivers that have been removed by the scheduler.

	f.mut.RLock()
	defer f.mut.RUnlock()

	var errs []error

	for i, consumer := range f.consumers {
		if i == len(f.consumers)-1 {
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

func (f *FanoutConsumer) ConsumeEntry(ctx context.Context, entry Entry) error {
	// NOTE: It's important that we hold a read lock for the duration of ConsumeEntry
	// rather than making a copy of consumers and releasing the lock early.
	//
	// When config is updated, the loader evaluates all components and updates
	// them while they continue running. The scheduler only stops removed components
	// after all updates complete. During this window, Send may execute concurrently
	// with receiver list updates. By holding the read lock for the entire Send
	// operation, receiver list updates (which require a write lock) will block
	// until all in-flight Send calls complete. This prevents sending entries to
	// receivers that have been removed by the scheduler.

	f.mut.RLock()
	defer f.mut.RUnlock()

	var errs []error

	for i, consumer := range f.consumers {
		if i == len(f.consumers)-1 {
			if err := consumer.ConsumeEntry(ctx, entry); err != nil {
				errs = append(errs, err)
			}
			continue
		}

		if err := consumer.ConsumeEntry(ctx, entry.Clone()); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (f *FanoutConsumer) Update(consumers []Consumer) {
	f.mut.RLock()
	if requireUpdate(f.consumers, consumers) {
		// Upgrade lock to write.
		f.mut.RUnlock()
		f.mut.Lock()
		f.consumers = consumers
		f.mut.Unlock()
	} else {
		f.mut.RUnlock()
	}
}
