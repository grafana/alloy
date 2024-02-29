package wal

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	maxOvershoot = 200 * time.Millisecond
)

func TestBackoffTimer(t *testing.T) {
	var min = time.Millisecond * 300
	var max = time.Second
	timer := newBackoffTimer(min, max)

	start := time.Now()
	<-timer.C
	verifyElapsedTime(t, min, time.Since(start))

	// backoff, and expect it will take twice the time
	start = time.Now()
	timer.backoff()
	<-timer.C
	verifyElapsedTime(t, 2*min, time.Since(start))

	// backoff capped, backoff will actually be 1200ms, but capped at 1000
	start = time.Now()
	timer.backoff()
	<-timer.C
	verifyElapsedTime(t, max, time.Since(start))
}

func verifyElapsedTime(t *testing.T, expected time.Duration, elapsed time.Duration) {
	require.GreaterOrEqual(t, elapsed, expected, "elapsed time should be greater or equal to the expected value")
	require.Less(t, elapsed, expected+maxOvershoot, "elapsed time should be less than the expected value plus the max overshoot")
}
