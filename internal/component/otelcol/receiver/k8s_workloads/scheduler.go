package k8s_workloads

import (
	"context"
	"time"
)

// scheduleScans waits for each scan to complete before scheduling another one.
// Only scan scheduling pauses; the delivery worker can process notifications.
func scheduleScans(ctx context.Context, args SnapshotArguments, enqueue func(), completed <-chan time.Duration) {
	for {
		if ctx.Err() != nil {
			return
		}
		enqueue()
		var duration time.Duration
		select {
		case <-ctx.Done():
			return
		case duration = <-completed:
		}
		timer := time.NewTimer(max(args.Interval-duration, args.MinIntervalAfterScan))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}
