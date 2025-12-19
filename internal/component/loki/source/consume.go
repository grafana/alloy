package source

import (
	"context"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// ConsumeAndProccess continuously reads log entries from recv, processes them using processFn,
// and forwards the processed entries to the fanout f.
// It runs until ctx is cancelled or an error occurs while sending to the fanout.
//
// This function is typically used in component Run methods to handle the forwarding
// of log entries from a component's internal handler to downstream receivers.
// The fanout allows entries to be sent to multiple receivers concurrently.
// The processFn is applied to each entry before forwarding, allowing for transformation
// or enrichment of log entries.
func ConsumeAndProccess(
	ctx context.Context,
	recv loki.LogsReceiver,
	f *loki.Fanout,
	processFn func(e loki.Entry) loki.Entry,
) {
	consume(ctx, recv, f, processFn)
}

// Consume continuously reads log entries from recv and forwards them to the fanout f.
// It runs until ctx is cancelled or an error occurs while sending to the fanout.
//
// This function is typically used in component Run methods to handle the forwarding
// of log entries from a component's internal handler to downstream receivers.
// The fanout allows entries to be sent to multiple receivers concurrently.
func Consume(ctx context.Context, recv loki.LogsReceiver, f *loki.Fanout) {
	consume(ctx, recv, f, func(e loki.Entry) loki.Entry { return e })
}

func consume(
	ctx context.Context,
	recv loki.LogsReceiver,
	f *loki.Fanout,
	processFn func(e loki.Entry) loki.Entry,
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
