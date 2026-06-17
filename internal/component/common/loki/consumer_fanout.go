package loki

import (
	"context"
	"errors"
	"slices"
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
	// NOTE: We snapshot the consumer list under a brief read lock and release it
	// before delivering, rather than holding the lock for the whole fan-out.
	//
	// This is safe because delivery is a plain function call: a consumer that has
	// stopped returns ErrConsumerStopped instead of blocking, so delivering against
	// a stale snapshot can never deadlock — a consumer removed concurrently with
	// delivery simply reports that it has stopped (see isReportableError). The
	// snapshot stays valid because Update replaces the slice rather than mutating it
	// in place. Releasing the lock early also keeps a slow consumer from blocking
	// reconfiguration.

	f.mut.RLock()
	consumers := f.consumers
	f.mut.RUnlock()

	var errs []error

	for i, consumer := range consumers {
		if consumer == nil {
			continue
		}

		if i == len(consumers)-1 {
			if err := consumer.Consume(ctx, batch); isReportableError(err) {
				errs = append(errs, err)
			}
			continue
		}

		if err := consumer.Consume(ctx, batch.Clone()); isReportableError(err) {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (f *FanoutConsumer) ConsumeEntry(ctx context.Context, entry Entry) error {
	// NOTE: We snapshot the consumer list under a brief read lock and release it
	// before delivering, rather than holding the lock for the whole fan-out.
	//
	// This is safe because delivery is a plain function call: a consumer that has
	// stopped returns ErrConsumerStopped instead of blocking, so delivering against
	// a stale snapshot can never deadlock — a consumer removed concurrently with
	// delivery simply reports that it has stopped (see isReportableError). The
	// snapshot stays valid because Update replaces the slice rather than mutating it
	// in place. Releasing the lock early also keeps a slow consumer from blocking
	// reconfiguration.

	f.mut.RLock()
	consumers := f.consumers
	f.mut.RUnlock()

	var errs []error

	for _, consumer := range consumers {
		if consumer == nil {
			continue
		}

		// ConsumeEntry does not clone entries. Components that mutate an entry must
		// clone it first, as they already do. Batching will own cloning at the batch/pipeline boundary.
		if err := consumer.ConsumeEntry(ctx, entry); isReportableError(err) {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func (f *FanoutConsumer) Update(consumers []Consumer) {
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

func isReportableError(err error) bool {
	// We can always ignore ErrConsumerStopped.
	// If a downstream component have been stopped due to config update
	// we should no longer forward entries there and when alloy itself
	// stops we shut down components from sources to sinks so we should
	// never hit this error.
	return err != nil && !errors.Is(err, ErrConsumerStopped)
}
