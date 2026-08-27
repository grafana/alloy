package loki

import (
	"context"
)

var _ Consumer = (*InterceptorConsumer)(nil)

// InterceptorConsumer is a Consumer that runs callback before forwarding to next.
type InterceptorConsumer struct {
	componentID string
	next        Consumer

	onConsume func(ctx context.Context, batch Batch) (Batch, error)
}

// NewInterceptorConsumer creates an InterceptorConsumer. The next consumer and fn must be non-nil.
func NewInterceptorConsumer(componentID string, next Consumer, fn func(ctx context.Context, batch Batch) (Batch, error)) *InterceptorConsumer {
	return &InterceptorConsumer{
		componentID: componentID,
		next:        next,
		onConsume:   fn,
	}
}

func (i *InterceptorConsumer) Consume(ctx context.Context, batch Batch) error {
	batch, err := i.onConsume(ctx, batch)
	if err != nil || batch.EntryLen() == 0 {
		return err
	}
	return i.next.Consume(ctx, batch)
}

func (i *InterceptorConsumer) String() string {
	return i.componentID + ".receiver"
}
