package stages

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func TestPipeline3(t *testing.T) {
	ctx := context.Background()

	var (
		cri   = newCRIStage3(10)
		match = newMatchStage3(
			func(e Entry) bool { return strings.Contains(e.Line, "World") },
			MatchActionKeep,
			NewPipeline3([]Stage3{
				newStaticLabelsStage3([]string{"inner", "true"}),
			}),
		)
		multilineMatch = newMatchStage3(
			func(e Entry) bool { return e.Labels["app"] == "multiline" },
			MatchActionKeep,
			NewPipeline3([]Stage3{
				newMultilineStage3(regexp.MustCompile("^START"), 10, 50*time.Millisecond, true),
			}),
		)
		labels = newStaticLabelsStage3([]string{"outer", "test"})
	)

	pipeline := NewPipeline3([]Stage3{cri, match, multilineMatch, labels})

	criLabels := model.LabelSet{"app": "foo"}
	plainLabels := model.LabelSet{"app": "bar"}
	multilineLabels := model.LabelSet{"app": "multiline"}

	// Batch in, same as a Consumer boundary would receive: one stream
	// carrying the CRI partial/full pair, one stream with a plain line,
	// and one stream exercising multilineMatch: a start line, a
	// continuation, and a second start line that begins a new block.
	in := loki.NewBatch()
	in.Add(loki.NewStream(criLabels,
		push.Entry{Line: "2024-01-01T00:00:00.000000000Z stdout P Hello "},
		push.Entry{Line: "2024-01-01T00:00:00.000000000Z stdout F World"},
	))
	in.Add(loki.NewStream(plainLabels,
		push.Entry{Line: "plain line"},
	))
	in.Add(loki.NewStream(multilineLabels,
		push.Entry{Line: "START of the line"},
		push.Entry{Line: "continue"},
		push.Entry{Line: "START of a new line"},
	))

	out, err := pipeline.ProcessBatch(ctx, in)

	// The partial CRI line is held until its full line arrives, and the
	// second "START" line is held until it's explicitly flushed below, so
	// this batch's 6 input entries produce 3 output entries so far.
	require.Equal(t, 3, out.EntryLen())

	outStreams := collectStreams(out)
	require.Len(t, outStreams, 3)

	// Unlike Pipeline2, matchStage3 doesn't preserve relative order
	// between matched and unmatched entries (see matchStage3's doc
	// comment), so streams are looked up by label set instead of index.

	// "Hello " + "World" merged by criStage; the merged line contains
	// "World" so matchStage3 routed it through the nested pipeline
	// (picking up inner=true), and every entry picks up outer=test from
	// the outer stage.
	foo, ok := findStream(outStreams, model.LabelSet{"app": "foo", "inner": "true", "outer": "test"})
	require.True(t, ok)
	require.Len(t, foo.Entries, 1)
	require.Equal(t, "Hello World", foo.Entries[0].Line)

	// The plain line never parses as CRI, so criStage forwards it
	// untouched; it doesn't contain "World", so matchStage3 leaves it in
	// the unmatched set -- but it still picks up outer=test from the
	// outer stage.
	bar, ok := findStream(outStreams, model.LabelSet{"app": "bar", "outer": "test"})
	require.True(t, ok)
	require.Len(t, bar.Entries, 1)
	require.Equal(t, "plain line", bar.Entries[0].Line)

	// "START of the line" + "continue" were merged by multilineStage3,
	// nested inside multilineMatch, and flushed synchronously the moment
	// the second "START" line arrived and started a new block. That
	// second block sat held until the Flush call below returned it as its
	// own entry, joining the same stream.
	ml, ok := findStream(outStreams, model.LabelSet{"app": "multiline", "outer": "test"})
	require.True(t, ok)
	require.Len(t, ml.Entries, 1)
	require.Equal(t, "START of the line\ncontinue", ml.Entries[0].Line)

	// Nothing is due yet: the second "START" block was just created.
	require.True(t, pipeline.NextDeadline().After(time.Now()))

	require.Eventually(t, func() bool {
		return pipeline.NextDeadline().Before(time.Now())
	}, 1*time.Second, 100*time.Millisecond)

	flushed, err := pipeline.Flush(ctx)
	require.NoError(t, err)
	require.Len(t, flushed, 1)

	asyncBatch := loki.NewBatch()
	for _, e := range flushed {
		asyncBatch.AddEntry(e.Labels, e.Entry.Entry)
	}
	require.Equal(t, 1, asyncBatch.EntryLen())
	require.Equal(t, 1, asyncBatch.StreamLen())

	asyncStreams := collectStreams(asyncBatch)
	require.Equal(t, model.LabelSet{"app": "multiline", "outer": "test"}, asyncStreams[0].Labels)
	require.Equal(t, "START of a new line", asyncStreams[0].Entries[0].Line)
}

func findStream(streams []loki.Stream, labels model.LabelSet) (loki.Stream, bool) {
	for _, s := range streams {
		if s.Labels.Equal(labels) {
			return s, true
		}
	}
	return loki.Stream{}, false
}
