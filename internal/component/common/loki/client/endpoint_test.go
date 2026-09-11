package client

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal/marker"
	"github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestEndpoint(t *testing.T) {
	type testCase struct {
		name                 string
		endpointConfig       Config
		serverResponseStatus int
		inputEntries         []loki.Entry
		inputDelay           time.Duration
		expectedReqs         []util.RemoteWriteRequest
		expectedMetrics      string
	}

	tests := []testCase{
		{
			name: "batch log entries together until the batch size is reached",
			endpointConfig: Config{
				BatchSize: logEntries[0].Size() + logEntries[1].Size(),
				BatchWait: 100 * time.Millisecond,
			},
			serverResponseStatus: 200,
			inputEntries:         []loki.Entry{logEntries[0], logEntries[1], logEntries[2]},
			expectedReqs: []util.RemoteWriteRequest{
				{
					TenantID: "",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[0].Entry, logEntries[1].Entry}}}},
				},
				{
					TenantID: "",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[2].Entry}}}},
				},
			},
			expectedMetrics: `
                               # HELP loki_write_sent_entries_total Number of log entries sent to the ingester.
                               # TYPE loki_write_sent_entries_total counter
                               loki_write_sent_entries_total{host="__HOST__",tenant=""} 3.0
                               # HELP loki_write_dropped_entries_total Number of log entries dropped because failed to be sent to the ingester after all retries.
                               # TYPE loki_write_dropped_entries_total counter
                               loki_write_dropped_entries_total{host="__HOST__",reason="batch_too_large",tenant=""} 0
                               loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
							   loki_write_dropped_entries_total{host="__HOST__",reason="queue_is_full",tenant=""} 0
                               loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                               loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                       `,
		},
		{
			name: "batch log entries separately when the batch wait time is reached",
			endpointConfig: Config{
				BatchSize: int(1 * units.GB),
				BatchWait: 50 * time.Millisecond,
			},
			serverResponseStatus: 200,
			inputEntries:         []loki.Entry{logEntries[0], logEntries[1]},
			inputDelay: func() time.Duration {
				// On windows this test is really flaky, a lot of times
				// shards are not started before we queue all items so they
				// end up in the same batch so we need to wait longer to make
				// sure everything is running.
				if runtime.GOOS == "windows" {
					return 2 * time.Second
				}
				return 700 * time.Millisecond
			}(),
			expectedReqs: []util.RemoteWriteRequest{
				{
					TenantID: "",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[0].Entry}}}},
				},
				{
					TenantID: "",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[1].Entry}}}},
				},
			},
			expectedMetrics: `
                              # HELP loki_write_sent_entries_total Number of log entries sent to the ingester.
                              # TYPE loki_write_sent_entries_total counter
                              loki_write_sent_entries_total{host="__HOST__",tenant=""} 2.0
                              # HELP loki_write_dropped_entries_total Number of log entries dropped because failed to be sent to the ingester after all retries.
                              # TYPE loki_write_dropped_entries_total counter
                              loki_write_dropped_entries_total{host="__HOST__",reason="batch_too_large",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
							  loki_write_dropped_entries_total{host="__HOST__",reason="queue_is_full",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                       `,
		},
		{
			name: "retry send a batch up to backoff's max retries in case the server responds with a 5xx",
			endpointConfig: Config{
				BatchSize: 10,
				BatchWait: 10 * time.Millisecond,
			},
			serverResponseStatus: 500,
			inputEntries:         []loki.Entry{logEntries[0]},
			expectedReqs: []util.RemoteWriteRequest{
				{
					TenantID: "",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[0].Entry}}}},
				},
				{
					TenantID: "",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[0].Entry}}}},
				},
				{
					TenantID: "",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[0].Entry}}}},
				},
			},
			expectedMetrics: `
                              # HELP loki_write_dropped_entries_total Number of log entries dropped because failed to be sent to the ingester after all retries.
                              # TYPE loki_write_dropped_entries_total counter
                              loki_write_dropped_entries_total{host="__HOST__",reason="batch_too_large",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 1
							  loki_write_dropped_entries_total{host="__HOST__",reason="queue_is_full",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_sent_entries_total Number of log entries sent to the ingester.
                              # TYPE loki_write_sent_entries_total counter
                              loki_write_sent_entries_total{host="__HOST__",tenant=""} 0
                       `,
		},
		{
			name: "do not retry send a batch in case the server responds with a 4xx",
			endpointConfig: Config{
				BatchSize: 10,
				BatchWait: 10 * time.Millisecond,
			},
			serverResponseStatus: 400,
			inputEntries:         []loki.Entry{logEntries[0]},
			expectedReqs: []util.RemoteWriteRequest{
				{
					TenantID: "",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[0].Entry}}}},
				},
			},
			expectedMetrics: `
                              # HELP loki_write_dropped_entries_total Number of log entries dropped because failed to be sent to the ingester after all retries.
                              # TYPE loki_write_dropped_entries_total counter
                              loki_write_dropped_entries_total{host="__HOST__",reason="batch_too_large",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 1
							  loki_write_dropped_entries_total{host="__HOST__",reason="queue_is_full",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_sent_entries_total Number of log entries sent to the ingester.
                              # TYPE loki_write_sent_entries_total counter
                              loki_write_sent_entries_total{host="__HOST__",tenant=""} 0
                       `,
		},
		{
			name: "do retry sending a batch in case the server responds with a 429",
			endpointConfig: Config{
				BatchSize: 10,
				BatchWait: 10 * time.Millisecond,
			},
			serverResponseStatus: 429,
			inputEntries:         []loki.Entry{logEntries[0]},
			expectedReqs: []util.RemoteWriteRequest{
				{
					TenantID: "",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[0].Entry}}}},
				},
				{
					TenantID: "",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[0].Entry}}}},
				},
				{
					TenantID: "",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[0].Entry}}}},
				},
			},
			expectedMetrics: `
                              # HELP loki_write_dropped_entries_total Number of log entries dropped because failed to be sent to the ingester after all retries.
                              # TYPE loki_write_dropped_entries_total counter
                              loki_write_dropped_entries_total{host="__HOST__",reason="batch_too_large",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
							  loki_write_dropped_entries_total{host="__HOST__",reason="queue_is_full",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 1
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_sent_entries_total Number of log entries sent to the ingester.
                              # TYPE loki_write_sent_entries_total counter
                              loki_write_sent_entries_total{host="__HOST__",tenant=""} 0
                       `,
		},
		{
			name: "do not retry in case of 429 when endpoint is configured to drop rate limited batches",
			endpointConfig: Config{
				BatchSize:              10,
				BatchWait:              10 * time.Millisecond,
				DropRateLimitedBatches: true,
			},
			serverResponseStatus: 429,
			inputEntries:         []loki.Entry{logEntries[0]},
			expectedReqs: []util.RemoteWriteRequest{
				{
					TenantID: "",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[0].Entry}}}},
				},
			},
			expectedMetrics: `
                              # HELP loki_write_dropped_entries_total Number of log entries dropped because failed to be sent to the ingester after all retries.
                              # TYPE loki_write_dropped_entries_total counter
                              loki_write_dropped_entries_total{host="__HOST__",reason="batch_too_large",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
							  loki_write_dropped_entries_total{host="__HOST__",reason="queue_is_full",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 1
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_sent_entries_total Number of log entries sent to the ingester.
                              # TYPE loki_write_sent_entries_total counter
                              loki_write_sent_entries_total{host="__HOST__",tenant=""} 0
                       `,
		},
		{
			name: "batch log entries together honoring the endpoint tenant ID",
			endpointConfig: Config{
				BatchSize: 100,
				BatchWait: 100 * time.Millisecond,
				TenantID:  "tenant-default",
			},
			serverResponseStatus: 200,
			inputEntries:         []loki.Entry{logEntries[0], logEntries[1]},
			expectedReqs: []util.RemoteWriteRequest{
				{
					TenantID: "tenant-default",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[0].Entry, logEntries[1].Entry}}}},
				},
			},
			expectedMetrics: `
                              # HELP loki_write_sent_entries_total Number of log entries sent to the ingester.
                              # TYPE loki_write_sent_entries_total counter
                              loki_write_sent_entries_total{host="__HOST__",tenant="tenant-default"} 2.0
                              # HELP loki_write_dropped_entries_total Number of log entries dropped because failed to be sent to the ingester after all retries.
                              # TYPE loki_write_dropped_entries_total counter
                              loki_write_dropped_entries_total{host="__HOST__", reason="batch_too_large", tenant="tenant-default"} 0
                              loki_write_dropped_entries_total{host="__HOST__", reason="ingester_error", tenant="tenant-default"} 0
							  loki_write_dropped_entries_total{host="__HOST__",reason="queue_is_full",tenant="tenant-default"} 0
                              loki_write_dropped_entries_total{host="__HOST__", reason="rate_limited", tenant="tenant-default"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant="tenant-default"} 0
                       `,
		},
		{
			name: "batch log entries together honoring the tenant ID overridden while processing the pipeline stages",
			endpointConfig: Config{
				BatchSize: 100,
				BatchWait: 100 * time.Millisecond,
				TenantID:  "tenant-default",
			},
			serverResponseStatus: 200,
			inputEntries:         []loki.Entry{logEntries[0], logEntries[3], logEntries[4], logEntries[5]},
			expectedReqs: []util.RemoteWriteRequest{
				{
					TenantID: "tenant-default",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[0].Entry}}}},
				},
				{
					TenantID: "tenant-1",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[3].Entry, logEntries[4].Entry}}}},
				},
				{
					TenantID: "tenant-2",
					Request:  push.PushRequest{Streams: []push.Stream{{Labels: "{}", Entries: []push.Entry{logEntries[5].Entry}}}},
				},
			},
			expectedMetrics: `
                              # HELP loki_write_sent_entries_total Number of log entries sent to the ingester.
                              # TYPE loki_write_sent_entries_total counter
                              loki_write_sent_entries_total{host="__HOST__",tenant="tenant-1"} 2.0
                              loki_write_sent_entries_total{host="__HOST__",tenant="tenant-2"} 1.0
                              loki_write_sent_entries_total{host="__HOST__",tenant="tenant-default"} 1.0
                              # HELP loki_write_dropped_entries_total Number of log entries dropped because failed to be sent to the ingester after all retries.
                              # TYPE loki_write_dropped_entries_total counter
                              loki_write_dropped_entries_total{host="__HOST__",reason="batch_too_large",tenant="tenant-1"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="batch_too_large",tenant="tenant-2"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="batch_too_large",tenant="tenant-default"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant="tenant-1"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant="tenant-2"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant="tenant-default"} 0
							  loki_write_dropped_entries_total{host="__HOST__",reason="queue_is_full",tenant="tenant-1"} 0
							  loki_write_dropped_entries_total{host="__HOST__",reason="queue_is_full",tenant="tenant-2"} 0
							  loki_write_dropped_entries_total{host="__HOST__",reason="queue_is_full",tenant="tenant-default"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant="tenant-1"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant="tenant-2"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant="tenant-default"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant="tenant-1"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant="tenant-2"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant="tenant-default"} 0
                       `,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := prometheus.NewRegistry()

			// Create a buffer channel where we do enqueue received requests
			receivedReqsChan := make(chan util.RemoteWriteRequest, 10)

			// Start a local HTTP server
			server := util.NewRemoteWriteServer(receivedReqsChan, tt.serverResponseStatus)
			require.NotNil(t, server)
			defer server.Close()

			// Get the URL at which the local test server is listening to
			serverURL := flagext.URLValue{}
			err := serverURL.Set(server.URL)
			require.NoError(t, err)

			tt.endpointConfig.URL = serverURL
			tt.endpointConfig.Client = config.DefaultHTTPClientConfig
			tt.endpointConfig.BackoffConfig = backoff.Config{MinBackoff: 1 * time.Millisecond, MaxBackoff: 2 * time.Millisecond, MaxRetries: 3}
			tt.endpointConfig.Timeout = 1 * time.Second
			tt.endpointConfig.QueueConfig.BlockOnOverflow = true
			tt.endpointConfig.QueueConfig.DrainTimeout = 30 * time.Second

			m := newMetrics(reg)
			c, err := newEndpoint(m, tt.endpointConfig, logging.NewSlogNop(), marker.NewNopTracker())
			require.NoError(t, err)

			// Send all the input log entries
			for i, logEntry := range tt.inputEntries {
				c.enqueue(t.Context(), logEntry, 0)

				if tt.inputDelay > 0 && i < len(tt.inputEntries)-1 {
					time.Sleep(tt.inputDelay)
				}
			}

			// Wait until the expected push requests are received (with a timeout)
			deadline := time.Now().Add(1 * time.Second)
			for len(receivedReqsChan) < len(tt.expectedReqs) && time.Now().Before(deadline) {
				time.Sleep(5 * time.Millisecond)
			}

			// Stop the endpoint: it waits until the current batch is sent
			c.stop()
			close(receivedReqsChan)

			// Get all push requests received on the server side
			receivedReqs := make([]util.RemoteWriteRequest, 0)
			for req := range receivedReqsChan {
				receivedReqs = append(receivedReqs, req)
			}

			assert.ElementsMatch(t, tt.expectedReqs, receivedReqs)

			expectedMetrics := strings.ReplaceAll(tt.expectedMetrics, "__HOST__", serverURL.Host)
			err = testutil.GatherAndCompare(reg, strings.NewReader(expectedMetrics), "loki_write_sent_entries_total", "loki_write_dropped_entries_total")
			assert.NoError(t, err)
		})
	}
}

