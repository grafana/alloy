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
	collectBatches := func(batches *[]Batch) consumerFunc {
		return consumerFunc{consume: func(_ context.Context, batch Batch) error {
			*batches = append(*batches, batch.Clone())
			return nil
		}}
	}

	t.Run("forwards modified batch", func(t *testing.T) {
		var batches []Batch
		consumer := NewInterceptorConsumer("test", collectBatches(&batches), func(_ context.Context, batch Batch) (Batch, error) {
			batch.FilterMap(func(entry Entry) (Entry, bool) {
				entry.Line = "modified"
				entry.Labels["hook"] = "true"
				return entry, true
			})
			return batch, nil
		})

		batch := NewBatch()
		batch.Add(NewStream(model.LabelSet{"job": "test"}, push.Entry{Line: "original"}))

		err := consumer.Consume(t.Context(), batch)
		require.NoError(t, err)

		require.Len(t, batches, 1)
		require.Equal(t, 1, batches[0].EntryLen())
		stream := batches[0].Streams()[0]
		require.Equal(t, model.LabelValue("true"), stream.Labels["hook"])
		require.Equal(t, "modified", stream.Entries[0].Line)
	})

	t.Run("drops empty batch", func(t *testing.T) {
		var batches []Batch
		consumer := NewInterceptorConsumer("test", collectBatches(&batches), func(_ context.Context, batch Batch) (Batch, error) {
			batch.FilterMap(func(entry Entry) (Entry, bool) {
				return entry, false
			})
			return batch, nil
		})

		batch := NewBatch()
		batch.Add(NewStream(model.LabelSet{"job": "test"}, push.Entry{Line: "dropped"}))

		err := consumer.Consume(t.Context(), batch)
		require.NoError(t, err)
		require.Empty(t, batches)
	})

	t.Run("returns callback error", func(t *testing.T) {
		callbackErr := errors.New("callback failed")
		var batches []Batch
		consumer := NewInterceptorConsumer("test", collectBatches(&batches), func(_ context.Context, batch Batch) (Batch, error) {
			return batch, callbackErr
		})

		batch := NewBatch()
		batch.Add(NewStream(model.LabelSet{"job": "test"}, push.Entry{Line: "failed"}))

		err := consumer.Consume(t.Context(), batch)
		require.ErrorIs(t, err, callbackErr)
		require.Empty(t, batches)
	})
}
