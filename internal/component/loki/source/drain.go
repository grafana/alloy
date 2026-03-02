package source

import (
	"context"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// Drain consumes log entries from recv in a background goroutine while f executes.
// This prevents deadlocks that can occur when stopping components that may still be
// sending entries to the receiver channel. The draining goroutine will continue
// consuming entries until f returns, at which point the context is cancelled and
// the goroutine exits.
//
// This is typically used during component shutdown to drain any remaining entries
// from a receiver channel while performing cleanup operations.
func Drain(recv loki.LogsReceiver, f func()) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
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
	}()

	f()
}
