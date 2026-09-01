package stages

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

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

func generateCRIMixedWorkerBatch(worker, run, fullLines, partialLines int) []criBenchLine {
	out := make([]criBenchLine, 0, fullLines+partialLines)
	for i := range fullLines {
		out = append(out, criBenchLine{
			labels: model.LabelSet{
				"namespace": "default",
				"pod":       model.LabelValue(fmt.Sprintf("worker-%d-full-%d", worker, i)),
				"container": "main",
			},
			line: fmt.Sprintf("%s stdout F full line %d of worker %d", criTestTS, i, worker),
		})
	}
	for i := range partialLines {
		out = append(out, criBenchLine{
			labels: model.LabelSet{
				"namespace": "default",
				"pod":       model.LabelValue(fmt.Sprintf("worker-%d-run-%d-partial-%d", worker, run, i)),
				"container": "main",
			},
			line: fmt.Sprintf("%s stdout P partial line %d of worker %d run %d ", criTestTS, i, worker, run),
		})
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

func resetCRIBenchEntries(entries []Entry, src []criBenchLine, ts time.Time) {
	for i, s := range src {
		clear(entries[i].Extracted)
		clear(entries[i].Labels)
		for k, v := range s.labels {
			entries[i].Labels[k] = v
		}
		entries[i].Line = s.line
		entries[i].Timestamp = ts
	}
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
		streams           = 50
		partialsPerStream = 9
	)

	src := generateCRIPartialThenFull(streams, partialsPerStream)
	ts := time.Now()
	entries := buildCRIBenchEntries(src, ts)

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
		b.StopTimer()
		resetCRIBenchEntries(entries, src, ts)
		b.StartTimer()

		if err := c.process(ctx, entries); err != nil {
			b.Fatal(err)
		}
	}

	require.Empty(b, c.partialLines)
	require.Equal(b, streams*b.N, forwarded)
}

func BenchmarkCRIStageProcessMaxPartialLinesFlush(b *testing.B) {
	const (
		streams         = 500
		maxPartialLines = 25
	)

	src := generateCRIPartialsOnly(streams)
	ts := time.Now()
	entries := buildCRIBenchEntries(src, ts)

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
		b.StopTimer()
		resetCRIBenchEntries(entries, src, ts)
		clear(c.partialLines)
		b.StartTimer()

		if err := c.process(ctx, entries); err != nil {
			b.Fatal(err)
		}
	}

	flushesPerIteration := (streams - 1) / maxPartialLines
	wantEntries := flushesPerIteration * maxPartialLines * b.N
	require.Equal(b, wantEntries, flushedEntries)
	require.Len(b, c.partialLines, streams-flushesPerIteration*maxPartialLines)
}

func BenchmarkCRIStageProcessConcurrent(b *testing.B) {
	const (
		numGoroutines   = 10
		fullLinesPerRun = 20
		partialsPerRun  = 20
		maxPartialLines = 4
	)

	ts := time.Now()

	batches := make([][][]Entry, numGoroutines)
	for g := range numGoroutines {
		batches[g] = make([][]Entry, b.N)
		for i := range b.N {
			src := generateCRIMixedWorkerBatch(g, i, fullLinesPerRun, partialsPerRun)
			batches[g][i] = buildCRIBenchEntries(src, ts)
		}
	}

	var flushedEntries atomic.Int64
	c := newCRIBenchStage(b, CRIConfig{MaxPartialLines: maxPartialLines},
		func(_ context.Context, entries []Entry) error {
			flushedEntries.Add(int64(len(entries)))
			return nil
		})
	ctx := context.Background()

	b.ReportAllocs()
	b.ResetTimer()

	var wg sync.WaitGroup
	for g := range numGoroutines {
		wg.Add(1)
		go func(batches [][]Entry) {
			defer wg.Done()
			for _, entries := range batches {
				if err := c.process(ctx, entries); err != nil {
					b.Error(err)
				}
			}
		}(batches[g])
	}
	wg.Wait()

	require.Positive(b, flushedEntries.Load())
}
