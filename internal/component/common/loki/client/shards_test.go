package client

import (
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/logging"
)

var (
	entry = loki.Entry{
		Labels: model.LabelSet{"foo": "bar"},
		Entry:  push.Entry{Timestamp: time.Now(), Line: "test"},
	}
	newStreamSize  = entry.Size() + labelSetSize(entry.Labels)
	oneEntrySize   = newStreamSize
	twoEntriesSize = newStreamSize + entry.Size()
)

func TestQueue_append(t *testing.T) {
	q := newQueue(newMetrics(prometheus.NewRegistry()), logging.NewSlogNop(), Config{
		BatchSize: twoEntriesSize,
	})

	// add 2 entries to the queue
	for range 2 {
		queued := q.append("tenant-1", entry, 0)
		assert.True(t, queued)
	}
	assert.Equal(t, twoEntriesSize, q.batches["tenant-1"].size)

	// add two more entries, the current batch should be queued and a new batch should be created.
	for range 2 {
		queued := q.append("tenant-1", entry, 0)
		assert.True(t, queued)
	}
	assert.Equal(t, twoEntriesSize, q.batches["tenant-1"].size)

	// adding one more should fail because both queue and batch is full
	queued := q.append("tenant-1", entry, 0)
	assert.False(t, queued)

	// dequeue current batch.
	<-q.channel()

	// add batch again.
	queued = q.append("tenant-1", entry, 0)
	assert.True(t, queued)
	assert.Equal(t, oneEntrySize, q.batches["tenant-1"].size)
}

func TestQueue_drain(t *testing.T) {
	t.Run("should drain queue and current batch", func(t *testing.T) {
		// a queue with batches that will fit two entries and only one batch can queued at any given time.
		q := newQueue(newMetrics(prometheus.NewRegistry()), logging.NewSlogNop(), Config{
			BatchSize: twoEntriesSize,
		})

		// fill up queue and current batch
		for range 4 {
			queued := q.append("tenant-1", entry, 0)
			assert.True(t, queued)
		}
		assert.Equal(t, q.batches["tenant-1"].size, twoEntriesSize)

		batches := q.drain()
		// We should drain queued batch and batch stored in memory
		assert.Len(t, batches, 2)
	})

	t.Run("should only drain queue", func(t *testing.T) {
		// a queue with batches that will fit two entries and only one batch can queued at any given time.
		q := newQueue(newMetrics(prometheus.NewRegistry()), logging.NewSlogNop(), Config{
			BatchSize: twoEntriesSize,
			BatchWait: 10 * time.Second,
		})

		// fill up queue and current batch
		for range 4 {
			queued := q.append("tenant-1", entry, 0)
			assert.True(t, queued)
		}
		assert.Equal(t, q.batches["tenant-1"].size, twoEntriesSize)

		batches := q.drain()
		// We should drain queued batch and batch stored in memory
		assert.Len(t, batches, 1)
	})
}

func TestQueue_flushAndShutdown(t *testing.T) {
	t.Run("should flush all batches to queue", func(t *testing.T) {
		// a queue with batches that will fit two entries and only one batch can queued at any given time.
		q := newQueue(newMetrics(prometheus.NewRegistry()), logging.NewSlogNop(), Config{
			BatchSize: twoEntriesSize,
		})

		// fill current batch but don't queue it.
		for range 2 {
			queued := q.append("tenant-1", entry, 0)
			assert.True(t, queued)
		}
		assert.Equal(t, q.batches["tenant-1"].size, twoEntriesSize)

		var wg sync.WaitGroup

		wg.Go(func() {
			done := make(chan struct{})
			defer close(done)
			q.flushAndShutdown(done)
		})

		wg.Go(func() {
			var batches []queuedBatch
			for {
				b, ok := <-q.channel()
				if !ok {
					break
				}
				batches = append(batches, b)
			}
			assert.Len(t, batches, 1)
		})
		wg.Wait()
	})

	t.Run("should stop early if done channel is closed", func(t *testing.T) {
		// a queue with batches that will fit two entries and only one batch can queued at any given time.
		q := newQueue(newMetrics(prometheus.NewRegistry()), logging.NewSlogNop(), Config{
			BatchSize: twoEntriesSize,
		})

		// fill current batch but don't queue it.
		for range 4 {
			queued := q.append("tenant-1", entry, 0)
			assert.True(t, queued)
		}

		// Create and immediately close the done channel.
		done := make(chan struct{})
		close(done)

		// Flush and shutdown - should stop early when done channel is signaled.
		q.flushAndShutdown(done)

		// Verify batches map is nil.
		assert.Nil(t, q.batches)

		// First batch should already be in queue.
		_, ok := <-q.channel()
		assert.True(t, ok)

		// Second batch should not have been queued
		_, ok = <-q.channel()
		assert.False(t, ok)
	})
}

type recordingTracker struct {
	mut      sync.Mutex
	recorded []sentData
}

func (t *recordingTracker) UpdateSentData(segment, count int) {
	t.mut.Lock()
	defer t.mut.Unlock()
	t.recorded = append(t.recorded, sentData{segment: segment, count: count})
}

func (t *recordingTracker) sent() []sentData {
	t.mut.Lock()
	defer t.mut.Unlock()
	return slices.Clone(t.recorded)
}

type sentData struct {
	segment int
	count   int
}

func TestShards(t *testing.T) {
	t.Run("should not update marker when in-flight requests are canceled due to hard shutdown", func(t *testing.T) {
		server, blocked, release := newBlockedServer()
		defer server.Close()
		defer release()

		var url flagext.URLValue
		require.NoError(t, url.Set(server.URL))

		tracker := &recordingTracker{}

		s, err := newShards(newMetrics(prometheus.NewRegistry()), logging.NewSlogNop(), tracker, Config{
			URL:       url,
			BatchSize: 1,
			Timeout:   time.Minute,
			Client:    config.DefaultHTTPClientConfig,
			BackoffConfig: backoff.Config{
				MinBackoff: time.Millisecond,
				MaxBackoff: 10 * time.Millisecond,
			},
			QueueConfig: QueueConfig{
				Capacity:     1,
				DrainTimeout: 100 * time.Millisecond,
			},
		})
		require.NoError(t, err)
		s.start(1)

		require.NoError(t, s.enqueue("", entry, 1))
		require.Eventually(t, blocked.Load, 5*time.Second, 10*time.Millisecond)

		s.stop()

		require.Len(t, tracker.sent(), 0)
	})
}
