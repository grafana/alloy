package stages

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/runtime/logging"
)

const criTestTS = "2019-05-07T18:57:50.904275087+00:00"

type criBenchLine struct {
	labels model.LabelSet
	line   string
}

var benchCRISink []Entry

func criBenchLabels(stream int) model.LabelSet {
	return model.LabelSet{
		"namespace": "default",
		"pod":       model.LabelValue(fmt.Sprintf("app-%d", stream)),
		"container": "main",
	}
}

func generateCRIPartialThenFull(streams, partialsPerStream int) []criBenchLine {
	out := make([]criBenchLine, 0, streams*(partialsPerStream+1))
	for round := range partialsPerStream + 1 {
		for s := range streams {
			flag, content := "P", fmt.Sprintf("partial %d of stream %d ", round, s)
			if round == partialsPerStream {
				flag, content = "F", fmt.Sprintf("final line of stream %d", s)
			}
			out = append(out, criBenchLine{
				labels: criBenchLabels(s),
				line:   fmt.Sprintf("%s stdout %s %s", criTestTS, flag, content),
			})
		}
	}
	return out
}

func generateCRIPartialsOnly(streams int) []criBenchLine {
	out := make([]criBenchLine, 0, streams)
	for s := range streams {
		out = append(out, criBenchLine{
			labels: criBenchLabels(s),
			line:   fmt.Sprintf("%s stdout P partial line of stream %d ", criTestTS, s),
		})
	}
	return out
}

func generateCRIMergingWorkerBatch(worker, run, streams, partialsPerStream int) []criBenchLine {
	out := make([]criBenchLine, 0, streams*(partialsPerStream+1))
	for round := range partialsPerStream + 1 {
		for st := range streams {
			flag, content := "P", fmt.Sprintf("partial %d ", round)
			if round == partialsPerStream {
				flag, content = "F", "final line"
			}
			out = append(out, criBenchLine{
				labels: model.LabelSet{
					"namespace": "default",
					"pod":       model.LabelValue(fmt.Sprintf("worker-%d-run-%d-stream-%d", worker, run, st)),
					"container": "main",
				},
				line: fmt.Sprintf("%s stdout %s %s", criTestTS, flag, content),
			})
		}
	}
	return out
}

func buildCRIBenchEntries(src []criBenchLine, ts time.Time) []Entry {
	out := make([]Entry, len(src))
	for i, s := range src {
		out[i] = newEntry(make(map[string]any, 4), s.labels.Clone(), s.line, ts)
	}
	return out
}

func newCRIBenchStage(b *testing.B, cfg CRIConfig, next nextFn) *criStage {
	b.Helper()
	return newCRIStage(cfg, stageOpts{
		slogger:    logging.NewSlogNop(),
		registerer: prometheus.NewRegistry(),
		next:       next,
	})
}

func BenchmarkCRIStageProcessHappyPath(b *testing.B) {
	const (
		streams           = 25
		partialsPerStream = 9
	)

	src := generateCRIPartialThenFull(streams, partialsPerStream)
	pristine := buildCRIBenchEntries(src, time.Now())
	work := make([]Entry, len(pristine))

	forwarded := 0
	c := newCRIBenchStage(b, CRIConfig{MaxPartialLines: streams * 10},
		func(_ context.Context, entries []Entry) error {
			benchCRISink = entries
			forwarded += len(entries)
			return nil
		})
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		copy(work, pristine)
		if err := c.process(ctx, work); err != nil {
			b.Fatal(err)
		}
	}

	b.StopTimer()
	require.Zero(b, c.partialLines.Size())
	require.Equal(b, streams*b.N, forwarded)
}

func BenchmarkCRIStageProcessMaxPartialLinesFlush(b *testing.B) {
	const (
		streams         = 250
		maxPartialLines = 25
	)

	src := generateCRIPartialsOnly(streams)
	pristine := buildCRIBenchEntries(src, time.Now())
	work := make([]Entry, len(pristine))

	flushedEntries := 0
	c := newCRIBenchStage(b, CRIConfig{MaxPartialLines: maxPartialLines},
		func(_ context.Context, entries []Entry) error {
			benchCRISink = entries
			flushedEntries += len(entries)
			return nil
		})
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		c.partialLines.Reset()
		copy(work, pristine)
		if err := c.process(ctx, work); err != nil {
			b.Fatal(err)
		}
	}

	b.StopTimer()
	require.Equal(b, streams*b.N, flushedEntries)
}

func BenchmarkCRIStageProcessConcurrent(b *testing.B) {
	const (
		numGoroutines     = 10
		streamsPerRun     = 5
		partialsPerStream = 3
		maxPartialLines   = 4
	)

	ts := time.Now()

	pristine := make([][]Entry, numGoroutines)
	work := make([][]Entry, numGoroutines)
	for g := range numGoroutines {
		src := generateCRIMergingWorkerBatch(g, 0, streamsPerRun, partialsPerStream)
		pristine[g] = buildCRIBenchEntries(src, ts)
		work[g] = make([]Entry, len(pristine[g]))
	}

	var forwarded atomic.Int64
	c := newCRIBenchStage(b, CRIConfig{MaxPartialLines: maxPartialLines},
		func(_ context.Context, entries []Entry) error {
			forwarded.Add(int64(len(entries)))
			return nil
		})
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	var wg sync.WaitGroup
	for g := range numGoroutines {
		wg.Add(1)
		go func(pristine, work []Entry) {
			defer wg.Done()
			for range b.N {
				copy(work, pristine)
				if err := c.process(ctx, work); err != nil {
					b.Error(err)
				}
			}
		}(pristine[g], work[g])
	}
	wg.Wait()

	b.StopTimer()
	require.Zero(b, c.partialLines.Size())
	require.Greater(b, forwarded.Load(), int64(numGoroutines*streamsPerRun*b.N))
}

func BenchmarkCRIStageProcessConcurrentNoFlush(b *testing.B) {
	const (
		numGoroutines     = 10
		streamsPerRun     = 5
		partialsPerStream = 3
		maxPartialLines   = 100
	)

	ts := time.Now()

	pristine := make([][]Entry, numGoroutines)
	work := make([][]Entry, numGoroutines)
	for g := range numGoroutines {
		src := generateCRIMergingWorkerBatch(g, 0, streamsPerRun, partialsPerStream)
		pristine[g] = buildCRIBenchEntries(src, ts)
		work[g] = make([]Entry, len(pristine[g]))
	}

	var forwarded atomic.Int64
	c := newCRIBenchStage(b, CRIConfig{MaxPartialLines: maxPartialLines},
		func(_ context.Context, entries []Entry) error {
			forwarded.Add(int64(len(entries)))
			return nil
		})
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	var wg sync.WaitGroup
	for g := range numGoroutines {
		wg.Add(1)
		go func(pristine, work []Entry) {
			defer wg.Done()
			for range b.N {
				copy(work, pristine)
				if err := c.process(ctx, work); err != nil {
					b.Error(err)
				}
			}
		}(pristine[g], work[g])
	}
	wg.Wait()

	b.StopTimer()
	require.Zero(b, c.partialLines.Size())
	require.Equal(b, int64(numGoroutines*streamsPerRun*b.N), forwarded.Load())
}
