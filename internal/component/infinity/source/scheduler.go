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

// startLoop calls poll after offset, and then on each tick of interval. A
// value on kick makes the loop poll after a jitter below interval/10. It
// never runs two polls at the same time. The ticker keeps one tick in its
// buffer, and kick keeps one value, so a tick or kick that comes during a
// slow poll gives one more poll after it.
func startLoop(parent context.Context, interval, offset time.Duration, kick <-chan struct{}, poll func(context.Context), onOverrun func()) *loop {
	ctx, cancel := context.WithCancel(parent)
	l := &loop{cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(l.done)

		first := time.NewTimer(offset)
		defer first.Stop()
		firstC := first.C
		var (
			ticker *time.Ticker
			tickC  <-chan time.Time
			soon   *time.Timer
			soonC  <-chan time.Time
		)
		// The deferred calls stop the timers, so a loop that stops leaves no
		// pending jitter.
		defer func() {
			if ticker != nil {
				ticker.Stop()
			}
			if soon != nil {
				soon.Stop()
			}
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case <-kick:
				if soon == nil {
					soon = time.NewTimer(jitter(interval))
					soonC = soon.C
				}
				continue
			case <-firstC:
			case <-tickC:
			case <-soonC:
				// Move the next tick, so it does not come just after this poll.
				if ticker != nil {
					ticker.Reset(interval)
				}
			}
			// select picks at random when more than one case is ready.
			if ctx.Err() != nil {
				return
			}
			// This poll also serves a pending kick.
			if soon != nil {
				soon.Stop()
				soon, soonC = nil, nil
			}
			if ticker == nil {
				first.Stop()
				firstC = nil
				ticker = time.NewTicker(interval)
				tickC = ticker.C
			}

			start := time.Now()
			poll(ctx)
			if time.Since(start) > interval {
				onOverrun()
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

// jitter spreads the polls of the queries that one cluster change moves to
// this node.
func jitter(interval time.Duration) time.Duration {
	if interval/10 <= 0 {
		return 0
	}
	return randomOffset(interval / 10)
}