func TestEndpointBlockOnOverflow(t *testing.T) {
	t.Run("should drop entry when queue is full and BlockOnOverflow is false", func(t *testing.T) {
		receivedReqsChan := make(chan util.RemoteWriteRequest)
		server := util.NewRemoteWriteServer(receivedReqsChan, http.StatusOK)

		var url flagext.URLValue
		require.NoError(t, url.Set(server.URL))

		m := newMetrics(prometheus.NewRegistry())
		e, err := newEndpoint(m, Config{
			URL:       url,
			BatchWait: 1 * time.Hour,
			BatchSize: 1,
			Timeout:   20 * time.Second,
			Client:    config.DefaultHTTPClientConfig,
			QueueConfig: QueueConfig{
				Capacity:        1,
				MinShards:       1,
				BlockOnOverflow: false,
			},
		}, logging.NewSlogNop(), marker.NewNopTracker())
		require.NoError(t, err)
		defer e.stop()

		entry := loki.Entry{Entry: push.Entry{Line: "my entry"}}

		// NOTE: We have configured batch size to 1 so only one entry will fit in each batch.
		// To exceed the queue's capacity we need to pass 4 entries. We have one batch that we are actively trying
		// to send, one batch that is queued and one batch that we are currently working with filling up.
		require.NoError(t, e.enqueue(t.Context(), entry, 0))
		require.NoError(t, e.enqueue(t.Context(), entry, 0))

		// Which enqueue fails depends on whether the shard worker has already
		// consumed the queued batch after the third call. If the third call loses that race,
		// it returns errQueueIsFull, otherwise the fourth call does.
		err3 := e.enqueue(t.Context(), entry, 0)
		err4 := e.enqueue(t.Context(), entry, 0)
		queueIsFull := errors.Is(err3, errQueueIsFull) || errors.Is(err4, errQueueIsFull)
		require.True(t, queueIsFull, "expected either the third or fourth enqueue to fail with queue full")
	})

	t.Run("should block until queue has space when BlockOnOverflow is true", func(t *testing.T) {
		receivedReqsChan := make(chan util.RemoteWriteRequest)
		server := util.NewRemoteWriteServer(receivedReqsChan, http.StatusOK)

		var url flagext.URLValue
		require.NoError(t, url.Set(server.URL))

		m := newMetrics(prometheus.NewRegistry())
		e, err := newEndpoint(m, Config{
			URL:       url,
			BatchWait: 1 * time.Hour,
			BatchSize: 1,
			Timeout:   20 * time.Second,
			Client:    config.DefaultHTTPClientConfig,
			QueueConfig: QueueConfig{
				Capacity:        1,
				MinShards:       1,
				BlockOnOverflow: true,
			},
		}, logging.NewSlogNop(), marker.NewNopTracker())
		require.NoError(t, err)
		defer e.stop()

		entry1 := loki.Entry{Entry: push.Entry{Line: "1"}}
		entry2 := loki.Entry{Entry: push.Entry{Line: "2"}}
		entry3 := loki.Entry{Entry: push.Entry{Line: "3"}}
		entry4 := loki.Entry{Entry: push.Entry{Line: "4"}}

		go func() {
			// After 200 milliseconds we read one request to unblock the queue.
			time.Sleep(200 * time.Millisecond)
			// We just need to finish one request in order for all entries to be successfully enqueued.
			<-receivedReqsChan
		}()
		require.NoError(t, e.enqueue(t.Context(), entry1, 0))
		require.NoError(t, e.enqueue(t.Context(), entry2, 0))
		require.NoError(t, e.enqueue(t.Context(), entry3, 0))
		require.NoError(t, e.enqueue(t.Context(), entry4, 0))
	})
}

