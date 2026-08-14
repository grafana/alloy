package stages

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

type entrySink4 struct {
	mu      sync.Mutex
	calls   int
	entries []Entry
}

func (s *entrySink4) emit(_ context.Context, entries []Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.entries = append(s.entries, entries...)
	return nil
}

func (s *entrySink4) snapshot() (int, []Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Entry, len(s.entries))
	copy(out, s.entries)
	return s.calls, out
}

func TestPipeline4(t *testing.T) {
	ctx := context.Background()

	ml, err := newMultilineStage4(MultilineConfig{
		Expression:   "^START",
		MaxLines:     10,
		MaxWaitTime:  50 * time.Millisecond,
		TrimNewlines: true,
	})
	require.NoError(t, err)
	t.Cleanup(ml.Cleanup)

	var (
		cri   = newCRIStage4(10)
		match = newMatchStage4(
			func(e Entry) bool { return strings.Contains(e.Line, "World") },
			MatchActionKeep,
			[]Stage4{newStaticLabelsStage4([]string{"inner", "true"})},
		)
		multilineMatch = newMatchStage4(
			func(e Entry) bool { return e.Labels["app"] == "multiline" },
			MatchActionKeep,
			[]Stage4{ml},
		)
		labels = newStaticLabelsStage4([]string{"outer", "test"})
	)

	pipeline := NewPipeline4([]Stage4{cri, match, multilineMatch, labels})

	sink := &entrySink4{}
	pipeline.SetNext(sink.emit)

	criLabels := model.LabelSet{"app": "foo"}
	plainLabels := model.LabelSet{"app": "bar"}
	multilineLabels := model.LabelSet{"app": "multiline"}

	// Same input as TestPipeline2/TestPipeline3.
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

	err = pipeline.ProcessBatch(ctx, in)
	require.NoError(t, err)

	calls, entries := sink.snapshot()
	require.Equal(t, 3, calls)

	// The partial CRI line is held until its full line arrives, and the
	// second "START" line is held until the flush loop picks it up
	// asynchronously below, so this batch's 6 input entries produce 3
	// entries so far.
	require.Len(t, entries, 3)

	// "Hello " + "World" merged by criStage4; the merged line contains
	// "World" so match routed it through its nested pipeline (picking up
	// inner=true), and it picks up outer=test from the outer stage.
	foo, ok := findEntry(entries, model.LabelSet{"app": "foo", "inner": "true", "outer": "test"})
	require.True(t, ok)
	require.Equal(t, "Hello World", foo.Line)

	// The plain line never parses as CRI, so criStage4 forwards it
	// untouched; it doesn't contain "World", so match leaves it unmatched
	// -- but it still picks up outer=test from the outer stage.
	bar, ok := findEntry(entries, model.LabelSet{"app": "bar", "outer": "test"})
	require.True(t, ok)
	require.Equal(t, "plain line", bar.Line)

	// "START of the line" + "continue" were merged by the multilineStage4
	// nested inside multilineMatch, flushed synchronously the moment the
	// second "START" line arrived and started a new block.
	block, ok := findEntry(entries, model.LabelSet{"app": "multiline", "outer": "test"})
	require.True(t, ok)
	require.Equal(t, "START of the line\ncontinue", block.Line)

	// The second "START" block is still held. Unlike Pipeline2/Pipeline3,
	// there's no NextDeadline/Flush to poll or call explicitly: the flush
	// loop ticks on its own every MaxWaitTime and calls next directly once
	// the block goes idle.
	require.Eventually(t, func() bool {
		_, entries := sink.snapshot()
		return len(entries) == 4
	}, time.Second, 10*time.Millisecond)

	_, entries = sink.snapshot()
	last := entries[3]
	require.Equal(t, model.LabelSet{"app": "multiline", "outer": "test"}, last.Labels)
	require.Equal(t, "START of a new line", last.Line)
}

func findEntry(entries []Entry, labels model.LabelSet) (Entry, bool) {
	for _, e := range entries {
		if e.Labels.Equal(labels) {
			return e, true
		}
	}
	return Entry{}, false
}
