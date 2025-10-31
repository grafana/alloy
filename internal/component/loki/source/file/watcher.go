package file

import (
	"context"
	"sync"
	"time"
)

func newWatcher(tick time.Duration) *watcher {
	if tick == 0 {
		tick = 10 * time.Second
	}

	return &watcher{
		tick:   tick,
		ticker: *time.NewTicker(tick),
	}
}

type watcher struct {
	mu     sync.Mutex
	tick   time.Duration
	ticker time.Ticker
	syncFn func()
}

// Run executes the watch loop which periodically invokes the configured
// sync function. The loop terminates when the provided context is canceled.
func (w *watcher) Run(ctx context.Context) {
	defer w.ticker.Stop()

	select {
	case <-w.ticker.C:
		w.mu.Lock()
		w.syncFn()
		w.mu.Unlock()
	case <-ctx.Done():
		return
	}
}

// Update configures the watcher with a new interval and sync function.
// If the interval changes, the internal ticker is reset without restarting
// the goroutine. The provided syncFn is called on each tick.
func (w *watcher) Update(tick time.Duration, syncFn func()) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.syncFn = syncFn
	if w.tick != tick && tick != 0 {
		w.ticker.Reset(tick)
	}
}
