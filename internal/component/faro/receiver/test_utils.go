package receiver

import (
	"sync"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/loki/v3/pkg/logproto"
)

type fakeLogsReceiver struct {
	ch chan loki.Entry

	entriesMut sync.RWMutex
	entries    []loki.Entry
}

var _ loki.LogsReceiver = (*fakeLogsReceiver)(nil)

func newFakeLogsReceiver(t *testing.T) *fakeLogsReceiver {
	ctx := componenttest.TestContext(t)

	lr := &fakeLogsReceiver{
		ch: make(chan loki.Entry),
	}

	go func() {
		defer close(lr.ch)

		select {
		case <-ctx.Done():
			return
		case ent := <-lr.Chan():
			lr.entriesMut.Lock()
			lr.entries = append(lr.entries, loki.Entry{
				Labels: ent.Labels,
				Entry: logproto.Entry{
					Timestamp:          time.Time{}, // Use consistent time for testing.
					Line:               ent.Line,
					StructuredMetadata: ent.StructuredMetadata,
				},
			})
			lr.entriesMut.Unlock()
		}
	}()

	return lr
}

func (lr *fakeLogsReceiver) Chan() chan loki.Entry {
	return lr.ch
}

func (lr *fakeLogsReceiver) GetEntries() []loki.Entry {
	lr.entriesMut.RLock()
	defer lr.entriesMut.RUnlock()
	return lr.entries
}
