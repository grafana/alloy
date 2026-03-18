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
	consume(ctx, recv, f, func(e Entry) Entry { return e })
}

// ConsumeAndProcess continuously reads log entries from recv, processes them using processFn,
// and forwards the processed entries to the fanout f.
// It runs until ctx is cancelled or an error occurs while sending to the fanout.
//
// This function is typically used in component Run methods to handle the forwarding
// of log entries from a component's internal handler to downstream receivers.
// The fanout allows entries to be sent to multiple receivers concurrently.
// The processFn is applied to each entry before forwarding, allowing for transformation
// or enrichment of log entries.
func ConsumeAndProcess(
	ctx context.Context,
	recv LogsReceiver,
	f *Fanout,
	processFn func(e Entry) Entry,
) {

	consume(ctx, recv, f, processFn)
}

func consume(
	ctx context.Context,
	recv LogsReceiver,
	f *Fanout,
	processFn func(e Entry) Entry,
) {

	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-recv.Chan():
			// NOTE: the only error we can get is context.Canceled.
			if err := f.Send(ctx, processFn(entry)); err != nil {
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
