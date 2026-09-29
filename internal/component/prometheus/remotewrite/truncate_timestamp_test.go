package remotewrite

import (
	"testing"
	"time"

	"github.com/prometheus/prometheus/model/timestamp"
	"github.com/stretchr/testify/require"
)

func TestTruncateTimestamp(t *testing.T) {
	minWALTime := 5 * time.Minute
	maxWALTime := 8 * time.Hour
	now := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)

	t.Run("normal lowest sent near now", func(t *testing.T) {
		lowestSent := timestamp.FromTime(now.Add(-time.Minute))
		ts, clampedFuture := truncateTimestamp(lowestSent, minWALTime, maxWALTime, now)
		require.False(t, clampedFuture)
		require.Equal(t, lowestSent-minWALTime.Milliseconds(), ts)
	})

	t.Run("too-old lowest sent is floored to now-maxWALTime", func(t *testing.T) {
		lowestSent := timestamp.FromTime(now.Add(-48 * time.Hour))
		ts, clampedFuture := truncateTimestamp(lowestSent, minWALTime, maxWALTime, now)
		require.False(t, clampedFuture)
		require.Equal(t, timestamp.FromTime(now.Add(-maxWALTime)), ts)
	})

	t.Run("future lowest sent is capped to now-minWALTime", func(t *testing.T) {
		// Mimic a ~79-day clock skew that latched LowestSentTimestamp far ahead.
		lowestSent := timestamp.FromTime(now.Add(79 * 24 * time.Hour))
		ts, clampedFuture := truncateTimestamp(lowestSent, minWALTime, maxWALTime, now)
		require.True(t, clampedFuture)
		require.Equal(t, timestamp.FromTime(now.Add(-minWALTime)), ts)
	})

	t.Run("zero lowest sent stays non-negative before floor", func(t *testing.T) {
		ts, clampedFuture := truncateTimestamp(0, minWALTime, maxWALTime, now)
		require.False(t, clampedFuture)
		// 0 - minWALTime would be negative, then floored to now-maxWALTime.
		require.Equal(t, timestamp.FromTime(now.Add(-maxWALTime)), ts)
	})
}
