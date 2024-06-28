package jitter

import (
	"math/rand"
	"sync"
	"time"
)

type Ticker struct {
	C     <-chan time.Time
	stop  chan struct{}
	reset chan struct{}

	mut sync.RWMutex
	d   time.Duration
	j   time.Duration
}

// NewTicker creates a Ticker that works similar to time.Ticker, but sends the
// time with a period specified by `duration` adjusted by a pseudorandom jitter
// in the range of [duration-jitter, duration+jitter).
// Following the behavior of time.Ticker, we use a 1-buffer channel, so if the
// client falls behind while reading, we'll drop ticks on the floor until the
// client catches up.
// Callers have to make sure that both duration and the [d-j, d+j) intervals
// are valid positive int64 values (non-negative and non-overflowing).
// Use Stop to release associated resources and the Reset methods to modify the
// duration and jitter.
func NewTicker(duration time.Duration, jitter time.Duration) *Ticker {
	ticker := time.NewTicker(duration)
	c := make(chan time.Time, 1)
	t := &Ticker{
		C: c,

		stop:  make(chan struct{}),
		reset: make(chan struct{}),
		d:     duration,
		j:     jitter,
	}

	go func() {
		for {
			select {
			case tc := <-ticker.C:
				ticker.Reset(t.getNextPeriod())
				select {
				case c <- tc:
				default:
				}
			case <-t.stop:
				ticker.Stop()
				return
			case <-t.reset:
				ticker.Reset(t.getNextPeriod())
			}
		}
	}()
	return t
}

// Stop turns off the Ticker; no more ticks will be sent. Stop does not close
// Ticker's channel, to prevent a concurrent goroutine from seeing an erroneous
// "tick".
func (t *Ticker) Stop() {
	close(t.reset)
	close(t.stop)
}

// Reset stops the Ticker, resets its base duration to the specified argument
// and re-calculates the period with a jitter.
// The next tick will arrive after the new period elapses.
func (t *Ticker) Reset(d time.Duration) {
	t.mut.Lock()
	t.d = d
	t.mut.Unlock()
	t.reset <- struct{}{}
}

// Reset stops the Ticker, resets its jitter to the specified argument and
// re-calculates the period with the new jitter.
// The next tick will arrive after the new period elapses.
func (t *Ticker) ResetJitter(d time.Duration) {
	t.mut.Lock()
	t.j = d
	t.mut.Unlock()
	t.reset <- struct{}{}
}

// getNextPeriod is used to calculate the period for the Ticker.
func (t *Ticker) getNextPeriod() time.Duration {
	// jitter is a random value between [0, 2j)
	// the returned period is then d-j + jitter
	// which results in [d-j, d+j).
	t.mut.RLock()
	jitter := rand.Int63n(2 * int64(t.j))
	period := t.d - t.j + time.Duration(jitter)
	t.mut.RUnlock()

	return period
}
