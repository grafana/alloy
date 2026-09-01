package stages

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

const (
	plBenchGoroutines     = 10
	plBenchStreams        = 5
	plBenchPartialsPerRun = 3
)

var benchPLDrained atomic.Int64

type plBenchOp struct {
	fp model.Fingerprint
	e  Entry
}

type plBenchWorker struct {
	partials []plBenchOp
	fulls    []plBenchOp
}

func newPLBenchWorker(worker int, ts time.Time) plBenchWorker {
	labelsFor := func(stream int) model.LabelSet {
		return model.LabelSet{
			"namespace": "default",
			"pod":       model.LabelValue(fmt.Sprintf("worker-%d-stream-%d", worker, stream)),
			"container": "main",
			criStream:   "stdout",
		}
	}

	opFor := func(stream int, content string) plBenchOp {
		labels := labelsFor(stream)
		return plBenchOp{
			fp: labels.Fingerprint(),
			e:  newEntry(make(map[string]any, 4), labels, content, ts),
		}
	}

	w := plBenchWorker{
		partials: make([]plBenchOp, 0, plBenchStreams*plBenchPartialsPerRun),
		fulls:    make([]plBenchOp, 0, plBenchStreams),
	}
	for round := range plBenchPartialsPerRun {
		for stream := range plBenchStreams {
			w.partials = append(w.partials, opFor(stream, fmt.Sprintf("partial %d ", round)))
		}
	}
	for stream := range plBenchStreams {
		w.fulls = append(w.fulls, opFor(stream, "final line"))
	}
	return w
}

func newPLBenchWorkers(n int, ts time.Time) []plBenchWorker {
	out := make([]plBenchWorker, n)
	for i := range out {
		out[i] = newPLBenchWorker(i, ts)
	}
	return out
}

func newPLBenchStore(maxPartialLines int) *partialLineStore {
	return newPartialLineStore(CRIConfig{MaxPartialLines: maxPartialLines}, nil)
}

func runPLBenchWorker(s *partialLineStore, w plBenchWorker, maxPartialLines int) {
	for _, op := range w.partials {
		s.Append(op.fp, op.e)
	}
	for _, op := range w.fulls {
		s.Take(op.fp)
	}
	benchPLDrained.Add(int64(len(s.DrainIfAtLeast(maxPartialLines))))
}

func benchmarkPLStoreConcurrent(b *testing.B, maxPartialLines int) {
	workers := newPLBenchWorkers(plBenchGoroutines, time.Now())
	s := newPLBenchStore(maxPartialLines)

	b.ReportAllocs()
	b.ResetTimer()

	var wg sync.WaitGroup
	for _, w := range workers {
		wg.Add(1)
		go func(w plBenchWorker) {
			defer wg.Done()
			for range b.N {
				runPLBenchWorker(s, w, maxPartialLines)
			}
		}(w)
	}
	wg.Wait()

	b.StopTimer()
	require.Zero(b, s.Size())
}

func BenchmarkPartialLineStoreConcurrent(b *testing.B) {
	benchmarkPLStoreConcurrent(b, 4)
}

func BenchmarkPartialLineStoreConcurrentNoFlush(b *testing.B) {
	benchmarkPLStoreConcurrent(b, 100)
}

func BenchmarkPartialLineStoreSerial(b *testing.B) {
	const maxPartialLines = 100

	w := newPLBenchWorker(0, time.Now())
	s := newPLBenchStore(maxPartialLines)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		runPLBenchWorker(s, w, maxPartialLines)
	}

	b.StopTimer()
	require.Zero(b, s.Size())
}
