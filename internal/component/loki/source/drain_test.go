package source

import (
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestDrain(t *testing.T) {
	recv := loki.NewLogsReceiver()

	var wg sync.WaitGroup
	wg.Go(func() {
		for range 10 {
			entry := loki.Entry{
				Labels: model.LabelSet{"test": "label"},
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      "test log entry",
				},
			}
			recv.Chan() <- entry
		}
	})

	completed := false
	Drain(recv, func() {
		time.Sleep(100 * time.Millisecond)
		completed = true
	})

	wg.Wait()
	require.True(t, completed, "Drain should complete without deadlock")
}
