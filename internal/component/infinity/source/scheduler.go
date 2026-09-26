package source

import (
	"context"
	"math/rand/v2"
	"time"
)

type loop struct {
	cancel context.CancelFunc
	done   chan struct{}
}

// startLoop calls poll after offset, and then on each tick of interval. It
// never runs two polls at the same time. The ticker keeps one tick in its
// buffer, so after a slow poll the next poll starts at once.
func startLoop(parent context.Context, interval, offset time.Duration, poll func(context.Context), onOverrun func()) *loop {
	ctx, cancel := context.WithCancel(parent)
	l := &loop{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(l.done)

		timer := time.NewTimer(offset)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
		}

		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			start := time.Now()
			poll(ctx)
			if time.Since(start) > interval {
				onOverrun()
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
			// select picks at random when both cases are ready.
			if ctx.Err() != nil {
				return
			}
		}
	}()
	return l
}

// stop ends the loop and waits for a running poll to return.
func (l *loop) stop() {
	l.cancel()
	<-l.done
}

// randomOffset spreads the first poll of many Alloy instances over one
// interval, the same way prometheus.scrape does.
func randomOffset(interval time.Duration) time.Duration {
	return time.Duration(rand.Int64N(int64(interval)))
}
