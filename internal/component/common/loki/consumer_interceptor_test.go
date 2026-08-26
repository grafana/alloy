package loki

import (
	"context"
	"errors"
	"testing"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestInterceptorConsumer_Consume(t *testing.T) {
	t.Run("forwards modified batch", func(t *testing.T) {
		next := NewCollectingConsumer()
		consumer := NewInterceptorConsumer("test", next, func(_ context.Context, batch Batch) (Batch, error) {
			batch.FilterMap(func(entry *Entry) bool {
				entry.Line = "modified"
				entry.Labels["hook"] = "true"
				return true
			})
			return batch, nil
		})

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
		consumer := NewInterceptorConsumer("test", next, func(_ context.Context, batch Batch) (Batch, error) {
			batch.FilterMap(func(_ *Entry) bool {
				return false
			})
			return batch, nil
		})

		batch := NewBatch()
		batch.Add(NewStream(model.LabelSet{"job": "test"}, push.Entry{Line: "dropped"}))

		err := consumer.Consume(t.Context(), batch)
		require.NoError(t, err)
		require.Empty(t, next.Batches())
	})

	t.Run("returns callback error", func(t *testing.T) {
		callbackErr := errors.New("callback failed")
		next := NewCollectingConsumer()
		consumer := NewInterceptorConsumer("test", next, func(_ context.Context, batch Batch) (Batch, error) {
			return batch, callbackErr
		})

		batch := NewBatch()
		batch.Add(NewStream(model.LabelSet{"job": "test"}, push.Entry{Line: "failed"}))

		err := consumer.Consume(t.Context(), batch)
		require.ErrorIs(t, err, callbackErr)
		require.Empty(t, next.Batches())
	})
}
