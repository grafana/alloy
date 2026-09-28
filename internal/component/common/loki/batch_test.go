package loki

import (
	"fmt"
	"testing"
	"time"

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

func TestBatch_FilterMap(t *testing.T) {
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

	b.FilterMap(func(entry *Entry) bool {
		switch entry.Line {
		case "keep":
			entry.Line = "kept"
			return true
		case "move":
			entry.Line = "moved"
			entry.Labels = bar
			return true
		case "drop":
			return false
		default:
			t.Fatalf("unexpected entry %q", entry.Line)
			return false
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

func TestBatch_FilterMapStreams(t *testing.T) {
	stream1 := model.LabelSet{"job": "move"}
	stream2 := model.LabelSet{"job": "drop"}
	stream3 := model.LabelSet{"job": "keep"}

	var b Batch
	b.Add(NewStream(stream1, push.Entry{Line: "1"}))
	b.Add(NewStream(stream2, push.Entry{Line: "2"}))
	b.Add(NewStream(stream3, push.Entry{Line: "3"}))

	require.Equal(t, 3, b.EntryLen())
	require.Equal(t, 3, b.StreamLen())

	b.FilterMapStreams(func(stream *Stream) bool {
		action := stream.Labels[model.LabelName("job")]

		switch action {
		case "keep":
			return true
		case "move":
			stream.Labels = stream3
			return true
		case "drop":
			return false
		default:
			t.Fatalf("unexpected stream labels %v", stream.Labels)
			return false
		}
	})

	require.Equal(t, 2, b.EntryLen())
	require.Equal(t, 1, b.StreamLen())

	streams := collectStreams(&b)
	require.Equal(t, stream3, streams[0].Labels)
	require.Contains(t, streams[0].Entries, push.Entry{Line: "1"})
	require.Contains(t, streams[0].Entries, push.Entry{Line: "3"})
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

	original.FilterMap(func(entry *Entry) bool {
		switch entry.Line {
		case "keep":
			entry.Line = "kept"
			return true
		case "move":
			entry.Line = "moved"
			entry.Labels = bar
			return true
		case "drop":
			return false
		default:
			t.Fatalf("unexpected entry %q", entry.Line)
			return false
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

func BenchmarkBatch_Add(b *testing.B) {
	type testCase struct {
		name       string
		numEntries int
		numStreams int
	}

	tests := []testCase{
		{
			name:       "1000 entries, single stream",
			numEntries: 1000,
			numStreams: 1,
		},
		{
			name:       "1000 entries, 10 streams",
			numEntries: 1000,
			numStreams: 10,
		},
		{
			name:       "1000 entries, 100 streams",
			numEntries: 1000,
			numStreams: 100,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			streams := make([]Stream, 0, tt.numEntries)
			for i := range tt.numEntries {
				labels := model.LabelSet{"job": model.LabelValue(fmt.Sprintf("job-%d", i%tt.numStreams))}
				streams = append(streams, NewStream(labels, push.Entry{Timestamp: time.Now(), Line: "very important log"}))
			}

			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				batch := NewBatch()
				for _, s := range streams {
					batch.Add(s)
				}
			}
		})
	}
}

func BenchmarkBatch_AddEntry(b *testing.B) {
	type testCase struct {
		name       string
		numEntries int
		numStreams int
	}

	tests := []testCase{
		{
			name:       "1000 entries, single stream",
			numEntries: 1000,
			numStreams: 1,
		},
		{
			name:       "1000 entries, 10 streams",
			numEntries: 1000,
			numStreams: 10,
		},
		{
			name:       "1000 entries, 100 streams",
			numEntries: 1000,
			numStreams: 100,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			labels := make([]model.LabelSet, 0, tt.numStreams)
			for i := range tt.numStreams {
				labels = append(labels, model.LabelSet{"job": model.LabelValue(fmt.Sprintf("job-%d", i))})
			}
			entry := push.Entry{Timestamp: time.Now(), Line: "very important log"}

			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				batch := NewBatch()
				for i := range tt.numEntries {
					batch.AddEntry(labels[i%tt.numStreams], 0, entry)
				}
			}
		})
	}
}

func BenchmarkBatch_FilterMap(b *testing.B) {
	type testCase struct {
		name       string
		numEntries int
		numStreams int
	}

	tests := []testCase{
		{
			name:       "1000 entries, single stream",
			numEntries: 1000,
			numStreams: 1,
		},
		{
			name:       "1000 entries, 10 streams",
			numEntries: 1000,
			numStreams: 10,
		},
		{
			name:       "1000 entries, 100 streams",
			numEntries: 1000,
			numStreams: 100,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			labels := make([]model.LabelSet, 0, tt.numStreams)
			for i := range tt.numStreams {
				labels = append(labels, model.LabelSet{"job": model.LabelValue(fmt.Sprintf("job-%d", i))})
			}
			entry := push.Entry{Timestamp: time.Now(), Line: "very important log"}

			batch := NewBatch()
			for i := range tt.numEntries {
				batch.AddEntry(labels[i%tt.numStreams], 0, entry)
			}

			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				batch.FilterMap(func(*Entry) bool { return true })
			}
		})
	}
}

func BenchmarkBatch_FilterMapStreams(b *testing.B) {
	type testCase struct {
		name       string
		numEntries int
		numStreams int
	}

	tests := []testCase{
		{
			name:       "1000 entries, single stream",
			numEntries: 1000,
			numStreams: 1,
		},
		{
			name:       "1000 entries, 10 streams",
			numEntries: 1000,
			numStreams: 10,
		},
		{
			name:       "1000 entries, 100 streams",
			numEntries: 1000,
			numStreams: 100,
		},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			labels := make([]model.LabelSet, 0, tt.numStreams)
			for i := range tt.numStreams {
				labels = append(labels, model.LabelSet{"job": model.LabelValue(fmt.Sprintf("job-%d", i))})
			}
			entry := push.Entry{Timestamp: time.Now(), Line: "very important log"}

			batch := NewBatch()
			for i := range tt.numEntries {
				batch.AddEntry(labels[i%tt.numStreams], 0, entry)
			}

			b.ResetTimer()
			b.ReportAllocs()
			for b.Loop() {
				batch.FilterMapStreams(func(*Stream) bool { return true })
			}
		})
	}
}

func collectStreams(b *Batch) []Stream {
	var streams []Stream
	_ = b.ConsumeStreams(func(s Stream) error {
		streams = append(streams, s)
		return nil
	})
	return streams
}
