package loki

import (
	"context"
	"errors"
	"testing"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestFanoutConsumer_ConsumeEntry(t *testing.T) {
	firstErr := errors.New("first consumer failed")
	lastErr := errors.New("last consumer failed")

	var (
		firstEntry Entry
		lastEntry  Entry
	)

	fanout := NewFanoutConsumer([]Consumer{
		consumerFunc{
			consumeEntry: func(_ context.Context, entry Entry) error {
				entry.Line = "mutated by first"
				firstEntry = entry
				return firstErr
			},
		},
		consumerFunc{
			consumeEntry: func(_ context.Context, entry Entry) error {
				lastEntry = entry
				return lastErr
			},
		},
	})

	entry := Entry{
		Labels: model.LabelSet{"job": "test"},
		Entry:  push.Entry{Line: "original"},
	}

	err := fanout.ConsumeEntry(t.Context(), entry)
	require.ErrorIs(t, err, firstErr)
	require.ErrorIs(t, err, lastErr)

	require.Equal(t, "mutated by first", firstEntry.Line)
	require.Equal(t, "original", lastEntry.Line)
}

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
				batch.IterMut(func(entry *Entry) EntryAction {
					entry.Line = "mutated by first"
					entry.Labels = bar
					return ActionKeep
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

type consumerFunc struct {
	consume      func(context.Context, Batch) error
	consumeEntry func(context.Context, Entry) error
}

func (c consumerFunc) Consume(ctx context.Context, batch Batch) error {
	if c.consume == nil {
		return nil
	}
	return c.consume(ctx, batch)
}

func (c consumerFunc) ConsumeEntry(ctx context.Context, entry Entry) error {
	if c.consumeEntry == nil {
		return nil
	}
	return c.consumeEntry(ctx, entry)
}