func TestEndpointBatchSizeMetric(t *testing.T) {
	receivedReqsChan := make(chan util.RemoteWriteRequest, 10)
	server := util.NewRemoteWriteServer(receivedReqsChan, http.StatusOK)
	defer server.Close()

	var url flagext.URLValue
	require.NoError(t, url.Set(server.URL))

	entries := []loki.Entry{
		{Entry: push.Entry{Timestamp: time.Unix(1, 0), Line: "line 1"}},
		{Entry: push.Entry{Timestamp: time.Unix(2, 0), Line: "line 2"}},
	}

	reg := prometheus.NewRegistry()
	e, err := newEndpoint(newMetrics(reg), Config{
		URL: url,
		// Both entries fit in one batch, so they are sent together once
		// BatchWait is reached.
		BatchSize:     int(1 * units.MiB),
		BatchWait:     50 * time.Millisecond,
		Timeout:       time.Second,
		Client:        config.DefaultHTTPClientConfig,
		BackoffConfig: backoff.Config{MinBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond, MaxRetries: 3},
		QueueConfig:   QueueConfig{Capacity: int(10 * units.MiB), MinShards: 1, BlockOnOverflow: true, DrainTimeout: 30 * time.Second},
	}, logging.NewSlogNop(), marker.NewNopTracker())
	require.NoError(t, err)

	for _, entry := range entries {
		require.NoError(t, e.enqueue(t.Context(), entry, 0))
	}

	// Stopping the endpoint waits until the current batch is sent.
	e.stop()

	require.Equal(t, 1, len(receivedReqsChan), "expected both entries to be sent in a single batch")

	// The observed size is the uncompressed size of the log lines, which is what
	// the batch compares against the configured BatchSize.
	sum, count := histogramSumAndCount(t, reg, "loki_write_batch_size_bytes")
	assert.Equal(t, uint64(1), count)
	assert.Equal(t, float64(entries[0].Size()+entries[1].Size()), sum)
}

