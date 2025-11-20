package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/loki/util"
)

func TestFanoutConsumer(t *testing.T) {
	testEndpointConfig, rwReceivedReqs, closeServer := newServerAndEndpointConfig(t)

	consumer, err := NewFanoutConsumer(log.NewNopLogger(), prometheus.NewRegistry(), testEndpointConfig)
	require.NoError(t, err)

	receivedRequests := util.NewSyncSlice[util.RemoteWriteRequest]()
	go func() {
		for req := range rwReceivedReqs {
			receivedRequests.Append(req)
		}
	}()

	defer func() {
		consumer.Stop()
		closeServer()
	}()

	var testLabels = model.LabelSet{
		"pizza-flavour": "fugazzeta",
	}
	var totalLines = 100
	for i := range totalLines {
		consumer.Chan() <- loki.Entry{
			Labels: testLabels,
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      fmt.Sprintf("line%d", i),
			},
		}
	}

	require.Eventually(t, func() bool {
		return receivedRequests.Length() == totalLines
	}, 5*time.Second, time.Second, "timed out waiting for requests to be received")

	var seenEntries = map[string]struct{}{}
	// assert over rw received entries
	defer receivedRequests.DoneIterate()
	for _, req := range receivedRequests.StartIterate() {
		require.Len(t, req.Request.Streams, 1, "expected 1 stream requests to be received")
		require.Len(t, req.Request.Streams[0].Entries, 1, "expected 1 entry in the only stream received per request")
		require.Equal(t, `{pizza-flavour="fugazzeta"}`, req.Request.Streams[0].Labels)
		seenEntries[req.Request.Streams[0].Entries[0].Line] = struct{}{}
	}
	require.Len(t, seenEntries, totalLines)
}

