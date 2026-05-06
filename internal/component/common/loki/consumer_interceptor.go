package loki

import (
	"context"
	"errors"
)

var _ Consumer = (*InterceptorConsumer)(nil)

// InterceptorOption configures an InterceptorConsumer.
type InterceptorOption func(*InterceptorConsumer)

// WithConsumeHook hooks calls to Consume. Returning an empty batch drops it.
func WithConsumeHook(f func(ctx context.Context, batch Batch) (Batch, error)) InterceptorOption {
	return func(i *InterceptorConsumer) {
		i.onConsume = f
	}
}

// WithConsumeEntryHook hooks calls to ConsumeEntry. Returning false drops the entry.
func WithConsumeEntryHook(f func(ctx context.Context, entry Entry) (Entry, bool, error)) InterceptorOption {
	return func(i *InterceptorConsumer) {
		i.onConsumeEntry = f
	}
}

// InterceptorConsumer is a Consumer that runs hooks before forwarding to next.
type InterceptorConsumer struct {
	componentID string
	next        Consumer

	onConsume      func(ctx context.Context, batch Batch) (Batch, error)
	onConsumeEntry func(ctx context.Context, entry Entry) (Entry, bool, error)
}

// NewInterceptorConsumer creates an InterceptorConsumer. The next consumer must be non-nil.
func NewInterceptorConsumer(componentID string, next Consumer, opts ...InterceptorOption) *InterceptorConsumer {
	i := &InterceptorConsumer{
		componentID: componentID,
		next:        next,
	}

	for _, o := range opts {
		o(i)
	}

	return i
}

func (i *InterceptorConsumer) Consume(ctx context.Context, batch Batch) error {
	if i.onConsume != nil {
		batch, err := i.onConsume(ctx, batch)
		if err != nil || batch.EntryLen() == 0 {
			return err
		}
		return i.next.Consume(ctx, batch)
	}

	return errors.New("loki interceptor: unimplemented consume")
}

func (i *InterceptorConsumer) ConsumeEntry(ctx context.Context, entry Entry) error {
	if i.onConsumeEntry != nil {
		entry, keep, err := i.onConsumeEntry(ctx, entry)
		if err != nil || !keep {
			return err
		}
		return i.next.ConsumeEntry(ctx, entry)
	}

	return errors.New("loki interceptor: unimplemented consume entry")
}

func (i *InterceptorConsumer) String() string {
	return i.componentID + ".receiver"
}
