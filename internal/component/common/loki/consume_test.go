package loki

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConsume(t *testing.T) {
	t.Run("should fanout any consumed entries", func(t *testing.T) {
		consumer := NewCollectingConsumer()
		producer := NewLogsReceiver()
		fanout := NewFanoutConsumer([]Consumer{consumer})

		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}
		wg.Go(func() {
			Consume(ctx, producer, fanout)
		})

		producer.Chan() <- Entry{Entry: push.Entry{Line: "1"}}

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			require.Len(c, consumer.Entries(), 1)
		}, 5*time.Second, 100*time.Millisecond)

		require.Equal(t, "1", consumer.Entries()[0].Entry.Line)
		cancel()
		wg.Wait()
	})

	t.Run("should stop if context is canceled during fanout", func(t *testing.T) {
		consumer := newBlockedConsumer(0)
		producer := NewLogsReceiver()
		fanout := NewFanoutConsumer([]Consumer{consumer})

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
	t.Run("should process and fanout any consumed entries", func(t *testing.T) {
		consumer := NewCollectingConsumer()
		producer := NewLogsReceiver()
		fanout := NewFanoutConsumer([]Consumer{consumer})

		ctx, cancel := context.WithCancel(context.Background())

		processFn := func(e Entry) (Entry, bool) {
			e.Entry.Line = "processed: " + e.Entry.Line
			return e, true
		}

		wg := sync.WaitGroup{}
		wg.Go(func() {
			ConsumeAndProcess(ctx, producer, fanout, processFn)
		})

		producer.Chan() <- Entry{Entry: push.Entry{Line: "1"}}

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			require.Len(c, consumer.Entries(), 1)
		}, 5*time.Second, 100*time.Millisecond)

		require.Equal(t, "processed: 1", consumer.Entries()[0].Entry.Line)
		cancel()
		wg.Wait()
	})

	t.Run("should stop if context is canceled while trying to fanout", func(t *testing.T) {
		consumer := newBlockedConsumer(0)
		producer := NewLogsReceiver()
		fanout := NewFanoutConsumer([]Consumer{consumer})

		ctx, cancel := context.WithCancel(context.Background())
		processFn := func(e Entry) (Entry, bool) {
			return e, true
		}
		wg := sync.WaitGroup{}
		wg.Go(func() {
			ConsumeAndProcess(ctx, producer, fanout, processFn)
		})

		producer.Chan() <- Entry{Entry: push.Entry{Line: "1"}}
		cancel()
		wg.Wait()
	})

	t.Run("should drop entries when process function returns false", func(t *testing.T) {
		consumer := NewCollectingConsumer()
		producer := NewLogsReceiver()
		fanout := NewFanoutConsumer([]Consumer{consumer})

		ctx, cancel := context.WithCancel(context.Background())

		processFn := func(e Entry) (Entry, bool) {
			return e, false
		}

		wg := sync.WaitGroup{}
		wg.Go(func() {
			ConsumeAndProcess(ctx, producer, fanout, processFn)
		})

		producer.Chan() <- Entry{Entry: push.Entry{Line: "1"}}

		require.Never(t, func() bool {
			return len(consumer.Entries()) > 0
		}, 1*time.Second, 100*time.Millisecond)

		cancel()
		wg.Wait()
	})
}

func TestConsumeBatch(t *testing.T) {
	t.Run("should fanout any consumed entries", func(t *testing.T) {
		consumer := NewCollectingConsumer()
		producer := NewLogsBatchReceiver()
		fanout := NewFanoutConsumer([]Consumer{consumer})

		ctx, cancel := context.WithCancel(context.Background())

		wg := sync.WaitGroup{}
		wg.Go(func() {
			ConsumeBatch(ctx, producer, fanout)
		})

		producer.Chan() <- []Entry{{Entry: push.Entry{Line: "1"}}, {Entry: push.Entry{Line: "2"}}}

		require.EventuallyWithT(t, func(c *assert.CollectT) {
			require.Len(c, consumer.Entries(), 2)
		}, 5*time.Second, 100*time.Millisecond)

		got := consumer.Entries()
		require.Equal(t, "1", got[0].Line)
		require.Equal(t, "2", got[1].Line)
		cancel()
		wg.Wait()
	})

	t.Run("should stop if context is canceled while trying to fanout", func(t *testing.T) {
		consumer := newBlockedConsumer(0)
		producer := NewLogsBatchReceiver()
		fanout := NewFanoutConsumer([]Consumer{consumer})

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

func newBlockedConsumer(after int) *blockedConsumer {
	return &blockedConsumer{
		after: after,
		ch:    make(chan Entry, max(after, 1)),
	}
}

var _ Consumer = (*blockedConsumer)(nil)

type blockedConsumer struct {
	mut   sync.Mutex
	curr  int
	after int
	ch    chan Entry
}

func (c *blockedConsumer) Consume(ctx context.Context, _ Batch) error {
	panic("unimplemented")
}

func (c *blockedConsumer) ConsumeEntry(ctx context.Context, entry Entry) error {
	c.mut.Lock()
	defer c.mut.Unlock()

	if c.curr < c.after {
		c.curr++
		c.ch <- entry
		return nil
	}

	<-ctx.Done()
	return ctx.Err()
}

func (c *blockedConsumer) Chan() <-chan Entry {
	return c.ch
}
