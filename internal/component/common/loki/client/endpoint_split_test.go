package client

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/golang/snappy"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/flagext"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal/marker"
	"github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/loki/pkg/push"
)

// sizeLimitedServer is a fake Loki that rejects requests holding more than
// maxEntries entries with an HTTP 413, the way Loki rejects a push that exceeds
// its own limit on the request size. The shared util.NewRemoteWriteServer
// always responds with a fixed status and ignores the request, so it can't
// express "reject the big one, accept the halves".
//
// Limiting on entry count rather than bytes keeps the split tree predictable.
type sizeLimitedServer struct {
	*httptest.Server

	maxEntries int

	mut      sync.Mutex
	accepted []push.PushRequest
	requests int
}

func newSizeLimitedServer(t *testing.T, maxEntries int) *sizeLimitedServer {
	t.Helper()

	s := &sizeLimitedServer{maxEntries: maxEntries}
	s.Server = httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		var pushReq push.PushRequest
		if err := util.ParseProtoReader(req.Context(), req.Body, int(req.ContentLength), math.MaxInt32, &pushReq, util.RawSnappy); err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			return
		}

		var entries int
		for _, stream := range pushReq.Streams {
			entries += len(stream.Entries)
		}

		s.mut.Lock()
		defer s.mut.Unlock()

		s.requests++

		if entries > s.maxEntries {
			rw.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}

		s.accepted = append(s.accepted, pushReq)
		rw.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(s.Close)

	return s
}

// acceptedLines returns every log line the server accepted, across all
// requests and streams.
func (s *sizeLimitedServer) acceptedLines() []string {
	s.mut.Lock()
	defer s.mut.Unlock()

	var out []string
	for _, req := range s.accepted {
		for _, stream := range req.Streams {
			for _, e := range stream.Entries {
				out = append(out, e.Line)
			}
		}
	}
	return out
}

func (s *sizeLimitedServer) counts() (requests, accepted int) {
	s.mut.Lock()
	defer s.mut.Unlock()
	return s.requests, len(s.accepted)
}

// splitTestConfig points at url with a batch big enough to hold everything a
// test enqueues, so the whole input lands in one batch and the only thing that
// divides it is a 413.
func splitTestConfig(t *testing.T, url string) Config {
	t.Helper()

	serverURL := flagext.URLValue{}
	require.NoError(t, serverURL.Set(url))

	return Config{
		URL:           serverURL,
		Client:        config.DefaultHTTPClientConfig,
		BatchSize:     1024 * 1024,
		BatchWait:     10 * time.Millisecond,
		BackoffConfig: backoff.Config{MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond, MaxRetries: 3},
		Timeout:       time.Second,
		QueueConfig: QueueConfig{
			BlockOnOverflow: true,
			DrainTimeout:    30 * time.Second,
		},
	}
}

// entriesInOwnStreams returns n entries, each in its own stream so that split
// divides them by stream, with unique lines so they can be told apart.
func entriesInOwnStreams(n int) []loki.Entry {
	entries := make([]loki.Entry, 0, n)
	for i := range n {
		entries = append(entries, loki.NewEntry(
			model.LabelSet{"app": model.LabelValue(fmt.Sprintf("app-%d", i))},
			push.Entry{Timestamp: time.Unix(int64(i), 0).UTC(), Line: fmt.Sprintf("line-%d", i)},
		))
	}
	return entries
}

func expectedLines(entries []loki.Entry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Line)
	}
	return out
}

// TestEndpointSplitsBatchTooLarge checks that a batch rejected as too large is
// divided until the halves fit, and that no data is lost on the way.
func TestEndpointSplitsBatchTooLarge(t *testing.T) {
	// The server takes at most 2 entries per request, so a batch of 8 has to be
	// divided twice before it's accepted.
	server := newSizeLimitedServer(t, 2)
	entries := entriesInOwnStreams(8)

	reg := prometheus.NewRegistry()
	m := newMetrics(reg)
	cfg := splitTestConfig(t, server.URL)

	e, err := newEndpoint(m, cfg, logging.NewSlogNop(), marker.NewNopTracker())
	require.NoError(t, err)

	for _, entry := range entries {
		require.NoError(t, e.enqueue(entry, 0))
	}
	e.stop()

	// Every entry is delivered, spread across several smaller requests.
	require.ElementsMatch(t, expectedLines(entries), server.acceptedLines())

	// 1 rejected request for 8 streams, 2 rejected for 4 streams each, then 4
	// accepted for 2 streams each.
	requests, accepted := server.counts()
	require.Equal(t, 7, requests)
	require.Equal(t, 4, accepted)

	require.Equal(t, float64(len(entries)),
		testutil.ToFloat64(m.sentEntries.WithLabelValues(cfg.URL.Host, "")),
		"all entries should be counted as sent")
	require.Zero(t,
		testutil.ToFloat64(m.droppedEntries.WithLabelValues(cfg.URL.Host, "", reasonBatchTooLarge)),
		"nothing should be dropped when the halves fit")

	// One split of 8 into 4+4, then one of each 4 into 2+2.
	require.Equal(t, float64(3),
		testutil.ToFloat64(m.batchSplits.WithLabelValues(cfg.URL.Host, "")))
}