func TestFanoutConsumer_MultipleConfigs(t *testing.T) {
	testEndpointConfig, rwReceivedReqs, closeServer := newServerAndEndpointConfig(t)
	testEndpointConfig2, rwReceivedReqs2, closeServer2 := newServerAndEndpointConfig(t)
	testEndpointConfig2.Name = "test-client-2"

	// start writer and consumer
	consumer, err := NewFanoutConsumer(log.NewNopLogger(), prometheus.NewRegistry(), testEndpointConfig, testEndpointConfig2)
	require.NoError(t, err)

	receivedRequests := util.NewSyncSlice[util.RemoteWriteRequest]()
	ctx, cancel := context.WithCancel(t.Context())
	go func(ctx context.Context) {
		for {
			select {
			case req := <-rwReceivedReqs:
				receivedRequests.Append(req)
			case req := <-rwReceivedReqs2:
				receivedRequests.Append(req)
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	defer func() {
		consumer.Stop()
		closeServer()
		closeServer2()
		cancel()
	}()

	var testLabels = model.LabelSet{
		"pizza-flavour": "fugazzeta",
	}
	var totalLines = 100
	for i := range totalLines {
		consumer.Chan() <- loki.Entry{
			Labels: testLabels,
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      fmt.Sprintf("line%d", i),
			},
		}
	}

	// times 2 due to endpoints being run
	expectedTotalLines := totalLines * 2
	require.Eventually(t, func() bool {
		return receivedRequests.Length() == expectedTotalLines
	}, 5*time.Second, time.Second, "timed out waiting for requests to be received")

	var seenEntries int
	// assert over rw received entries
	defer receivedRequests.DoneIterate()
	for _, req := range receivedRequests.StartIterate() {
		require.Len(t, req.Request.Streams, 1, "expected 1 stream requests to be received")
		require.Len(t, req.Request.Streams[0].Entries, 1, "expected 1 entry in the only stream received per request")
		seenEntries += 1
	}
	require.Equal(t, seenEntries, expectedTotalLines)
}

func TestFanoutConsumer_InvalidConfig(t *testing.T) {
	t.Run("no endpoints", func(t *testing.T) {
		_, err := NewFanoutConsumer(log.NewNopLogger(), prometheus.NewRegistry())
		require.Error(t, err)
	})

	t.Run("repeated endpoint", func(t *testing.T) {
		host, _ := url.Parse("http://localhost:3100")
		config := Config{URL: flagext.URLValue{URL: host}}
		_, err := NewFanoutConsumer(log.NewNopLogger(), prometheus.NewRegistry(), config, config)
		require.Error(t, err)
	})
}

func TestFanoutConsumer_NoDuplicateMetricsPanic(t *testing.T) {
	var (
		host, _ = url.Parse("http://localhost:3100")
		reg     = prometheus.NewRegistry()
	)

	require.NotPanics(t, func() {
		for range 2 {
			_, err := NewFanoutConsumer(log.NewNopLogger(), reg, Config{URL: flagext.URLValue{URL: host}})
			require.NoError(t, err)
		}
	})
}

var logEntries = []loki.Entry{
	{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Unix(1, 0).UTC(), Line: "line1"}},
	{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Unix(2, 0).UTC(), Line: "line2"}},
	{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Unix(3, 0).UTC(), Line: "line3"}},
	{Labels: model.LabelSet{"__tenant_id__": "tenant-1"}, Entry: push.Entry{Timestamp: time.Unix(4, 0).UTC(), Line: "line4"}},
	{Labels: model.LabelSet{"__tenant_id__": "tenant-1"}, Entry: push.Entry{Timestamp: time.Unix(5, 0).UTC(), Line: "line5"}},
	{Labels: model.LabelSet{"__tenant_id__": "tenant-2"}, Entry: push.Entry{Timestamp: time.Unix(6, 0).UTC(), Line: "line6"}},
	{Labels: model.LabelSet{}, Entry: push.Entry{Timestamp: time.Unix(6, 0).UTC(), Line: "line0123456789"}},
	{
		Labels: model.LabelSet{},
		Entry: push.Entry{
			Timestamp: time.Unix(7, 0).UTC(),
			Line:      "line7",
			StructuredMetadata: push.LabelsAdapter{
				{Name: "trace_id", Value: "12345"},
			},
		},
	},
}

