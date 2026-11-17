package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
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

func feedUntilBlocked(t *testing.T, blocked *atomic.Bool, consumer Consumer) {
	t.Helper()

	const timeout = 10 * time.Second

	e := loki.NewEntry(model.LabelSet{"A": "b"}, push.Entry{
		Line:      "test",
		Timestamp: time.Now(),
	})

	deadline := time.Now().Add(timeout)
	for !blocked.Load() {
		if time.Now().After(deadline) {
			t.Fatalf("endpoint did not block within %s", timeout)
		}

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)

		_ = consumer.ConsumeEntry(ctx, e)
		cancel()
	}
}
