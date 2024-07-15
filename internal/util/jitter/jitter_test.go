package jitter

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Inspired by a test on github.com/mroth/jitter
func TestTicker(t *testing.T) {
	var (
		d     = 100 * time.Millisecond
		j     = 5 * time.Millisecond
		n     = 5
		delta = 100 * time.Millisecond
		min   = time.Duration(math.Floor(float64(d)-float64(j)))*time.Duration(n) - delta
		max   = time.Duration(math.Ceil(float64(d)+float64(j)))*time.Duration(n) + delta
	)

	// Check that the time required for N ticks is within expected range.
	ticker := NewTicker(d, j)
	start := time.Now()
	for i := 0; i < n; i++ {
		<-ticker.C
	}

	elapsed := time.Since(start)
	if elapsed < min || elapsed > max {
		require.Fail(t, "ticker didn't meet timing criteria", "time needed for %d ticks %v outside of expected range [%v - %v]", n, elapsed, min, max)
	}
}

func TestTickerStop(t *testing.T) {
	t.Parallel()

	var (
		d      = 50 * time.Millisecond
		j      = 10 * time.Millisecond
		before = 3      // ticks before stop
		wait   = d * 10 // monitor after stop
	)

	ticker := NewTicker(d, j)
	for i := 0; i < before; i++ {
		<-ticker.C
	}

	ticker.Stop()
	select {
	case <-ticker.C:
		require.Fail(t, "Got tick after Stop()")
	case <-time.After(wait):
	}
}

func TestTickerReset(t *testing.T) {
	var (
		d1    = 50 * time.Millisecond
		d2    = 100 * time.Millisecond
		j1    = 3 * time.Millisecond
		j2    = 9 * time.Millisecond
		n     = 5
		delta = 100 * time.Millisecond
		min1  = time.Duration(math.Floor(float64(d1)-float64(j1)))*time.Duration(n) - delta
		max1  = time.Duration(math.Ceil(float64(d1)+float64(j1)))*time.Duration(n) + delta
		min2  = time.Duration(math.Floor(float64(d2)-float64(j2)))*time.Duration(n) - delta
		max2  = time.Duration(math.Ceil(float64(d2)+float64(j2)))*time.Duration(n) + delta
	)

	// Check that the time required for N ticks is within expected range.
	ticker := NewTicker(d1, j1)
	start := time.Now()
	for i := 0; i < n; i++ {
		<-ticker.C
	}
	ticker.Reset(d2)
	ticker.ResetJitter(j2)
	for i := 0; i < n; i++ {
		<-ticker.C
	}

	elapsed := time.Since(start)
	if elapsed < (min1+min2) || elapsed > (max1+max2) {
		require.Fail(t, "ticker didn't meet timing criteria", "time needed for %d ticks %v outside of expected range [%v - %v]", n, elapsed, (min1 + min2), (max1 + max2))
	}
}
