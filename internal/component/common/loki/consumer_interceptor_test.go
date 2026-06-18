package loki

import (
	"context"
	"errors"
	"testing"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestInterceptorConsumer_ConsumeEntry(t *testing.T) {
	t.Run("forwards modified entry", func(t *testing.T) {
		next := NewCollectingConsumer()
		consumer := NewInterceptorConsumer("test", next, WithConsumeEntryHook(func(_ context.Context, entry Entry) (Entry, bool, error) {
			entry.Line = "modified"
			entry.Labels["hook"] = "true"
			return entry, true, nil
		}))

		entry := NewEntry(model.LabelSet{"job": "test"}, push.Entry{Line: "original"})
		err := consumer.ConsumeEntry(t.Context(), entry)
		require.NoError(t, err)

		entries := next.Entries()
		require.Len(t, entries, 1)
		require.Equal(t, "modified", entries[0].Line)
		require.Equal(t, model.LabelValue("true"), entries[0].Labels["hook"])
	})

	t.Run("drops entry", func(t *testing.T) {
		next := NewCollectingConsumer()
		consumer := NewInterceptorConsumer("test", next, WithConsumeEntryHook(func(_ context.Context, entry Entry) (Entry, bool, error) {
			return entry, false, nil
		}))

		err := consumer.ConsumeEntry(t.Context(), NewEntry(model.LabelSet{"job": "test"}, push.Entry{Line: "dropped"}))
		require.NoError(t, err)
		require.Empty(t, next.Entries())
	})

	t.Run("returns hook error", func(t *testing.T) {
		hookErr := errors.New("hook failed")
		next := NewCollectingConsumer()
		consumer := NewInterceptorConsumer("test", next, WithConsumeEntryHook(func(_ context.Context, entry Entry) (Entry, bool, error) {
			return entry, true, hookErr
		}))

		err := consumer.ConsumeEntry(t.Context(), NewEntry(model.LabelSet{"job": "test"}, push.Entry{Line: "failed"}))
		require.ErrorIs(t, err, hookErr)
		require.Empty(t, next.Entries())
	})

	t.Run("returns error without hook", func(t *testing.T) {
		consumer := NewInterceptorConsumer("test", NewCollectingConsumer())

		err := consumer.ConsumeEntry(t.Context(), Entry{})
		require.EqualError(t, err, "loki interceptor: unimplemented consume entry")
	})
}

func TestInterceptorConsumer_Consume(t *testing.T) {
	t.Run("forwards modified batch", func(t *testing.T) {
		next := NewCollectingConsumer()
		consumer := NewInterceptorConsumer("test", next, WithConsumeHook(func(_ context.Context, batch Batch) (Batch, error) {
			batch.FilterMap(func(entry *Entry) bool {
				entry.Line = "modified"
				entry.Labels["hook"] = "true"
				return true
			})
			return batch, nil
		}))

		batch := NewBatch()
		batch.Add(NewStream(model.LabelSet{"job": "test"}, push.Entry{Line: "original"}))

		err := consumer.Consume(t.Context(), batch)
		require.NoError(t, err)

		batches := next.Batches()
		require.Len(t, batches, 1)
		require.Equal(t, 1, batches[0].EntryLen())
		_ = batches[0].ConsumeStreams(func(stream Stream, _ int64) error {
			require.Equal(t, model.LabelValue("true"), stream.Labels["hook"])
			require.Equal(t, "modified", stream.Entries[0].Line)
			return nil
		})
	})

	t.Run("drops empty batch", func(t *testing.T) {
		next := NewCollectingConsumer()
		consumer := NewInterceptorConsumer("test", next, WithConsumeHook(func(_ context.Context, batch Batch) (Batch, error) {
			batch.FilterMap(func(_ *Entry) bool {
				return false
			})
			return batch, nil
		}))

		batch := NewBatch()
		batch.Add(NewStream(model.LabelSet{"job": "test"}, push.Entry{Line: "dropped"}))

		err := consumer.Consume(t.Context(), batch)
		require.NoError(t, err)
		require.Empty(t, next.Batches())
	})

	t.Run("returns hook error", func(t *testing.T) {
		hookErr := errors.New("hook failed")
		next := NewCollectingConsumer()
		consumer := NewInterceptorConsumer("test", next, WithConsumeHook(func(_ context.Context, batch Batch) (Batch, error) {
			return batch, hookErr
		}))

		batch := NewBatch()
		batch.Add(NewStream(model.LabelSet{"job": "test"}, push.Entry{Line: "failed"}))

		err := consumer.Consume(t.Context(), batch)
		require.ErrorIs(t, err, hookErr)
		require.Empty(t, next.Batches())
	})

	t.Run("fallback to consumeEntry", func(t *testing.T) {
		var called bool
		consumer := NewInterceptorConsumer("test", NewCollectingConsumer(), WithConsumeEntryHook(func(ctx context.Context, entry Entry) (Entry, bool, error) {
			called = true
			return entry, true, nil
		}))

		batch := NewBatch()
		batch.Add(NewStream(model.LabelSet{}, push.Entry{}))
		require.NoError(t, consumer.Consume(t.Context(), batch))
		require.True(t, called)
	})
}
