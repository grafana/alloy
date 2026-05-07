package loki

import (
	"context"
)

// Consume continuously reads log entries from recv and forwards them to the fanout f.
// It runs until ctx is cancelled or an error occurs while sending to the fanout.
//
// This function is typically used in component Run methods to handle the forwarding
// of log entries from a component's internal handler to downstream receivers.
// The fanout allows entries to be sent to multiple receivers concurrently.
func Consume(ctx context.Context, recv LogsReceiver, f *Fanout) {
	consume(ctx, recv, f, func(e Entry) (Entry, bool) { return e, true })
}

// ConsumeAndProcess continuously reads log entries from recv, processes them using processFn,
// and forwards the processed entries to the fanout f.
// It runs until ctx is cancelled or an error occurs while sending to the fanout.
//
// This function is typically used in component Run methods to handle the forwarding
// of log entries from a component's internal handler to downstream receivers.
// The fanout allows entries to be sent to multiple receivers concurrently.
// The processFn is applied to each entry before forwarding, allowing for transformation
// or enrichment of log entries. If processFn returns false, the entry is dropped.
func ConsumeAndProcess(
	ctx context.Context,
	recv LogsReceiver,
	f *Fanout,
	processFn func(e Entry) (Entry, bool),
) {

	consume(ctx, recv, f, processFn)
}

func consume(
	ctx context.Context,
	recv LogsReceiver,
	f *Fanout,
	processFn func(e Entry) (Entry, bool),
) {

	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-recv.Chan():
			entry, ok := processFn(entry)
			if !ok {
				continue
			}
			// NOTE: the only error we can get is context.Canceled.
			if err := f.Send(ctx, entry); err != nil {
				return
			}
		}
	}
}

// ConsumeBatch continuously reads batches of log entries from recv and forwards them to the fanout f.
// It runs until ctx is cancelled or an error occurs while sending to the fanout.
//
// This function is typically used in component Run methods to handle the forwarding
// of log entries from a component's internal handler to downstream receivers.
// The fanout allows entries to be sent to multiple receivers concurrently.
func ConsumeBatch(ctx context.Context, recv LogsBatchReceiver, f *Fanout) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch := <-recv.Chan():
			// NOTE: the only error we can get is context.Canceled.
			if err := f.SendBatch(ctx, batch); err != nil {
				return
			}
		}
	}
}

// FIXME: Rename ConsumeBatch2 to ConsumeBatch and remove the old ConsumeBatch once all
// components have migrated to Consumer.
// ConsumeBatch2 continuously reads batches of log entries from recv and forwards them to the fanout f.
// It runs until ctx is cancelled or an error occurs while sending to the fanout.
//
// This function is typically used in component Run methods to handle the forwarding
// of log entries from a component's internal handler to downstream receivers.
// The fanout allows entries to be sent to multiple receivers concurrently.
func ConsumeBatch2(ctx context.Context, recv LogsBatchReceiver, f *FanoutConsumer) {
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

// FIXME: Rename Consume2 to Consume and remove the old Consume once all
// components have migrated to Consumer.
// Consume2 continuously reads log entries from recv and forwards them to the fanout f.
// It runs until ctx is cancelled.
//
// This function is typically used in component Run methods to handle the forwarding
// of log entries from a component's internal handler to downstream receivers.
// The fanout allows entries to be sent to multiple receivers concurrently.
func Consume2(ctx context.Context, recv LogsReceiver, f *FanoutConsumer) {
	consume2(ctx, recv, f, func(e Entry) (Entry, bool) { return e, true })
}

// FIXME: Rename ConsumeAndProcess2 to ConsumeAndProcess and remove the old
// ConsumeAndProcess once all components have migrated to Consumer.
// ConsumeAndProcess2 continuously reads log entries from recv, processes them using processFn,
// and forwards the processed entries to the fanout f. It runs until ctx is cancelled.
//
// This function is typically used in component Run methods to handle the forwarding
// of log entries from a component's internal handler to downstream receivers.
// The fanout allows entries to be sent to multiple receivers concurrently.
// The processFn is applied to each entry before forwarding, allowing for transformation
// or enrichment of log entries. If processFn returns false, the entry is dropped.
func ConsumeAndProcess2(
	ctx context.Context,
	recv LogsReceiver,
	f *FanoutConsumer,
	processFn func(e Entry) (Entry, bool),
) {

	consume2(ctx, recv, f, processFn)
}

func consume2(
	ctx context.Context,
	recv LogsReceiver,
	f *FanoutConsumer,
	processFn func(e Entry) (Entry, bool),
) {

	for {
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
