package stages

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"

	"github.com/grafana/alloy/internal/component/common/loki"
)

// Shape shared by every benchmark: mostly plain streams that need none of
// the stateful stages, plus a minority of CRI and multiline streams -- so
// the "does a stateful stage always lock, even when nothing in the batch
// needs it" question from notes.md is actually exercised, not just the
// happy path where every entry needs special handling.
const (
	benchPlainStreams     = 10
	benchCRIStreams       = 10
	benchMultilineStreams = 10
)

// newBenchBatch builds the shared input batch for both pipelines.
// entriesPerStream controls how many lines go into every stream, plain,
// CRI, and multiline alike, so the same builder produces either many
// small streams (1-2 entries each, close to one line per file per poll)
// or a handful of large ones (100s of entries each, like one high-volume
// file) -- to test whether Pipeline3's per-stream slice allocation
// amortizes better once there's more to spread it across, for every
// stage's hold/merge logic, not just plain pass-through.
//
// CRI and multiline streams need at least 2 entries to still resolve one
// held block plus start a second one, so entriesPerStream is clamped to 2
// for those two regardless of what's passed in.
//
// The returned Batch is only ever read by ProcessBatch (ConsumeStreams
// drains its own by-value copy of the Batch struct, never the caller's),
// so it's safe to build once per benchmark and reuse across every
// iteration.
func newBenchBatch(entriesPerStream int) loki.Batch {
	b := loki.NewBatch()

	for i := 0; i < benchPlainStreams; i++ {
		entries := make([]push.Entry, entriesPerStream)
		for j := range entries {
			entries[j] = push.Entry{Line: fmt.Sprintf("plain log line number %d", j)}
		}
		b.Add(loki.NewStream(
			model.LabelSet{"app": "plain", "job": model.LabelValue(fmt.Sprintf("plain-%d", i))},
			entries...,
		))
	}

	// Repeated partial+full pairs -- a busy CRI source splitting many
	// separate lines over time, not one line fragmented absurdly far.
	// entriesPerStream=2 is exactly today's single pair.
	criPairs := max(entriesPerStream/2, 1)
	for i := 0; i < benchCRIStreams; i++ {
		entries := make([]push.Entry, 0, criPairs*2)
		for p := 0; p < criPairs; p++ {
			entries = append(entries,
				push.Entry{Line: fmt.Sprintf("2024-01-01T00:00:00.000000000Z stdout P line%d ", p)},
				push.Entry{Line: "2024-01-01T00:00:00.000000000Z stdout F World"},
			)
		}
		b.Add(loki.NewStream(
			model.LabelSet{"app": "cri", "job": model.LabelValue(fmt.Sprintf("cri-%d", i))},
			entries...,
		))
	}

	// Repeated 2-line blocks (start + 1 continuation) -- a source emitting
	// many separate multi-line entries over time, not one enormous merge
	// -- plus one trailing start line left held, so there's still
	// something for Flush to pick up. entriesPerStream=3 is exactly
	// today's single start/continue/new-start triple (1 block + 1
	// trailing line).
	const mlBlockSize = 2
	mlBlocks := max(entriesPerStream/mlBlockSize, 1)
	for i := 0; i < benchMultilineStreams; i++ {
		entries := make([]push.Entry, 0, mlBlocks*mlBlockSize+1)
		for blk := 0; blk < mlBlocks; blk++ {
			entries = append(entries, push.Entry{Line: "START of the line"})
			for c := 0; c < mlBlockSize-1; c++ {
				entries = append(entries, push.Entry{Line: fmt.Sprintf("continue %d", c)})
			}
		}
		entries = append(entries, push.Entry{Line: "START of a new line"})
		b.Add(loki.NewStream(
			model.LabelSet{"app": "multiline", "job": model.LabelValue(fmt.Sprintf("ml-%d", i))},
			entries...,
		))
	}

	return b
}

// Same 4-stage shape as TestPipeline2/TestPipeline3: cri -> match("World")
// with a nested static-labels pipeline -> multilineMatch with a nested
// multiline pipeline -> a final static-labels stage.

func newBenchPipeline2() *Pipeline2 {
	match := newMatchStage2(
		func(e Entry) bool { return strings.Contains(e.Line, "World") },
		MatchActionKeep,
		NewPipeline2([]Stage2{newStaticLabelsStage2([]string{"inner", "true"})}),
	)
	multilineMatch := newMatchStage2(
		func(e Entry) bool { return e.Labels["app"] == "multiline" },
		MatchActionKeep,
		NewPipeline2([]Stage2{newMultilineStage2(regexp.MustCompile("^START"), 10, 50*time.Millisecond, true)}),
	)

	return NewPipeline2([]Stage2{newCRIStage2(10), match, multilineMatch})
}