// TestEndpointDropsBatchTooLargeWhenIndivisible checks the terminal case: a
// server that refuses everything makes us divide down to single entries, and
// those are then dropped rather than retried forever.
func TestEndpointDropsBatchTooLargeWhenIndivisible(t *testing.T) {
	// maxEntries 0 rejects every request, including single-entry ones.
	server := newSizeLimitedServer(t, 0)
	entries := entriesInOwnStreams(4)

	reg := prometheus.NewRegistry()
	m := newMetrics(reg)
	cfg := splitTestConfig(t, server.URL)

	e, err := newEndpoint(m, cfg, logging.NewSlogNop(), marker.NewNopTracker())
	require.NoError(t, err)

	for _, entry := range entries {
		require.NoError(t, e.enqueue(entry, 0))
	}
	e.stop()

	require.Empty(t, server.acceptedLines())

	// The shape of the split tree is fixed even though which stream lands in
	// which half isn't: 1 request for 4 streams, 2 for 2 streams each, then 4
	// for a single stream holding a single entry, which can't be divided.
	requests, _ := server.counts()
	require.Equal(t, 7, requests)

	require.Equal(t, float64(len(entries)),
		testutil.ToFloat64(m.droppedEntries.WithLabelValues(cfg.URL.Host, "", reasonBatchTooLarge)),
		"every entry should be dropped as too large")
	require.Zero(t, testutil.ToFloat64(m.sentEntries.WithLabelValues(cfg.URL.Host, "")))

	// 1 split of 4 into 2+2, then 2 splits of 2 into 1+1.
	require.Equal(t, float64(3),
		testutil.ToFloat64(m.batchSplits.WithLabelValues(cfg.URL.Host, "")))
}

// TestShardsTrySendBatchMaxSplitDepth checks that splitting is bounded, so a
// server rejecting everything can't make one batch generate requests without
// limit. It drives trySendBatch directly at the depth boundary rather than
// enqueueing the >1024 entries it would take to reach the cap end to end.
func TestShardsTrySendBatchMaxSplitDepth(t *testing.T) {
	tests := []struct {
		name             string
		depth            int
		expectedRequests int
		expectedSplits   float64
	}{
		{
			// Already at the cap, so the batch is dropped without dividing
			// even though it holds 4 divisible entries.
			name:             "at max depth",
			depth:            maxBatchSplitDepth,
			expectedRequests: 1,
			expectedSplits:   0,
		},
		{
			// One below the cap: divides once, and both halves are then at the
			// cap and dropped.
			name:             "one below max depth",
			depth:            maxBatchSplitDepth - 1,
			expectedRequests: 3,
			expectedSplits:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := newSizeLimitedServer(t, 0)

			reg := prometheus.NewRegistry()
			m := newMetrics(reg)
			cfg := splitTestConfig(t, server.URL)

			s, err := newShards(m, logging.NewSlogNop(), marker.NewNopTracker(), cfg)
			require.NoError(t, err)

			// trySendBatch needs a context to send under. Set it directly
			// rather than calling start, which would also spawn shard workers.
			s.ctx, s.cancel = context.WithCancel(context.Background())
			defer s.cancel()

			b := newBatch(0, cfg.BatchSize)
			for _, entry := range entriesInOwnStreams(4) {
				require.NoError(t, b.add(entry, 0))
			}

			protoBuf := make([]byte, cfg.BatchSize)
			snappyBuf := make([]byte, snappy.MaxEncodedLen(cfg.BatchSize))
			s.trySendBatch("", b, &protoBuf, &snappyBuf, tt.depth)

			requests, _ := server.counts()
			require.Equal(t, tt.expectedRequests, requests)
			require.Equal(t, tt.expectedSplits,
				testutil.ToFloat64(m.batchSplits.WithLabelValues(cfg.URL.Host, "")))

			// Whatever the depth, all four entries are accounted for as
			// dropped rather than silently lost.
			require.Equal(t, float64(4),
				testutil.ToFloat64(m.droppedEntries.WithLabelValues(cfg.URL.Host, "", reasonBatchTooLarge)))
		})
	}
}
