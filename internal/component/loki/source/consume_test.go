package source

import (
	"context"
	"sync"
	"testing"

	"github.com/grafana/loki/pkg/push"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func TestConsume(t *testing.T) {
	consumer := loki.NewLogsReceiver()
	producer := loki.NewLogsReceiver()
	fanout := loki.NewFanout([]loki.LogsReceiver{consumer})

	t.Run("should fanout any consumed entries", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}
		wg.Go(func() {
			Consume(ctx, producer, fanout)
		})

		producer.Chan() <- loki.Entry{Entry: push.Entry{Line: "1"}}
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

		producer.Chan() <- loki.Entry{Entry: push.Entry{Line: "1"}}
		cancel()
		wg.Wait()
	})
}

func TestConsumeBatch(t *testing.T) {
	consumer := loki.NewLogsReceiver()
	producer := loki.NewLogsBatchReceiver()
	fanout := loki.NewFanout([]loki.LogsReceiver{consumer})

	t.Run("should fanout any consumed entries", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}
		wg.Go(func() {
			ConsumeBatch(ctx, producer, fanout)
		})

		producer.Chan() <- []loki.Entry{{Entry: push.Entry{Line: "1"}}, {Entry: push.Entry{Line: "2"}}}
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

		producer.Chan() <- []loki.Entry{{Entry: push.Entry{Line: "1"}}, {Entry: push.Entry{Line: "2"}}}
		cancel()
		wg.Wait()
	})
}
