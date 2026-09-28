package client

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/loki/util"
)

type consumeFunc func(ctx context.Context, c Consumer, entries []loki.Entry) error

// runConsumerTest runs fn as a subtest once for Consume and once for ConsumeEntry.
func runConsumerTest(t *testing.T, fn func(t *testing.T, consume consumeFunc)) {
	t.Run("Consume", func(t *testing.T) {
		fn(t, func(ctx context.Context, c Consumer, entries []loki.Entry) error {
			batch := loki.NewBatch()
			for _, e := range entries {
				batch.AddEntry(e.Labels, 0, e.Entry)
			}
			return c.Consume(ctx, batch)
		})
	})

	t.Run("ConsumeEntry", func(t *testing.T) {
		fn(t, func(ctx context.Context, c Consumer, entries []loki.Entry) error {
			for _, e := range entries {
				if err := c.ConsumeEntry(ctx, e); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

// newTestEntries returns n entries with labels lset and lines "line0" to "line<n-1>".
func newTestEntries(lset model.LabelSet, n int) []loki.Entry {
	entries := make([]loki.Entry, 0, n)
	for i := range n {
		entries = append(entries, loki.NewEntry(lset, push.Entry{
			Timestamp: time.Now(),
			Line:      fmt.Sprintf("line%d", i),
		}))
	}
	return entries
}

// requireReceivedEntries waits for numEntries requests and asserts that each one
// holds a single entry with the expected labels and that no line is duplicated.
func requireReceivedEntries(t *testing.T, received *util.SyncSlice[util.RemoteWriteRequest], labels string, numEntries int) {
	t.Helper()

	require.Eventually(t, func() bool {
		return received.Length() == numEntries
	}, 5*time.Second, 100*time.Millisecond, "timed out waiting for requests to be received")

	var seenEntries = map[string]struct{}{}
	defer received.DoneIterate()
	for _, req := range received.StartIterate() {
		require.Len(t, req.Request.Streams, 1, "expected 1 stream requests to be received")
		require.Len(t, req.Request.Streams[0].Entries, 1, "expected 1 entry in the only stream received per request")
		require.Equal(t, labels, req.Request.Streams[0].Labels)
		seenEntries[req.Request.Streams[0].Entries[0].Line] = struct{}{}
	}
	require.Len(t, seenEntries, numEntries)
}

func newServerAndEndpointConfig(t *testing.T) (Config, *util.SyncSlice[util.RemoteWriteRequest], func()) {
	receivedReqsChan := make(chan util.RemoteWriteRequest, 10)
	receivedRequests := util.NewSyncSlice[util.RemoteWriteRequest]()
	go func() {
		for req := range receivedReqsChan {
			receivedRequests.Append(req)
		}
	}()

	// Start a local HTTP server
	server := util.NewRemoteWriteServer(receivedReqsChan, http.StatusOK)
	require.NotNil(t, server)

	url, _ := url.Parse(server.URL)
	endpointConfig := Config{
		Name:      "test-client",
		URL:       flagext.URLValue{URL: url},
		Timeout:   time.Second * 2,
		BatchSize: 1,
		BackoffConfig: backoff.Config{
			MaxRetries: 0,
		},
		QueueConfig: QueueConfig{
			Capacity:        10,
			DrainTimeout:    time.Second * 10,
			BlockOnOverflow: true,
		},
	}
	return endpointConfig, receivedRequests, func() {
		server.Close()
		close(receivedReqsChan)
	}
}

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

func feedUntilBlocked(t *testing.T, blocked *atomic.Bool, consumer Consumer, consume consumeFunc) {
	t.Helper()

	const timeout = 10 * time.Second

	entries := []loki.Entry{loki.NewEntry(model.LabelSet{"A": "b"}, push.Entry{
		Line:      "test",
		Timestamp: time.Now(),
	})}

	deadline := time.Now().Add(timeout)
	for !blocked.Load() {
		if time.Now().After(deadline) {
			t.Fatalf("endpoint did not block within %s", timeout)
		}

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)

		_ = consume(ctx, consumer, entries)
		cancel()
	}
}
