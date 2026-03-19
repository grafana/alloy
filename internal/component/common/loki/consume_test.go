package loki

import (
	"context"
	"sync"
	"testing"

	"github.com/grafana/loki/pkg/push"
	"github.com/stretchr/testify/require"
)

func TestConsume(t *testing.T) {
	consumer := NewLogsReceiver()
	producer := NewLogsReceiver()
	fanout := NewFanout([]LogsReceiver{consumer})

	t.Run("should fanout any consumed entries", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}
		wg.Go(func() {
			Consume(ctx, producer, fanout)
		})

		producer.Chan() <- Entry{Entry: push.Entry{Line: "1"}}
		e := <-consumer.Chan()
		require.Equal(t, "1", e.Entry.Line)
		cancel()
		wg.Wait()
	})

	t.Run("should stop if context is canceled while trying to fanout", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		wg := sync.WaitGroup{}
		wg.Go(func() {
			Consume(ctx, producer, fanout)
		})

		producer.Chan() <- Entry{Entry: push.Entry{Line: "1"}}
		cancel()
		wg.Wait()
	})
}

func TestConsumeAndProcess(t *testing.T) {
	consumer := NewLogsReceiver()
	producer := NewLogsReceiver()
	fanout := NewFanout([]LogsReceiver{consumer})

	t.Run("should process and fanout any consumed entries", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		processFn := func(e Entry) Entry {
			e.Entry.Line = "processed: " + e.Entry.Line
			return e
		}

		wg := sync.WaitGroup{}
		wg.Go(func() {
			ConsumeAndProcess(ctx, producer, fanout, processFn)
		})

		producer.Chan() <- Entry{Entry: push.Entry{Line: "1"}}
		e := <-consumer.Chan()
		require.Equal(t, "processed: 1", e.Entry.Line)
		cancel()
		wg.Wait()
	})

	t.Run("should stop if context is canceled while trying to fanout", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		processFn := func(e Entry) Entry {
			return e
		}
		wg := sync.WaitGroup{}
		wg.Go(func() {
			ConsumeAndProcess(ctx, producer, fanout, processFn)
		})

		producer.Chan() <- Entry{Entry: push.Entry{Line: "1"}}
		cancel()
		wg.Wait()
	})
}

func TestConsumeBatch(t *testing.T) {
	consumer := NewLogsReceiver()
	producer := NewLogsBatchReceiver()
	fanout := NewFanout([]LogsReceiver{consumer})

	t.Run("should fanout any consumed entries", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}
		wg.Go(func() {
			ConsumeBatch(ctx, producer, fanout)
		})

		producer.Chan() <- []Entry{{Entry: push.Entry{Line: "1"}}, {Entry: push.Entry{Line: "2"}}}
		e := <-consumer.Chan()
		require.Equal(t, "1", e.Entry.Line)
		e = <-consumer.Chan()
		require.Equal(t, "2", e.Entry.Line)
		cancel()
		wg.Wait()
	})

	t.Run("should stop if context is canceled while trying to fanout", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		wg := sync.WaitGroup{}
		wg.Go(func() {
			ConsumeBatch(ctx, producer, fanout)
		})

		producer.Chan() <- []Entry{{Entry: push.Entry{Line: "1"}}, {Entry: push.Entry{Line: "2"}}}
		cancel()
		wg.Wait()
	})
}
