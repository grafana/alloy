package loki

import (
	"testing"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestShardingConsumer_Consume(t *testing.T) {
	t.Run("splits batch and preserves created", func(t *testing.T) {
		const created = int64(1234)

		foo := model.LabelSet{"job": "foo"}
		bar := model.LabelSet{"job": "bar"}

		c := NewCollectingConsumer()
		sharding := NewShardingConsumer(2, c)
		defer sharding.Stop()

		batch := NewBatchWithCreatedUnixMicro(created)
		batch.Add(NewStream(foo, push.Entry{Line: "1"}))
		batch.Add(NewStream(bar, push.Entry{Line: "2"}))

		err := sharding.Consume(t.Context(), batch)
		require.NoError(t, err)

		batches := c.Batches()
		require.Len(t, batches, 2)

		got := make(map[string]Batch, len(batches))
		for _, batch := range batches {
			require.Equal(t, created, batch.created)
			require.Len(t, batch.streams, 1)
			got[batch.streams[0].Labels.String()] = batch
		}

		require.Equal(t, foo, got[foo.String()].streams[0].Labels)
		require.Equal(t, "1", got[foo.String()].streams[0].Entries[0].Line)
		require.Equal(t, bar, got[bar.String()].streams[0].Labels)
		require.Equal(t, "2", got[bar.String()].streams[0].Entries[0].Line)
	})

	t.Run("single stream fast path", func(t *testing.T) {
		const created = int64(5678)

		foo := model.LabelSet{"job": "foo"}

		c := NewCollectingConsumer()
		consumer := NewShardingConsumer(2, c)
		defer consumer.Stop()

		batch := NewBatchWithCreatedUnixMicro(created)
		batch.Add(NewStream(foo,
			push.Entry{Line: "1"},
			push.Entry{Line: "2"},
		))

		err := consumer.Consume(t.Context(), batch)
		require.NoError(t, err)

		batches := c.Batches()
		require.Len(t, batches, 1)
		require.Equal(t, created, batches[0].created)
		require.Len(t, batches[0].streams, 1)
		require.Equal(t, foo, batches[0].streams[0].Labels)
		require.Len(t, batches[0].streams[0].Entries, 2)
		require.Equal(t, "1", batches[0].streams[0].Entries[0].Line)
		require.Equal(t, "2", batches[0].streams[0].Entries[1].Line)
	})
}

func TestShardingConsumer_ConsumeEntry(t *testing.T) {
	t.Run("forwards entry to consumer", func(t *testing.T) {
		c := NewCollectingConsumer()
		consumer := NewShardingConsumer(2, c)
		defer consumer.Stop()

		entry := Entry{
			Labels:  model.LabelSet{"job": "foo"},
			created: 1234,
			Entry:   push.Entry{Line: "hello"},
		}

		err := consumer.ConsumeEntry(t.Context(), entry)
		require.NoError(t, err)

		entries := c.Entries()
		require.Len(t, entries, 1)
		require.Equal(t, entry.Labels, entries[0].Labels)
		require.Equal(t, entry.Created(), entries[0].Created())
		require.Equal(t, entry.Line, entries[0].Line)
	})
}
