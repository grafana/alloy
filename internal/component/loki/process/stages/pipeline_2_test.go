package stages

import (
	"context"
	"strings"
	"testing"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func TestPipeline2(t *testing.T) {
	ctx := context.Background()

	var (
		cri   = newCRIStage2(10)
		match = newMatchStage2(
			func(e Entry) bool { return strings.Contains(e.Line, "World") },
			MatchActionKeep,
			NewPipeline2([]Stage2{
				newStaticLabelsStage2([]string{"inner", "true"}),
			}),
		)
		labels = newStaticLabelsStage2([]string{"outer", "test"})
	)

	pipeline := NewPipeline2([]Stage2{cri, match, labels})

	criLabels := model.LabelSet{"app": "foo"}
	plainLabels := model.LabelSet{"app": "bar"}

	// Batch in, same as a Consumer boundary would receive: one stream
	// carrying the CRI partial/full pair, one stream with a plain line.
	in := loki.NewBatch()
	in.Add(loki.NewStream(criLabels,
		push.Entry{Line: "2024-01-01T00:00:00.000000000Z stdout P Hello "},
		push.Entry{Line: "2024-01-01T00:00:00.000000000Z stdout F World"},
	))
	in.Add(loki.NewStream(plainLabels,
		push.Entry{Line: "plain line"},
	))

	out := loki.NewBatch()
	err := in.ConsumeStreams(func(stream loki.Stream, created int64) error {
		collect := EmitterFunc(func(_ context.Context, e Entry) error {
			out.Add(loki.NewStream(e.Labels, e.Entry.Entry))
			return nil
		})

		for _, pe := range stream.Entries {
			entry := Entry{
				Extracted: map[string]any{},
				Entry:     loki.NewEntryWithCreatedUnixMicro(stream.Labels.Clone(), created, pe),
			}
			if err := pipeline.Process(ctx, entry, collect); err != nil {
				return err
			}
		}
		return nil
	})
	require.NoError(t, err)

	// The partial line is held by criStage until the full line arrives,
	// so three input entries produce two output entries.
	require.Equal(t, 2, out.EntryLen())

	var outStreams []loki.Stream
	require.NoError(t, out.ConsumeStreams(func(s loki.Stream, _ int64) error {
		outStreams = append(outStreams, s)
		return nil
	}))

	// "Hello " + "World" merged by criStage; the merged line contains
	// "World" so matchStage2 routed it through the nested pipeline
	// (picking up inner=true), and every entry picks up outer=test from
	// the outer stage.
	require.Equal(t, model.LabelSet{"app": "foo", "inner": "true", "outer": "test"}, outStreams[0].Labels)
	require.Len(t, outStreams[0].Entries, 1)
	require.Equal(t, "Hello World", outStreams[0].Entries[0].Line)

	// The plain line never parses as CRI, so criStage forwards it
	// untouched; it doesn't contain "World", so matchStage2 sends it
	// straight to next without the nested pipeline -- but it still picks
	// up outer=test from the outer stage.
	require.Equal(t, model.LabelSet{"app": "bar", "outer": "test"}, outStreams[1].Labels)
	require.Len(t, outStreams[1].Entries, 1)
	require.Equal(t, "plain line", outStreams[1].Entries[0].Line)
}
