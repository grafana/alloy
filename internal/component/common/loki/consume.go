package loki

import (
	"context"
)

// ConsumeBatch continuously reads batches of log entries from recv and forwards them to the fanout f.
// It runs until ctx is cancelled or an error occurs while sending to the fanout.
//
// This function is typically used in component Run methods to handle the forwarding
// of log entries from a component's internal handler to downstream receivers.
// The fanout allows entries to be sent to multiple receivers concurrently.
func ConsumeBatch(ctx context.Context, recv LogsBatchReceiver, f *FanoutConsumer) {
	for {
		// NOTE: Select is not deterministic so we should check if context was canceled
		// before we start waiting on channel again.
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ctx.Done():
			return
		case batch := <-recv.Chan():
			for _, e := range batch {
				// NOTE: For now we ignore errors here.
				_ = f.ConsumeEntry(ctx, e)
			}
		}
	}
}

// Consume continuously reads log entries from recv and forwards them to the fanout f.
// It runs until ctx is cancelled.
//
// This function is typically used in component Run methods to handle the forwarding
// of log entries from a component's internal handler to downstream receivers.
// The fanout allows entries to be sent to multiple receivers concurrently.
func Consume(ctx context.Context, recv LogsReceiver, f *FanoutConsumer) {
	consume(ctx, recv, f, func(e Entry) (Entry, bool) { return e, true })
}

// ConsumeAndProcess continuously reads log entries from recv, processes them using processFn,
// and forwards the processed entries to the fanout f. It runs until ctx is cancelled.
//
// This function is typically used in component Run methods to handle the forwarding
// of log entries from a component's internal handler to downstream receivers.
// The fanout allows entries to be sent to multiple receivers concurrently.
// The processFn is applied to each entry before forwarding, allowing for transformation
// or enrichment of log entries. If processFn returns false, the entry is dropped.
func ConsumeAndProcess(
	ctx context.Context,
	recv LogsReceiver,
	f *FanoutConsumer,
	processFn func(e Entry) (Entry, bool),
) {

	consume(ctx, recv, f, processFn)
}

func consume(
	ctx context.Context,
	recv LogsReceiver,
	f *FanoutConsumer,
	processFn func(e Entry) (Entry, bool),
) {

	for {
		// NOTE: Select is not deterministic so we should check if context was canceled
		// before we start waiting on channel again.
		select {
		case <-ctx.Done():
			return
		default:
		}

		select {
		case <-ctx.Done():
			return
		case entry := <-recv.Chan():
			entry, ok := processFn(entry)
			if !ok {
				continue
			}

			// NOTE: For now we ignore errors here.
			if err := f.ConsumeEntry(ctx, entry); err != nil {
				continue
			}
		}
	}
}
