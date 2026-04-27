package loki

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
)

func TestShardingConsumer_Consume(t *testing.T) {
	t.Run("splits batch into streams", func(t *testing.T) {
		const created = int64(1234)

		first := model.LabelSet{"job": "fist"}
		second := model.LabelSet{"job": "second"}

		c := NewCollectingConsumer()
		sharding := NewShardingConsumer(2, c)
		defer sharding.Stop()

		original := NewBatchWithCreatedUnixMicro(created)
		original.Add(NewStream(first, push.Entry{Line: "1"}))
		original.Add(NewStream(second, push.Entry{Line: "2"}))

		err := sharding.Consume(t.Context(), original)
		require.NoError(t, err)

		batches := c.Batches()
		require.Len(t, batches, 2)

		got := make(map[string]Batch, len(batches))
		for _, batch := range batches {
			got[batch.streams[0].Labels.String()] = batch
		}

		gotFirst := got[first.String()]
		require.Equal(t, 1, gotFirst.StreamLen())
		require.Equal(t, 1, gotFirst.EntryLen())
		gotFirst.ConsumeStreams(func(stream Stream, created int64) {
			require.Equal(t, original.Created(), created)
			require.Equal(t, first, stream.Labels)
			require.Equal(t, "1", stream.Entries[0].Line)
		})

		gotSecond := got[second.String()]
		require.Equal(t, 1, gotSecond.StreamLen())
		require.Equal(t, 1, gotSecond.EntryLen())
		gotSecond.ConsumeStreams(func(stream Stream, created int64) {
			require.Equal(t, original.Created(), created)
			require.Equal(t, second, stream.Labels)
			require.Equal(t, "2", stream.Entries[0].Line)
		})
	})

	t.Run("single stream fast path", func(t *testing.T) {
		labels := model.LabelSet{"job": "first"}

		c := NewCollectingConsumer()
		consumer := NewShardingConsumer(2, c)
		defer consumer.Stop()

		batch := NewBatch()
		batch.Add(NewStream(labels,
			push.Entry{Line: "1"},
			push.Entry{Line: "2"},
		))

		err := consumer.Consume(t.Context(), batch)
		require.NoError(t, err)

		batches := c.Batches()
		require.Len(t, batches, 1)
		require.Equal(t, 1, batch.StreamLen())
		require.Equal(t, 2, batch.EntryLen())
		batch.ConsumeStreams(func(stream Stream, _ int64) {
			require.Equal(t, labels, stream.Labels)
			require.Equal(t, "1", stream.Entries[0].Line)
			require.Equal(t, "2", stream.Entries[1].Line)
		})
	})

	t.Run("preserves backpressure per shard", func(t *testing.T) {
		var (
			wg             sync.WaitGroup
			callCount      = atomic.NewInt64(0)
			linesProcessed = make(chan string)
			release        = make(chan struct{})
		)

		consumer := NewShardingConsumer(2, consumerFunc{
			consume: func(_ context.Context, batch Batch) error {
				callCount.Inc()
				linesProcessed <- batch.streams[0].Entries[0].Line

				<-release
				return nil
			},
		})
		defer consumer.Stop()

		first := NewBatch()
		first.Add(NewStream(labelsForShard(consumer, 0), push.Entry{Line: "first"}))
		wg.Go(func() {
			_ = consumer.Consume(t.Context(), first)
		})

		// Make sure first is being processed and thus taking up shard 0.
		requireReceive(t, linesProcessed, "first", 1*time.Second)
		require.Equal(t, int64(1), callCount.Load())

		// Create a second batch with labels that will use shard 0.
		second := NewBatch()
		second.Add(NewStream(labelsForShard(consumer, 0), push.Entry{Line: "second"}))
		wg.Go(func() {
			_ = consumer.Consume(t.Context(), second)
		})

		// Create a third batch that will use shard 1 so it should
		// be able to progress.
		third := NewBatch()
		third.Add(NewStream(labelsForShard(consumer, 1), push.Entry{Line: "third"}))
		wg.Go(func() {
			_ = consumer.Consume(t.Context(), third)
		})

		requireReceive(t, linesProcessed, "third", 1*time.Second)
		require.Equal(t, int64(2), callCount.Load())

		// Finish both in-flight calls so that the second batch can progress.
		close(release)

		requireReceive(t, linesProcessed, "second", 1*time.Second)
		require.Equal(t, int64(3), callCount.Load())

		wg.Wait()
	})

	t.Run("returns context error while waiting for shard", func(t *testing.T) {
		var (
			wg        sync.WaitGroup
			errs      = make(chan error, 2)
			callCount = atomic.NewInt64(0)
		)

		consumer := NewShardingConsumer(1, consumerFunc{
			consume: func(ctx context.Context, batch Batch) error {
				callCount.Inc()
				<-ctx.Done()
				return ctx.Err()
			},
		})
		defer consumer.Stop()

		ctx1, cancel1 := context.WithCancel(context.Background())
		first := NewBatch()
		first.Add(NewStream(labelsForShard(consumer, 0), push.Entry{Line: "first"}))
		wg.Go(func() {
			errs <- consumer.Consume(ctx1, first)
		})

		require.Eventually(
			t,
			func() bool { return callCount.Load() == 1 },
			1*time.Second,
			50*time.Microsecond,
		)

		ctx2, cancel2 := context.WithCancel(context.Background())
		second := NewBatch()
		second.Add(NewStream(labelsForShard(consumer, 0), push.Entry{Line: "second"}))
		wg.Go(func() {
			errs <- consumer.Consume(ctx2, second)
		})

		// Cancel queued batch.
		cancel2()
		require.ErrorIs(t, <-errs, context.Canceled)

		// Cancel in-flight batch.
		cancel1()
		require.ErrorIs(t, <-errs, context.Canceled)
		require.Equal(t, int64(1), callCount.Load())

		wg.Wait()
	})

	t.Run("returns error after stop", func(t *testing.T) {
		consumer := NewShardingConsumer(1, consumerFunc{})
		consumer.Stop()

		batch := NewBatch()
		batch.Add(NewStream(labelsForShard(consumer, 0), push.Entry{Line: "first"}))

		err := consumer.Consume(t.Context(), batch)
		require.ErrorIs(t, err, ErrConsumerStopped)
	})

}

