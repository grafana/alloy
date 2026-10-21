package lokitest

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// BenchmarkSink runs a sub-benchmark for V1 and V2 Loki pipelines, with an Env set up for
// that pipeline. setup returns the receiver to send to and must call delivered with
// the number of entries the component delivered.
func BenchmarkSink(
	b *testing.B,
	batches []loki.Batch,
	setup func(b *testing.B, env Env, delivered func(n int)) loki.LogsReceiver,
) {

	b.Helper()
	require.NotEmpty(b, batches)

	b.Run("pipeline v1", func(b *testing.B) {
		env := Env{pipelineV2: false}
		counter := &atomic.Int64{}
		benchmarkV1(b, batches, setup(b, env, func(n int) { counter.Add(int64(n)) }), counter)
	})

	b.Run("pipeline v2", func(b *testing.B) {
		env := Env{pipelineV2: true}
		counter := &atomic.Int64{}
		benchmarkV2(b, batches, setup(b, env, func(n int) { counter.Add(int64(n)) }), counter)
	})
}

func benchmarkV1(b *testing.B, batches []loki.Batch, recv loki.LogsReceiver, counter *atomic.Int64) {
	workerEntries := make([][]loki.Entry, len(batches))
	for w, batch := range batches {
		for _, stream := range batch.Streams() {
			for _, e := range stream.Entries {
				workerEntries[w] = append(workerEntries[w], loki.NewEntryWithCreatedUnixMicro(stream.Labels, stream.Created(), e))
			}
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	distribute(b.N, len(batches), func(worker, iters int) {
		for range iters {
			for _, e := range workerEntries[worker] {
				// The receiver may modify the entry, so each send needs its own copy.
				// It also matches V2, where each batch must be cloned.
				select {
				case recv.Chan() <- e.Clone():
				case <-b.Context().Done():
					return
				}
			}
		}
	})
	waitAndReport(b, batches, counter)
}

func benchmarkV2(b *testing.B, batches []loki.Batch, recv loki.LogsReceiver, counter *atomic.Int64) {
	c, ok := recv.(loki.Consumer)
	require.True(b, ok, "receiver does not support Loki pipeline V2")

	b.ResetTimer()
	b.ReportAllocs()

	distribute(b.N, len(batches), func(worker, iters int) {
		for range iters {
			// Consume may modify the batch, so each call needs its own copy.
			assert.NoError(b, c.Consume(b.Context(), batches[worker].Clone()))
		}
	})
	waitAndReport(b, batches, counter)
}

// waitAndReport stops the timer, waits until counter has counted every sent
// entry and reports entries/s.
func waitAndReport(b *testing.B, batches []loki.Batch, counter *atomic.Int64) {
	var total int64
	for w, batch := range batches {
		total += int64(workerIters(b.N, len(batches), w) * batch.EntryLen())
	}

	// Delivery is not part of the measurement.
	b.StopTimer()
	require.EventuallyWithT(b, func(c *assert.CollectT) {
		assert.Equal(c, total, counter.Load())
	}, time.Minute, 10*time.Millisecond)

	b.ReportMetric(float64(total)/b.Elapsed().Seconds(), "entries/s")
}

// distribute splits n iterations across workers and runs them concurrently.
func distribute(n, workers int, work func(worker, iters int)) {
	var wg sync.WaitGroup
	for w := range workers {
		wg.Go(func() {
			work(w, workerIters(n, workers, w))
		})
	}
	wg.Wait()
}

// workerIters returns how many of n iterations worker runs.
func workerIters(n, workers, worker int) int {
	iters := n / workers
	if worker < n%workers {
		iters++
	}
	return iters
}
