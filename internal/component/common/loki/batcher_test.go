package loki

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestStreamBatcherV1(t *testing.T) {
	labels := model.LabelSet{"service_name": "app"}
	ts := time.Unix(0, 0)

	t.Run("flush sends added entry", func(t *testing.T) {
		recv := NewLogsReceiver(WithChannel(make(chan Entry, 1)))
		b := NewStreamBatcherV1(recv, labels)

		require.True(t, b.Add(push.Entry{Timestamp: ts, Line: "line 1"}))
		require.NoError(t, b.Flush(t.Context()))

		e := <-recv.Chan()
		require.Equal(t, labels, e.Labels)
		require.Equal(t, push.Entry{Timestamp: ts, Line: "line 1"}, e.Entry)
	})

	t.Run("flush without add sends nothing", func(t *testing.T) {
		recv := NewLogsReceiver(WithChannel(make(chan Entry, 1)))
		b := NewStreamBatcherV1(recv, labels)

		require.NoError(t, b.Flush(t.Context()))
		require.Len(t, recv.Chan(), 0)
	})

	t.Run("flush does not resend flushed entry", func(t *testing.T) {
		recv := NewLogsReceiver(WithChannel(make(chan Entry, 2)))
		b := NewStreamBatcherV1(recv, labels)

		require.True(t, b.Add(push.Entry{Timestamp: ts, Line: "line 1"}))
		require.NoError(t, b.Flush(t.Context()))
		require.NoError(t, b.Flush(t.Context()))
		require.Len(t, recv.Chan(), 1)
	})

	t.Run("failed flush drops entry", func(t *testing.T) {
		recv := NewLogsReceiver(WithChannel(make(chan Entry)))
		b := NewStreamBatcherV1(recv, labels)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.True(t, b.Add(push.Entry{Timestamp: ts, Line: "line 1"}))
		require.Error(t, b.Flush(ctx))
		require.NoError(t, b.Flush(ctx))
	})
}

