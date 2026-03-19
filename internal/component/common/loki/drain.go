package loki

import (
	"context"
	"sync"
	"time"
)

const DefaultDrainTimeout = 2 * time.Minute

// Drain forwards log entries from recv to fanout in a background goroutine while
// fn executes. If forwarding blocks for longer than timeout, Drain falls back
// to discarding entries from recv until fn returns. This prevents deadlocks in
// shutdown paths where component may still send to recv while fn is stopping them.
//
// This is typically used during component shutdown to drain any remaining entries
// from a receiver channel while performing cleanup operations.
func Drain(recv LogsReceiver, fanout *Fanout, timeout time.Duration, fn func()) {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	defer func() {
		cancel()
		wg.Wait()
	}()

	wg.Go(func() {
		consumeCtx, consumeCancel := context.WithTimeout(ctx, timeout)
		Consume(consumeCtx, recv, fanout)
		consumeCancel()

		// NOTE: If we could not forward entries within fallbackDuration we drain to nothing.
		// This is just to gaurd against deadlock. If/when fn finish sucessfully this will stop.
		discard(ctx, recv)
	})

	fn()
}

func discard(ctx context.Context, recv LogsReceiver) {
	for {
		select {
		case <-ctx.Done():
			return
		case _, ok := <-recv.Chan():
			// Consume and discard entries to prevent channel blocking
			if !ok {
				return
			}
		}
	}
}
