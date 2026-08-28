package kubetail

import (
	"sync"
	"time"
)

// rollingAverageCalculator calculates a rolling average between points in
// time.
//
// rollingAverageCalculator stores a circular buffer where the difference
// between timestamps are kept.
type rollingAverageCalculator struct {
	mtx        sync.Mutex
	window     []time.Duration
	windowSize int

	minEntries      int
	minDuration     time.Duration
	defaultDuration time.Duration

	currentIndex  int
	prevTimestamp time.Time
}

func newRollingAverageCalculator(windowSize, minEntries int, minDuration, defaultDuration time.Duration) *rollingAverageCalculator {
	return &rollingAverageCalculator{
		windowSize:      windowSize,
		window:          make([]time.Duration, windowSize),
		minEntries:      minEntries,
		minDuration:     minDuration,
		defaultDuration: defaultDuration,
		currentIndex:    -1,
	}
}

// AddTimestamp adds a new timestamp to the rollingAverageCalculator. If there
// is a previous timestamp, the difference between timestamps is calculated and
// stored in the window.
func (r *rollingAverageCalculator) AddTimestamp(timestamp time.Time) {
	r.mtx.Lock()
	defer func() {
		r.prevTimestamp = timestamp
		r.mtx.Unlock()
	}()

	// First timestamp
	if r.currentIndex == -1 && r.prevTimestamp.Equal(time.Time{}) {
		return
	}

	r.currentIndex++
	if r.currentIndex >= r.windowSize {
		r.currentIndex = 0
	}

	r.window[r.currentIndex] = timestamp.Sub(r.prevTimestamp)
}

// GetAverage calculates the average of all the durations in the window.
func (r *rollingAverageCalculator) GetAverage() time.Duration {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	var total time.Duration
	count := 0
	for _, v := range r.window {
		if v != 0 {
			total += v
			count++
		}
	}
	if count == 0 || count < r.minEntries {
		return r.defaultDuration
	}
	d := total / time.Duration(count)
	if d < r.minDuration {
		return r.minDuration
	}
	return d
}

// GetLast gets the last timestamp added to the rollingAverageCalculator.
func (r *rollingAverageCalculator) GetLast() time.Time {
	r.mtx.Lock()
	defer r.mtx.Unlock()

	return r.prevTimestamp
}