func TestStreamBatcherV2(t *testing.T) {
	var (
		ts     = time.Unix(0, 0)
		labels = model.LabelSet{"service_name": "app"}
	)

	collectBatches := func(batches *[]Batch) consumerFunc {
		return consumerFunc{consume: func(_ context.Context, batch Batch) error {
			*batches = append(*batches, batch.Clone())
			return nil
		}}
	}

	t.Run("add asks for flush at flush size", func(t *testing.T) {
		b := NewStreamBatcherV2(slog.New(slog.DiscardHandler), consumerFunc{}, labels, 3)

		require.False(t, b.Add(push.Entry{Timestamp: ts, Line: "line 1"}))
		require.False(t, b.Add(push.Entry{Timestamp: ts, Line: "line 2"}))
		require.True(t, b.Add(push.Entry{Timestamp: ts, Line: "line 3"}))
	})

	t.Run("flush passes added entries as one stream", func(t *testing.T) {
		var batches []Batch
		b := NewStreamBatcherV2(slog.New(slog.DiscardHandler), collectBatches(&batches), labels, 10)

		b.Add(push.Entry{Timestamp: ts, Line: "line 1"})
		b.Add(push.Entry{Timestamp: ts, Line: "line 2"})
		require.NoError(t, b.Flush(t.Context()))

		require.Len(t, batches, 1)
		streams := batches[0].Streams()
		require.Len(t, streams, 1)
		require.Equal(t, labels, streams[0].Labels)
		require.Equal(t, []push.Entry{
			{Timestamp: ts, Line: "line 1"},
			{Timestamp: ts, Line: "line 2"},
		}, streams[0].Entries)
	})

	t.Run("flush without add does not call consumer", func(t *testing.T) {
		var batches []Batch
		b := NewStreamBatcherV2(slog.New(slog.DiscardHandler), collectBatches(&batches), labels, 10)

		require.NoError(t, b.Flush(t.Context()))
		require.Empty(t, batches)
	})

	t.Run("flush after flush only passes new entries", func(t *testing.T) {
		var batches []Batch
		b := NewStreamBatcherV2(slog.New(slog.DiscardHandler), collectBatches(&batches), labels, 10)

		b.Add(push.Entry{Timestamp: ts, Line: "line 1"})
		b.Add(push.Entry{Timestamp: ts, Line: "line 2"})
		require.NoError(t, b.Flush(t.Context()))

		b.Add(push.Entry{Timestamp: ts, Line: "line 3"})
		require.NoError(t, b.Flush(t.Context()))

		require.Len(t, batches, 2)
		require.Equal(t, []push.Entry{
			{Timestamp: ts, Line: "line 1"},
			{Timestamp: ts, Line: "line 2"},
		}, batches[0].Streams()[0].Entries)
		require.Equal(t, []push.Entry{
			{Timestamp: ts, Line: "line 3"},
		}, batches[1].Streams()[0].Entries)
	})

	t.Run("flush retries until the consumer accepts", func(t *testing.T) {
		var (
			calls   int
			batches []Batch
		)
		consumer := consumerFunc{consume: func(_ context.Context, batch Batch) error {
			calls++
			if calls <= 3 {
				return ErrConsumerStopped
			}
			batches = append(batches, batch.Clone())
			return nil
		}}

		b := NewStreamBatcherV2(slog.New(slog.DiscardHandler), consumer, labels, 10)

		b.Add(push.Entry{Timestamp: ts, Line: "line 1"})
		b.Add(push.Entry{Timestamp: ts, Line: "line 2"})
		require.NoError(t, b.Flush(t.Context()))

		require.Equal(t, 4, calls)
		require.Len(t, batches, 1)
		streams := batches[0].Streams()
		require.Len(t, streams, 1)
		require.Equal(t, labels, streams[0].Labels)
		require.Equal(t, []push.Entry{
			{Timestamp: ts, Line: "line 1"},
			{Timestamp: ts, Line: "line 2"},
		}, streams[0].Entries)
	})

	t.Run("retry sees entries as they were added", func(t *testing.T) {
		var (
			calls   int
			batches []Batch
		)
		consumer := consumerFunc{consume: func(_ context.Context, batch Batch) error {
			calls++
			if calls == 1 {
				// Change lines, drop an entry and drop structured metadata in place, like components can do.
				batch.FilterMap(func(entry Entry) (Entry, bool) {
					if entry.Line == "line 2" {
						return entry, false
					}
					entry.Line = "modified " + entry.Line
					entry.StructuredMetadata = slices.DeleteFunc(entry.StructuredMetadata, func(l push.LabelAdapter) bool {
						return l.Name == "trace_id"
					})
					return entry, true
				})
				return ErrConsumerStopped
			}
			batches = append(batches, batch.Clone())
			return nil
		}}
		b := NewStreamBatcherV2(slog.New(slog.DiscardHandler), consumer, labels, 10)

		b.Add(push.Entry{
			Timestamp: ts,
			Line:      "line 1",
			StructuredMetadata: push.LabelsAdapter{
				{Name: "trace_id", Value: "abc"},
				{Name: "span_id", Value: "def"},
			},
		})
		b.Add(push.Entry{Timestamp: ts, Line: "line 2"})
		b.Add(push.Entry{
			Timestamp: ts,
			Line:      "line 3",
			StructuredMetadata: push.LabelsAdapter{
				{Name: "trace_id", Value: "ghi"},
			},
		})
		require.NoError(t, b.Flush(t.Context()))

		require.Equal(t, 2, calls)
		require.Len(t, batches, 1)
		require.Equal(t, []push.Entry{
			{
				Timestamp: ts,
				Line:      "line 1",
				StructuredMetadata: push.LabelsAdapter{
					{Name: "trace_id", Value: "abc"},
					{Name: "span_id", Value: "def"},
				},
			},
			{Timestamp: ts, Line: "line 2"},
			{
				Timestamp: ts,
				Line:      "line 3",
				StructuredMetadata: push.LabelsAdapter{
					{Name: "trace_id", Value: "ghi"},
				},
			},
		}, batches[0].Streams()[0].Entries)
	})

	t.Run("flush stops retrying when the context is canceled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		var calls int
		consumer := consumerFunc{consume: func(_ context.Context, _ Batch) error {
			calls++
			if calls == 2 {
				cancel()
			}
			return ErrConsumerStopped
		}}
		b := NewStreamBatcherV2(slog.New(slog.DiscardHandler), consumer, labels, 10)

		b.Add(push.Entry{Timestamp: ts, Line: "line 1"})
		require.ErrorIs(t, b.Flush(ctx), context.Canceled)

		require.Equal(t, 2, calls)
	})

	t.Run("flush drops entries without retrying on an unexpected error", func(t *testing.T) {
		consumeErr := errors.New("consume failed")

		var calls int
		consumer := consumerFunc{consume: func(_ context.Context, _ Batch) error {
			calls++
			return consumeErr
		}}
		b := NewStreamBatcherV2(slog.New(slog.DiscardHandler), consumer, labels, 10)

		b.Add(push.Entry{Timestamp: ts, Line: "line 1"})
		err := b.Flush(t.Context())
		require.ErrorIs(t, err, consumeErr)
		require.NotErrorIs(t, err, ErrConsumerStopped)
		require.Equal(t, 1, calls)

		require.NoError(t, b.Flush(t.Context()))
		require.Equal(t, 1, calls)
	})
}

func BenchmarkStreamBatcherV2(b *testing.B) {
	ts := time.Unix(0, 0)

	b.Run("without structured metadata", func(b *testing.B) {
		benchmarkStreamBatcherV2(b, push.Entry{Timestamp: ts, Line: "line"})
	})

	b.Run("with structured metadata", func(b *testing.B) {
		benchmarkStreamBatcherV2(b, push.Entry{
			Timestamp: ts,
			Line:      "line",
			StructuredMetadata: push.LabelsAdapter{
				{Name: "span_id", Value: "b7ad6b7169203331"},
				{Name: "trace_id", Value: "0af7651916cd43dd8448eb211c80319c"},
			},
		})
	})
}

// benchmarkStreamBatcherV2 measures adding flushSize entries and flushing them to a consumer that does nothing.
func benchmarkStreamBatcherV2(b *testing.B, entry push.Entry) {
	const flushSize = 100

	batcher := NewStreamBatcherV2(slog.New(slog.DiscardHandler), NewNopConsumer(), model.LabelSet{"service_name": "app"}, flushSize)

	b.ReportAllocs()
	for b.Loop() {
		for range flushSize {
			batcher.Add(entry)
		}
		if err := batcher.Flush(context.Background()); err != nil {
			b.Fatal(err)
		}
	}
}
