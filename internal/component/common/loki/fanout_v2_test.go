package loki

import (
	"context"
	"errors"
	"testing"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestFanoutV2_Consume(t *testing.T) {
	firstErr := errors.New("first consumer failed")
	lastErr := errors.New("last consumer failed")

	foo := model.LabelSet{"job": "foo"}
	bar := model.LabelSet{"job": "bar"}

	var (
		firstBatch Batch
		lastBatch  Batch
	)

	fanout := NewFanoutV2([]LogsReceiver{
		NewLogsReceiverV2(consumerFunc{
			consume: func(_ context.Context, batch Batch) error {
				batch.FilterMap(func(entry Entry) (Entry, bool) {
					entry.Line = "mutated by first"
					entry.Labels = bar
					return entry, true
				})
				firstBatch = batch
				return firstErr
			},
		}, "first"),
		NewLogsReceiverV2(consumerFunc{
			consume: func(_ context.Context, batch Batch) error {
				lastBatch = batch
				return lastErr
			},
		}, "last"),
	})

	var batch Batch
	batch.Add(NewStream(foo, push.Entry{Line: "original"}))

	err := fanout.Consume(t.Context(), batch)
	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, lastErr)

	firstStreams := firstBatch.Streams()
	require.Equal(t, bar, firstStreams[0].Labels)
	require.Equal(t, "mutated by first", firstStreams[0].Entries[0].Line)

	lastStreams := lastBatch.Streams()
	require.Equal(t, foo, lastStreams[0].Labels)
	require.Equal(t, "original", lastStreams[0].Entries[0].Line)
}

func TestFanoutV2_NilConsumer(t *testing.T) {
	fanout := NewFanoutV2([]LogsReceiver{nil})
	require.NotPanics(t, func() {
		require.NoError(t, fanout.Consume(context.Background(), NewBatch()))
	})
}

func TestFanoutV2_ConsumerStopped(t *testing.T) {
	var firstCalled, secondCalled bool

	fanout := NewFanoutV2([]LogsReceiver{
		NewLogsReceiverV2(consumerFunc{
			consume: func(_ context.Context, batch Batch) error {
				firstCalled = true
				return ErrConsumerStopped
			},
		}, "first"),
		NewLogsReceiverV2(consumerFunc{
			consume: func(_ context.Context, batch Batch) error {
				secondCalled = true
				return nil
			},
		}, "second"),
	})

	err := fanout.Consume(context.Background(), NewBatch())
	require.ErrorIs(t, err, ErrConsumerStopped)
	require.True(t, firstCalled)
	require.True(t, secondCalled)
}

func TestFanoutV2_ReceiverWithoutConsumer(t *testing.T) {
	recv := NewLogsReceiver()

	require.Panics(t, func() {
		NewFanoutV2([]LogsReceiver{recv})
	})

	fanout := NewFanoutV2(nil)
	require.Panics(t, func() {
		fanout.UpdateChildren([]LogsReceiver{recv})
	})
}

type consumerFunc struct {
	consume func(context.Context, Batch) error
}

func (c consumerFunc) Consume(ctx context.Context, batch Batch) error {
	if c.consume == nil {
		return nil
	}
	return c.consume(ctx, batch)
}