func newBenchPipeline3() *Pipeline3 {
	match := newMatchStage3(
		func(e Entry) bool { return strings.Contains(e.Line, "World") },
		MatchActionKeep,
		NewPipeline3([]Stage3{newStaticLabelsStage3([]string{"inner", "true"})}),
	)
	multilineMatch := newMatchStage3(
		func(e Entry) bool { return e.Labels["app"] == "multiline" },
		MatchActionKeep,
		NewPipeline3([]Stage3{newMultilineStage3(regexp.MustCompile("^START"), 10, 50*time.Millisecond, true)}),
	)
	labels := newStaticLabelsStage3([]string{"outer", "test"})

	return NewPipeline3([]Stage3{newCRIStage3(10), match, multilineMatch, labels})
}

// benchOut keeps the compiler from eliminating ProcessBatch's result as
// dead code.
var benchOut loki.Batch

func runPipeline2Bench(b *testing.B, entriesPerStream int) {
	ctx := context.Background()
	pipeline := newBenchPipeline2()
	in := newBenchBatch(entriesPerStream)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := pipeline.ProcessBatch(ctx, in)
		if err != nil {
			b.Fatal(err)
		}
		benchOut = out
	}
}

func runPipeline3Bench(b *testing.B, entriesPerStream int) {
	ctx := context.Background()
	pipeline := newBenchPipeline3()
	in := newBenchBatch(entriesPerStream)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := pipeline.ProcessBatch(ctx, in)
		if err != nil {
			b.Fatal(err)
		}
		benchOut = out
	}
}

// Small: 1-2 entries per stream -- the shape already benchmarked.
func BenchmarkPipeline2Small(b *testing.B) { runPipeline2Bench(b, 1) }
func BenchmarkPipeline3Small(b *testing.B) { runPipeline3Bench(b, 1) }

// Big: 200 entries per stream -- a handful of high-volume streams instead
// of many small ones, to test amortization across every stage.
func BenchmarkPipeline2Big(b *testing.B) { runPipeline2Bench(b, 200) }
func BenchmarkPipeline3Big(b *testing.B) { runPipeline3Bench(b, 200) }

// newBenchEntries returns a small, representative burst of entries meant
// to be replayed repeatedly against ProcessEntry: a few plain lines, one
// CRI partial+full pair, and the start of a multiline block. Each
// repetition of the burst naturally resolves the previous repetition's
// held multiline block -- a new "START" line always flushes whatever came
// before it -- so cycling through this forever is stable: no unbounded
// growth in criStage/multilineStage's held state.
func newBenchEntries() []loki.Entry {
	return []loki.Entry{
		loki.NewEntry(model.LabelSet{"app": "plain", "job": "p0"}, push.Entry{Line: "plain log line"}),
		loki.NewEntry(model.LabelSet{"app": "plain", "job": "p1"}, push.Entry{Line: "plain log line"}),
		loki.NewEntry(model.LabelSet{"app": "plain", "job": "p2"}, push.Entry{Line: "plain log line"}),
		loki.NewEntry(model.LabelSet{"app": "cri", "job": "c0"}, push.Entry{Line: "2024-01-01T00:00:00.000000000Z stdout P Hello "}),
		loki.NewEntry(model.LabelSet{"app": "cri", "job": "c0"}, push.Entry{Line: "2024-01-01T00:00:00.000000000Z stdout F World"}),
		loki.NewEntry(model.LabelSet{"app": "multiline", "job": "m0"}, push.Entry{Line: "START of the line"}),
		loki.NewEntry(model.LabelSet{"app": "multiline", "job": "m0"}, push.Entry{Line: "continue"}),
	}
}

// benchEntryOut keeps the compiler from eliminating ProcessEntry's result
// as dead code.
var benchEntryOut []Entry

func BenchmarkPipelineEntry2(b *testing.B) {
	ctx := context.Background()
	pipeline := newBenchPipeline2()
	entries := newBenchEntries()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := pipeline.ProcessEntry(ctx, entries[i%len(entries)])
		if err != nil {
			b.Fatal(err)
		}
		benchEntryOut = out
	}
}

func BenchmarkPipelineEntry3(b *testing.B) {
	ctx := context.Background()
	pipeline := newBenchPipeline3()
	entries := newBenchEntries()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, err := pipeline.ProcessEntry(ctx, entries[i%len(entries)])
		if err != nil {
			b.Fatal(err)
		}
		benchEntryOut = out
	}
}
