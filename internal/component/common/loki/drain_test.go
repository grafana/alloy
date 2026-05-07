package loki

import (
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestDrain(t *testing.T) {
	defer goleak.VerifyNone(t)

	t.Run("forwards while fn runs", func(t *testing.T) {
		recv := NewLogsReceiver()
		consumer := NewCollectingConsumer()

		var producer sync.WaitGroup
		producer.Go(func() {
			recv.Chan() <- Entry{
				Labels: model.LabelSet{"test": "label"},
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      "forwarded",
				},
			}
		})

		Drain(recv, NewFanoutConsumer([]Consumer{consumer}), time.Second, func() {
			require.Eventually(t, func() bool {
				return len(consumer.Entries()) == 1
			}, time.Second, 10*time.Millisecond)
		})

		producer.Wait()
		require.Equal(t, "forwarded", consumer.Entries()[0].Line)
	})

	t.Run("falls back to discard when forwarding blocks", func(t *testing.T) {
		recv := NewLogsReceiver()
		blockedConsumer := newBlockedConsumer(0)

		var producer sync.WaitGroup
		producer.Go(func() {
			for range 2 {
				recv.Chan() <- Entry{
					Labels: model.LabelSet{"test": "label"},
					Entry: push.Entry{
						Timestamp: time.Now(),
						Line:      "blocked",
					},
				}
			}
		})

		Drain(recv, NewFanoutConsumer([]Consumer{blockedConsumer}), 20*time.Millisecond, func() {
			time.Sleep(100 * time.Millisecond)
		})

		producer.Wait()
	})

	t.Run("forwards one entry and discard rest", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			recv := NewLogsReceiver()
			consumer := newBlockedConsumer(1)

			var producerWG sync.WaitGroup
			producerWG.Go(func() {
				for i := range 3 {
					recv.Chan() <- Entry{
						Entry: push.Entry{
							Timestamp: time.Now(),
							Line:      strconv.Itoa(i),
						},
					}
				}
			})

			var wg sync.WaitGroup
			wg.Go(func() {
				Drain(recv, NewFanoutConsumer([]Consumer{consumer}), 100*time.Millisecond, func() {
					// Wait until the producer has finished sending all entries.
					producerWG.Wait()
				})
			})

			// Wait until all go routines are blocked and advance time.
			synctest.Wait()
			time.Sleep(101 * time.Millisecond)
			wg.Wait()

			// Make sure we only get the first entry.
			entry := <-consumer.Chan()
			require.Equal(t, "0", entry.Line)
			synctest.Wait()

			select {
			case extra := <-consumer.Chan():
				t.Fatalf("unexpected extra forwarded entry: %q", extra.Line)
			default:
			}
		})
	})
}