func TestShardingConsumer_ConsumeEntry(t *testing.T) {
	t.Run("forwards entry to consumer", func(t *testing.T) {
		c := NewCollectingConsumer()
		consumer := NewShardingConsumer(2, c)
		defer consumer.Stop()

		entry := NewEntry(model.LabelSet{"job": "foo"}, push.Entry{
			Line: "hello",
		})

		err := consumer.ConsumeEntry(t.Context(), entry)
		require.NoError(t, err)

		entries := c.Entries()
		require.Len(t, entries, 1)
		got := entries[0]
		require.Equal(t, entry.Line, got.Line)
		require.Equal(t, entry.Labels, got.Labels)
		require.Equal(t, entry.Created(), got.Created())
	})

	t.Run("preserves backpressure per shard", func(t *testing.T) {
		var (
			wg             sync.WaitGroup
			callCount      = atomic.NewInt64(0)
			linesProcessed = make(chan string)
			release        = make(chan struct{})
		)

		consumer := NewShardingConsumer(2, consumerFunc{
			consumeEntry: func(_ context.Context, entry Entry) error {
				callCount.Inc()
				linesProcessed <- entry.Line

				<-release
				return nil
			},
		})
		defer consumer.Stop()

		first := NewEntry(labelsForShard(consumer, 0), push.Entry{Line: "first"})
		wg.Go(func() {
			_ = consumer.ConsumeEntry(t.Context(), first)
		})

		requireReceive(t, linesProcessed, "first", 1*time.Second)
		require.Equal(t, int64(1), callCount.Load())

		second := NewEntry(labelsForShard(consumer, 0), push.Entry{Line: "second"})
		wg.Go(func() {
			_ = consumer.ConsumeEntry(t.Context(), second)
		})

		third := NewEntry(labelsForShard(consumer, 1), push.Entry{Line: "third"})
		wg.Go(func() {
			_ = consumer.ConsumeEntry(t.Context(), third)
		})

		requireReceive(t, linesProcessed, "third", 1*time.Second)
		require.Equal(t, int64(2), callCount.Load())

		close(release)

		requireReceive(t, linesProcessed, "second", 1*time.Second)
		require.Equal(t, int64(3), callCount.Load())

		wg.Wait()
	})

	t.Run("returns context error while waiting for shard", func(t *testing.T) {
		var (
			wg        sync.WaitGroup
			errs      = make(chan error, 2)
			callCount = atomic.NewInt64(0)
		)

		consumer := NewShardingConsumer(1, consumerFunc{
			consumeEntry: func(ctx context.Context, entry Entry) error {
				callCount.Inc()
				<-ctx.Done()
				return ctx.Err()
			},
		})
		defer consumer.Stop()

		ctx1, cancel1 := context.WithCancel(context.Background())
		first := NewEntry(labelsForShard(consumer, 0), push.Entry{Line: "first"})
		wg.Go(func() {
			errs <- consumer.ConsumeEntry(ctx1, first)
		})

		require.Eventually(
			t,
			func() bool { return callCount.Load() == 1 },
			1*time.Second,
			50*time.Microsecond,
		)

		ctx2, cancel2 := context.WithCancel(context.Background())
		second := NewEntry(labelsForShard(consumer, 0), push.Entry{Line: "second"})
		wg.Go(func() {
			errs <- consumer.ConsumeEntry(ctx2, second)
		})

		cancel2()
		require.ErrorIs(t, <-errs, context.Canceled)

		cancel1()
		require.ErrorIs(t, <-errs, context.Canceled)
		require.Equal(t, int64(1), callCount.Load())

		wg.Wait()
	})

	t.Run("returns error after stop", func(t *testing.T) {
		consumer := NewShardingConsumer(1, consumerFunc{})
		consumer.Stop()

		entry := NewEntry(labelsForShard(consumer, 0), push.Entry{Line: "first"})

		err := consumer.ConsumeEntry(t.Context(), entry)
		require.ErrorIs(t, err, ErrConsumerStopped)
	})
}

func labelsForShard(consumer *ShardingConsumer, shard int) model.LabelSet {
	for i := 0; ; i++ {
		labels := model.LabelSet{"job": model.LabelValue(string(strconv.Itoa(i)))}
		if consumer.shardFor(labels) == shard {
			return labels
		}
	}
}

func requireReceive[T any](t *testing.T, ch <-chan T, expected T, timeout time.Duration) {
	t.Helper()

	select {
	case v := <-ch:
		require.Equal(t, expected, v)
	case <-time.After(timeout):
		t.Fatal("timed out waiting for receive")
	}
}
