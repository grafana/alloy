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

// Shape shared by both benchmarks: mostly plain lines that need none of the
// stateful stages, plus a minority of CRI and multiline streams -- so the
// "does a stateful stage always lock, even when nothing in the batch needs
// it" question from notes.md is actually exercised, not just the happy
// path where every entry needs special handling.
const (
	benchPlainStreams     = 80
	benchCRIStreams       = 10
	benchMultilineStreams = 10
)

// newBenchBatch builds the same input batch for both pipelines. It's only
// ever read by ProcessBatch (ConsumeStreams drains its own by-value copy
// of the Batch struct, never the caller's), so it's safe to build once and
// reuse across every iteration of both benchmarks.
func newBenchBatch() loki.Batch {
	b := loki.NewBatch()

	for i := 0; i < benchPlainStreams; i++ {
		b.Add(loki.NewStream(
			model.LabelSet{"app": "plain", "job": model.LabelValue(fmt.Sprintf("plain-%d", i))},
			push.Entry{Line: fmt.Sprintf("plain log line number %d", i)},
		))
	}

	for i := 0; i < benchCRIStreams; i++ {
		b.Add(loki.NewStream(
			model.LabelSet{"app": "cri", "job": model.LabelValue(fmt.Sprintf("cri-%d", i))},
			push.Entry{Line: "2024-01-01T00:00:00.000000000Z stdout P Hello "},
			push.Entry{Line: "2024-01-01T00:00:00.000000000Z stdout F World"},
		))
	}

	for i := 0; i < benchMultilineStreams; i++ {
		b.Add(loki.NewStream(
			model.LabelSet{"app": "multiline", "job": model.LabelValue(fmt.Sprintf("ml-%d", i))},
			push.Entry{Line: "START of the line"},
			push.Entry{Line: "continue"},
			push.Entry{Line: "START of a new line"},
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
	labels := newStaticLabelsStage2([]string{"outer", "test"})

	return NewPipeline2([]Stage2{newCRIStage2(10), match, multilineMatch, labels})
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

func BenchmarkPipeline2(b *testing.B) {
	ctx := context.Background()
	pipeline := newBenchPipeline2()
	in := newBenchBatch()

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

func BenchmarkPipeline3(b *testing.B) {
	ctx := context.Background()
	pipeline := newBenchPipeline3()
	in := newBenchBatch()

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