func TestEndpointCallerCancel(t *testing.T) {
	t.Run("entry is not queued or sent if callers context is canceled", func(t *testing.T) {
		called := atomic.NewBool(false)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called.Store(true)
		}))
		defer server.Close()

		var url flagext.URLValue
		require.NoError(t, url.Set(server.URL))

		e, err := newEndpoint(newMetrics(prometheus.NewRegistry()), Config{URL: url}, logging.NewSlogNop(), marker.NewNopTracker())
		require.NoError(t, err)
		defer e.stop()

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		require.ErrorIs(t, e.enqueue(ctx, loki.Entry{Entry: push.Entry{Line: "my entry"}}, 0), context.Canceled)
		require.Equal(t, false, called.Load())
	})

	t.Run("when queue is full and callers context is done", func(t *testing.T) {
		server, blocked, release := newBlockedServer()
		defer server.Close()
		defer release()

		var url flagext.URLValue
		require.NoError(t, url.Set(server.URL))

		e, err := newEndpoint(newMetrics(prometheus.NewRegistry()), Config{
			Name:      "test-client",
			URL:       url,
			Timeout:   time.Minute,
			BatchSize: 1,
			BackoffConfig: backoff.Config{
				MinBackoff: time.Millisecond,
				MaxBackoff: 10 * time.Millisecond,
				MaxRetries: 0,
			},
			QueueConfig: QueueConfig{
				Capacity:        1,
				MinShards:       1,
				DrainTimeout:    1 * time.Second,
				BlockOnOverflow: true,
			},
		}, logging.NewSlogNop(), marker.NewNopTracker())
		require.NoError(t, err)

		defer e.stop()

		entry := loki.Entry{Entry: push.Entry{Line: "my entry"}}

		require.NoError(t, e.enqueue(t.Context(), entry, 0))
		require.Eventually(t, func() bool { return blocked.Load() }, 3*time.Second, 100*time.Millisecond)

		require.NoError(t, e.enqueue(t.Context(), entry, 0))
		require.NoError(t, e.enqueue(t.Context(), entry, 0))

		ctx, cancel := context.WithTimeout(t.Context(), 1*time.Second)
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- e.enqueue(ctx, entry, 0)
		}()

		select {
		case err := <-done:
			require.ErrorIs(t, err, context.DeadlineExceeded)
		case <-time.After(2 * time.Second):
			t.Fatal("entry trying to be queued was not canceled in time")
		}
	})
}

// histogramSumAndCount returns the sum and count of the single series of the
// named histogram in reg.
func histogramSumAndCount(t *testing.T, reg *prometheus.Registry, name string) (float64, uint64) {
	t.Helper()

	families, err := reg.Gather()
	require.NoError(t, err)

	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		require.Len(t, family.GetMetric(), 1)
		h := family.GetMetric()[0].GetHistogram()
		return h.GetSampleSum(), h.GetSampleCount()
	}

	t.Fatalf("metric %s not found", name)
	return 0, 0
}
