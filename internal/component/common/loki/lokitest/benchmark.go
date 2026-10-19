package lokitest

import (
	"context"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// BenchmarkWithPipelines runs a sub-benchmark once with Loki pipeline V1 and
// once with Loki pipeline V2. setup returns the receiver to send to and should
// forward to forwardTo. Each batch is sent by its own worker, and b.N is split
// across the workers. With Loki pipeline V1 the same entries are sent on every
// iteration, so the component must not mutate entry labels in place.
func BenchmarkWithPipelines(
	b *testing.B,
	batches []loki.Batch,
	setup func(b *testing.B, p Pipeline, forwardTo loki.LogsReceiver) loki.LogsReceiver,
) {

	b.Helper()
	require.NotEmpty(b, batches)

	b.Run("pipeline v1", func(b *testing.B) {
		p := Pipeline{pipelineV2: false}
		counter := newCountingReceiver(b, p)
		benchmarkV1(b, batches, setup(b, p, counter), counter)
	})

	b.Run("pipeline v2", func(b *testing.B) {
		p := Pipeline{pipelineV2: true}
		counter := newCountingReceiver(b, p)
		benchmarkV2(b, batches, setup(b, p, counter), counter)
	})
}

func benchmarkV1(b *testing.B, batches []loki.Batch, recv loki.LogsReceiver, counter *countingReceiver) {
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
				select {
				case recv.Chan() <- e:
				case <-b.Context().Done():
					return
				}
			}
		}
	})
	waitAndReport(b, batches, counter)
}

func benchmarkV2(b *testing.B, batches []loki.Batch, recv loki.LogsReceiver, counter *countingReceiver) {
	c, ok := recv.(loki.Consumer)
	require.True(b, ok, "receiver does not support Loki pipeline V2")

	b.ResetTimer()
	b.ReportAllocs()

	distribute(b.N, len(batches), func(worker, iters int) {
		for range iters {
			assert.NoError(b, c.Consume(b.Context(), batches[worker].Clone()))
		}
	})
	waitAndReport(b, batches, counter)
}

// waitAndReport waits until every sent entry has reached counter, stops the
// timer and reports entries/s. setup must forward every entry it receives.
func waitAndReport(b *testing.B, batches []loki.Batch, counter *countingReceiver) {
	var total int64
	for w, batch := range batches {
		total += int64(workerIters(b.N, len(batches), w) * batch.EntryLen())
	}

	for counter.count.Load() < total {
		runtime.Gosched()
	}
	b.StopTimer()

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

var _ loki.Consumer = (*countingReceiver)(nil)

// countingReceiver counts the entries it receives.
type countingReceiver struct {
	loki.LogsReceiver
	count atomic.Int64
}

func newCountingReceiver(b *testing.B, p Pipeline) *countingReceiver {
	r := &countingReceiver{}
	if p.pipelineV2 {
		r.LogsReceiver = loki.NewLogsReceiverV2(r, "lokitest")
		return r
	}

	r.LogsReceiver = loki.NewLogsReceiver()
	go func() {
		for {
			select {
			case <-r.Chan():
				r.count.Add(1)
			case <-b.Context().Done():
				return
			}
		}
	}()
	return r
}

func (r *countingReceiver) Consume(_ context.Context, batch loki.Batch) error {
	r.count.Add(int64(batch.EntryLen()))
	return nil
}
