package client

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
)

func newBlockedServer() (*httptest.Server, *atomic.Bool, func()) {
	var (
		done     = make(chan struct{})
		doneOnce sync.Once
		blocked  = atomic.NewBool(false)
	)

	release := func() { doneOnce.Do(func() { close(done) }) }

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		blocked.Store(true)
		select {
		case <-done:
		case <-r.Context().Done():
		}
	}))

	return server, blocked, release
}

func feedUntilBlocked(t *testing.T, blocked *atomic.Bool, c chan<- loki.Entry) {
	e := loki.NewEntry(model.LabelSet{"A": "b"}, push.Entry{
		Line:      "test",
		Timestamp: time.Now(),
	})

	for !blocked.Load() {
		select {
		case c <- e:
		case <-time.After(50 * time.Millisecond):
		}
	}
	require.True(t, blocked.Load())

}
