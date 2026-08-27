package loki

import (
	"context"
	"errors"
	"testing"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestFanoutConsumer_Consume(t *testing.T) {
	firstErr := errors.New("first consumer failed")
	lastErr := errors.New("last consumer failed")

	foo := model.LabelSet{"job": "foo"}
	bar := model.LabelSet{"job": "bar"}

	var (
		firstBatch Batch
		lastBatch  Batch
	)

	fanout := NewFanoutConsumer([]Consumer{
		consumerFunc{
			consume: func(_ context.Context, batch Batch) error {
				batch.FilterMap(func(entry *Entry) bool {
					entry.Line = "mutated by first"
					entry.Labels = bar
					return true
				})
				firstBatch = batch
				return firstErr
			},
		},
		consumerFunc{
			consume: func(_ context.Context, batch Batch) error {
				lastBatch = batch
				return lastErr
			},
		},
	})

	var batch Batch
	batch.Add(NewStream(foo, push.Entry{Line: "original"}))

	err := fanout.Consume(t.Context(), batch)
	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, lastErr)

	firstStreams := collectStreams(&firstBatch)
	require.Equal(t, bar, firstStreams[0].Labels)
	require.Equal(t, "mutated by first", firstStreams[0].Entries[0].Line)

	lastStreams := collectStreams(&lastBatch)
	require.Equal(t, foo, lastStreams[0].Labels)
	require.Equal(t, "original", lastStreams[0].Entries[0].Line)
}

func TestFanoutConsumer_NilConsumer(t *testing.T) {
	fanout := NewFanoutConsumer([]Consumer{nil})
	require.NotPanics(t, func() {
		require.NoError(t, fanout.Consume(context.Background(), NewBatch()))
	})
}

func TestFanoutConsumer_ConsumerStopped(t *testing.T) {
	fanout := NewFanoutConsumer([]Consumer{
		consumerFunc{
			consume: func(_ context.Context, batch Batch) error {
				return ErrConsumerStopped
			},
		},
		consumerFunc{
			consume: func(_ context.Context, batch Batch) error {
				return ErrConsumerStopped
			},
		},
	})

	require.NoError(t, fanout.Consume(context.Background(), NewBatch()))
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
