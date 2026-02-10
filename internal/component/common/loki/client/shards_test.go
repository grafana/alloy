package client

import (
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/loki/pkg/push"
)

var (
	entry = loki.Entry{
		Labels: model.LabelSet{"foo": "bar"},
		Entry:  push.Entry{Timestamp: time.Now(), Line: "test"},
	}
	oneEntrySize   = entry.Size()
	twoEntriesSize = entry.Size() * 2
)

func TestQueue_append(t *testing.T) {
	q := newQueue(newMetrics(prometheus.NewRegistry()), log.NewNopLogger(), Config{
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
		q := newQueue(newMetrics(prometheus.NewRegistry()), log.NewNopLogger(), Config{
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
		q := newQueue(newMetrics(prometheus.NewRegistry()), log.NewNopLogger(), Config{
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
		q := newQueue(newMetrics(prometheus.NewRegistry()), log.NewNopLogger(), Config{
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
		q := newQueue(newMetrics(prometheus.NewRegistry()), log.NewNopLogger(), Config{
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
