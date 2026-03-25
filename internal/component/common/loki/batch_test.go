package loki

import (
	"testing"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestBatch_Add(t *testing.T) {
	foo := model.LabelSet{"job": "foo"}
	bar := model.LabelSet{"job": "bar"}

	var b Batch
	b.Add(NewStream(foo, push.Entry{Line: "1"}))
	b.Add(NewStream(foo, push.Entry{Line: "2"}))
	b.Add(NewStream(bar, push.Entry{Line: "3"}))

	require.Equal(t, 3, b.EntryLen())
	require.Equal(t, 2, b.StreamLen())

	streams := collectStreams(&b)
	require.Equal(t, foo, streams[0].Labels)
	require.Equal(t, []push.Entry{{Line: "1"}, {Line: "2"}}, streams[0].Entries)
	require.Equal(t, bar, streams[1].Labels)
	require.Equal(t, []push.Entry{{Line: "3"}}, streams[1].Entries)
}

func TestBatch_IterMut(t *testing.T) {
	foo := model.LabelSet{"job": "foo"}
	bar := model.LabelSet{"job": "bar"}

	var b Batch
	b.Add(NewStream(foo,
		push.Entry{Line: "keep"},
		push.Entry{Line: "move"},
		push.Entry{Line: "drop"},
	))

	require.Equal(t, 3, b.EntryLen())
	require.Equal(t, 1, b.StreamLen())

	b.IterMut(func(entry *Entry) EntryAction {
		switch entry.Line {
		case "keep":
			entry.Line = "kept"
			return ActionKeep
		case "move":
			entry.Line = "moved"
			entry.Labels = bar
			return ActionKeep
		case "drop":
			return ActionDrop
		default:
			t.Fatalf("unexpected entry %q", entry.Line)
			return ActionDrop
		}
	})

	require.Equal(t, 2, b.EntryLen())
	require.Equal(t, 2, b.StreamLen())

	streams := collectStreams(&b)
	require.Equal(t, foo, streams[0].Labels)
	require.Equal(t, []push.Entry{{Line: "kept"}}, streams[0].Entries)
	require.Equal(t, bar, streams[1].Labels)
	require.Equal(t, []push.Entry{{Line: "moved"}}, streams[1].Entries)
}

func TestBatch_ConsumeStreams(t *testing.T) {
	foo := model.LabelSet{"job": "foo"}
	bar := model.LabelSet{"job": "bar"}

	var b Batch
	b.Add(NewStream(foo, push.Entry{Line: "1"}))

	first := collectStreams(&b)
	require.Equal(t, 0, b.EntryLen())
	require.Equal(t, 0, b.StreamLen())
	require.Equal(t, foo, first[0].Labels)
	require.Equal(t, []push.Entry{{Line: "1"}}, first[0].Entries)

	b.Add(NewStream(bar, push.Entry{Line: "2"}))

	second := collectStreams(&b)
	require.Equal(t, 0, b.EntryLen())
	require.Equal(t, 0, b.StreamLen())

	require.Equal(t, bar, second[0].Labels)
	require.Equal(t, []push.Entry{{Line: "2"}}, second[0].Entries)
}

func TestBatch_Clone(t *testing.T) {
	foo := model.LabelSet{"job": "foo"}
	bar := model.LabelSet{"job": "bar"}

	var original Batch
	original.Add(NewStream(foo,
		push.Entry{Line: "keep"},
		push.Entry{Line: "move"},
		push.Entry{Line: "drop"},
	))

	cloned := original.Clone()

	original.IterMut(func(entry *Entry) EntryAction {
		switch entry.Line {
		case "keep":
			entry.Line = "kept"
			return ActionKeep
		case "move":
			entry.Line = "moved"
			entry.Labels = bar
			return ActionKeep
		case "drop":
			return ActionDrop
		default:
			t.Fatalf("unexpected entry %q", entry.Line)
			return ActionDrop
		}
	})

	require.Equal(t, 2, original.EntryLen())
	require.Equal(t, 2, original.StreamLen())

	originalStreams := collectStreams(&original)
	require.Equal(t, foo, originalStreams[0].Labels)
	require.Equal(t, []push.Entry{{Line: "kept"}}, originalStreams[0].Entries)
	require.Equal(t, bar, originalStreams[1].Labels)
	require.Equal(t, []push.Entry{{Line: "moved"}}, originalStreams[1].Entries)

	require.Equal(t, 3, cloned.EntryLen())
	require.Equal(t, 1, cloned.StreamLen())

	clonedStreams := collectStreams(&cloned)
	require.Equal(t, foo, clonedStreams[0].Labels)
	require.Equal(t, "keep", clonedStreams[0].Entries[0].Line)
	require.Equal(t, "move", clonedStreams[0].Entries[1].Line)
	require.Equal(t, "drop", clonedStreams[0].Entries[2].Line)
}

func collectStreams(b *Batch) []Stream {
	var streams []Stream
	b.ConsumeStreams(func(s Stream) {
		streams = append(streams, s)
	})
	return streams
}
