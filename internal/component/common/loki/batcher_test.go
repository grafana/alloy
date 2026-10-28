package loki

import (
	"context"
	"errors"
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
	labels := model.LabelSet{"service_name": "app"}
	ts := time.Unix(0, 0)

	t.Run("add asks for flush at flush size", func(t *testing.T) {
		b := NewStreamBatcherV2(NewCollectingConsumer(), labels, 3)

		require.False(t, b.Add(push.Entry{Timestamp: ts, Line: "line 1"}))
		require.False(t, b.Add(push.Entry{Timestamp: ts, Line: "line 2"}))
		require.True(t, b.Add(push.Entry{Timestamp: ts, Line: "line 3"}))
	})

	t.Run("flush passes added entries as one stream", func(t *testing.T) {
		consumer := NewCollectingConsumer()
		b := NewStreamBatcherV2(consumer, labels, 10)

		b.Add(push.Entry{Timestamp: ts, Line: "line 1"})
		b.Add(push.Entry{Timestamp: ts, Line: "line 2"})
		require.NoError(t, b.Flush(t.Context()))

		batches := consumer.Batches()
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
		consumer := NewCollectingConsumer()
		b := NewStreamBatcherV2(consumer, labels, 10)

		require.NoError(t, b.Flush(t.Context()))
		require.Len(t, consumer.Batches(), 0)
	})

	t.Run("flush after flush only passes new entries", func(t *testing.T) {
		consumer := NewCollectingConsumer()
		b := NewStreamBatcherV2(consumer, labels, 10)

		b.Add(push.Entry{Timestamp: ts, Line: "line 1"})
		b.Add(push.Entry{Timestamp: ts, Line: "line 2"})
		require.NoError(t, b.Flush(t.Context()))

		b.Add(push.Entry{Timestamp: ts, Line: "line 3"})
		require.NoError(t, b.Flush(t.Context()))

		batches := consumer.Batches()
		require.Len(t, batches, 2)
		require.Equal(t, []push.Entry{
			{Timestamp: ts, Line: "line 1"},
			{Timestamp: ts, Line: "line 2"},
		}, batches[0].Streams()[0].Entries)
		require.Equal(t, []push.Entry{
			{Timestamp: ts, Line: "line 3"},
		}, batches[1].Streams()[0].Entries)
	})

	t.Run("failed flush drops entries", func(t *testing.T) {
		var calls int
		consumer := consumerFunc{consume: func(_ context.Context, _ Batch) error {
			calls++
			return errors.New("consume failed")
		}}
		b := NewStreamBatcherV2(consumer, labels, 10)

		b.Add(push.Entry{Timestamp: ts, Line: "line 1"})
		require.Error(t, b.Flush(t.Context()))
		require.NoError(t, b.Flush(t.Context()))
		require.Equal(t, 1, calls)
	})
}