func TestEndpoint(t *testing.T) {
	tests := map[string]struct {
		endpointConfig       Config
		serverResponseStatus int
		inputEntries         []loki.Entry
		inputDelay           time.Duration
		expectedReqs         []util.RemoteWriteRequest
		expectedMetrics      string
	}{
		"batch log entries together until the batch size is reached": {
			endpointConfig: Config{
				BatchSize: 10,
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
                               loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                               loki_write_dropped_entries_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                               loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                               loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                               # HELP loki_write_mutated_entries_total The total number of log entries that have been mutated.
                               # TYPE loki_write_mutated_entries_total counter
                               loki_write_mutated_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                               loki_write_mutated_entries_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                               loki_write_mutated_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                               loki_write_mutated_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                               # HELP loki_write_mutated_bytes_total The total number of bytes that have been mutated.
                               # TYPE loki_write_mutated_bytes_total counter
                               loki_write_mutated_bytes_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                               loki_write_mutated_bytes_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                               loki_write_mutated_bytes_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                               loki_write_mutated_bytes_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                       `,
		},
		"batch log entries together until the batch wait time is reached": {
			endpointConfig: Config{
				BatchSize: 10,
				BatchWait: 100 * time.Millisecond,
			},
			serverResponseStatus: 200,
			inputEntries:         []loki.Entry{logEntries[0], logEntries[1]},
			inputDelay:           110 * time.Millisecond,
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
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_mutated_entries_total The total number of log entries that have been mutated.
                              # TYPE loki_write_mutated_entries_total counter
                              loki_write_mutated_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_mutated_bytes_total The total number of bytes that have been mutated.
                              # TYPE loki_write_mutated_bytes_total counter
                              loki_write_mutated_bytes_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                       `,
		},
		"retry send a batch up to backoff's max retries in case the server responds with a 5xx": {
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
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 1
                              loki_write_dropped_entries_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_mutated_entries_total The total number of log entries that have been mutated.
                              # TYPE loki_write_mutated_entries_total counter
                              loki_write_mutated_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_mutated_bytes_total The total number of bytes that have been mutated.
                              # TYPE loki_write_mutated_bytes_total counter
                              loki_write_mutated_bytes_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_sent_entries_total Number of log entries sent to the ingester.
                              # TYPE loki_write_sent_entries_total counter
                              loki_write_sent_entries_total{host="__HOST__",tenant=""} 0
                       `,
		},
		"do not retry send a batch in case the server responds with a 4xx": {
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
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 1
                              loki_write_dropped_entries_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_mutated_entries_total The total number of log entries that have been mutated.
                              # TYPE loki_write_mutated_entries_total counter
                              loki_write_mutated_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_mutated_bytes_total The total number of bytes that have been mutated.
                              # TYPE loki_write_mutated_bytes_total counter
                              loki_write_mutated_bytes_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_sent_entries_total Number of log entries sent to the ingester.
                              # TYPE loki_write_sent_entries_total counter
                              loki_write_sent_entries_total{host="__HOST__",tenant=""} 0
                       `,
		},
		"do retry sending a batch in case the server responds with a 429": {
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
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 1
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_mutated_entries_total The total number of log entries that have been mutated.
                              # TYPE loki_write_mutated_entries_total counter
                              loki_write_mutated_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_mutated_bytes_total The total number of bytes that have been mutated.
                              # TYPE loki_write_mutated_bytes_total counter
                              loki_write_mutated_bytes_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_sent_entries_total Number of log entries sent to the ingester.
                              # TYPE loki_write_sent_entries_total counter
                              loki_write_sent_entries_total{host="__HOST__",tenant=""} 0
                       `,
		},
		"do not retry in case of 429 when endpoint is configured to drop rate limited batches": {
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
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 1
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_mutated_entries_total The total number of log entries that have been mutated.
                              # TYPE loki_write_mutated_entries_total counter
                              loki_write_mutated_entries_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_mutated_bytes_total The total number of bytes that have been mutated.
                              # TYPE loki_write_mutated_bytes_total counter
                              loki_write_mutated_bytes_total{host="__HOST__",reason="ingester_error",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="line_too_long",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="rate_limited",tenant=""} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="stream_limited",tenant=""} 0
                              # HELP loki_write_sent_entries_total Number of log entries sent to the ingester.
                              # TYPE loki_write_sent_entries_total counter
                              loki_write_sent_entries_total{host="__HOST__",tenant=""} 0
                       `,
		},
		"batch log entries together honoring the endpoint tenant ID": {
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
                              loki_write_dropped_entries_total{host="__HOST__", reason="ingester_error", tenant="tenant-default"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="line_too_long",tenant="tenant-default"} 0
                              loki_write_dropped_entries_total{host="__HOST__", reason="rate_limited", tenant="tenant-default"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant="tenant-default"} 0
                              # HELP loki_write_mutated_entries_total The total number of log entries that have been mutated.
                              # TYPE loki_write_mutated_entries_total counter
                              loki_write_mutated_entries_total{host="__HOST__",reason="ingester_error",tenant="tenant-default"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="line_too_long",tenant="tenant-default"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="rate_limited",tenant="tenant-default"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="stream_limited",tenant="tenant-default"} 0
                              # HELP loki_write_mutated_bytes_total The total number of bytes that have been mutated.
                              # TYPE loki_write_mutated_bytes_total counter
                              loki_write_mutated_bytes_total{host="__HOST__",reason="ingester_error",tenant="tenant-default"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="line_too_long",tenant="tenant-default"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="rate_limited",tenant="tenant-default"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="stream_limited",tenant="tenant-default"} 0
                       `,
		},
		"batch log entries together honoring the tenant ID overridden while processing the pipeline stages": {
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
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant="tenant-1"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant="tenant-2"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="ingester_error",tenant="tenant-default"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="line_too_long",tenant="tenant-1"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="line_too_long",tenant="tenant-2"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="line_too_long",tenant="tenant-default"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant="tenant-1"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant="tenant-2"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="rate_limited",tenant="tenant-default"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant="tenant-1"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant="tenant-2"} 0
                              loki_write_dropped_entries_total{host="__HOST__",reason="stream_limited",tenant="tenant-default"} 0
                              # HELP loki_write_mutated_entries_total The total number of log entries that have been mutated.
                              # TYPE loki_write_mutated_entries_total counter
                              loki_write_mutated_entries_total{host="__HOST__",reason="ingester_error",tenant="tenant-1"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="ingester_error",tenant="tenant-2"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="ingester_error",tenant="tenant-default"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="line_too_long",tenant="tenant-1"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="line_too_long",tenant="tenant-2"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="line_too_long",tenant="tenant-default"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="rate_limited",tenant="tenant-1"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="rate_limited",tenant="tenant-2"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="rate_limited",tenant="tenant-default"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="stream_limited",tenant="tenant-1"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="stream_limited",tenant="tenant-2"} 0
                              loki_write_mutated_entries_total{host="__HOST__",reason="stream_limited",tenant="tenant-default"} 0
                              # HELP loki_write_mutated_bytes_total The total number of bytes that have been mutated.
                              # TYPE loki_write_mutated_bytes_total counter
                              loki_write_mutated_bytes_total{host="__HOST__",reason="ingester_error",tenant="tenant-1"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="ingester_error",tenant="tenant-2"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="ingester_error",tenant="tenant-default"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="line_too_long",tenant="tenant-1"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="line_too_long",tenant="tenant-2"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="line_too_long",tenant="tenant-default"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="rate_limited",tenant="tenant-1"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="rate_limited",tenant="tenant-2"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="rate_limited",tenant="tenant-default"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="stream_limited",tenant="tenant-1"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="stream_limited",tenant="tenant-2"} 0
                              loki_write_mutated_bytes_total{host="__HOST__",reason="stream_limited",tenant="tenant-default"} 0
                       `,
		},
	}

	for testName, tt := range tests {
		t.Run(testName, func(t *testing.T) {
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

			m := NewMetrics(reg)
			c, err := newEndpoint(m, tt.endpointConfig, log.NewNopLogger())
			require.NoError(t, err)

			// Send all the input log entries
			for i, logEntry := range tt.inputEntries {
				c.Chan() <- logEntry

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
			c.Stop()
			close(receivedReqsChan)

			// Get all push requests received on the server side
			receivedReqs := make([]util.RemoteWriteRequest, 0)
			for req := range receivedReqsChan {
				receivedReqs = append(receivedReqs, req)
			}

			assert.ElementsMatch(t, tt.expectedReqs, receivedReqs)

			expectedMetrics := strings.ReplaceAll(tt.expectedMetrics, "__HOST__", serverURL.Host)
			err = testutil.GatherAndCompare(reg, strings.NewReader(expectedMetrics), "loki_write_sent_entries_total", "loki_write_dropped_entries_total", "loki_write_mutated_entries_total", "loki_write_mutated_bytes_total")
			assert.NoError(t, err)
		})
	}
}

func newServerAndEndpointConfig(t *testing.T) (Config, chan util.RemoteWriteRequest, func()) {
	receivedReqsChan := make(chan util.RemoteWriteRequest, 10)

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
		Queue: QueueConfig{
			Capacity:     10, // buffered channel of size 10
			DrainTimeout: time.Second * 10,
		},
	}
	return endpointConfig, receivedReqsChan, func() {
		server.Close()
		close(receivedReqsChan)
	}
}
