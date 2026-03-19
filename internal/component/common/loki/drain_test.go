package loki

import (
	"sync"
	"testing"
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
		collector := NewCollectingHandler()
		defer collector.Stop()

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

		completed := false
		Drain(recv, NewFanout([]LogsReceiver{collector.Receiver()}), time.Second, func() {
			require.Eventually(t, func() bool {
				return len(collector.Received()) == 1
			}, time.Second, 10*time.Millisecond)
			completed = true
		})

		producer.Wait()
		require.True(t, completed)
		require.Len(t, collector.Received(), 1)
		require.Equal(t, "forwarded", collector.Received()[0].Line)
	})

	t.Run("falls back to discard when forwarding blocks", func(t *testing.T) {
		recv := NewLogsReceiver()
		blockedRecv := NewLogsReceiver()

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

		completed := false
		Drain(recv, NewFanout([]LogsReceiver{blockedRecv}), 20*time.Millisecond, func() {
			time.Sleep(100 * time.Millisecond)
			completed = true
		})

		producer.Wait()
		require.True(t, completed)
	})
}
