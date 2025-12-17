package source

import (
	"context"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// Consume continuously reads log entries from recv and forwards them to the fanout f.
// It runs until ctx is cancelled or an error occurs while sending to the fanout.
//
// This function is typically used in component Run methods to handle the forwarding
// of log entries from a component's internal handler to downstream receivers.
// The fanout allows entries to be sent to multiple receivers concurrently.
func Consume(ctx context.Context, recv loki.LogsReceiver, f *loki.Fanout) {
	for {
		select {
		case <-ctx.Done():
			return
		case entry := <-recv.Chan():
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
func ConsumeBatch(ctx context.Context, recv loki.LogsBatchReceiver, f *loki.Fanout) {
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
