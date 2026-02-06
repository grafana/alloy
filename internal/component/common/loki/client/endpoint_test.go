package client

import (
	"net/http"
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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal"
	"github.com/grafana/alloy/internal/loki/util"
)

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
			tt.endpointConfig.QueueConfig.BlockOnOverflow = true
			tt.endpointConfig.QueueConfig.DrainTimeout = 30 * time.Second

			m := newMetrics(reg)
			c, err := newEndpoint(m, tt.endpointConfig, log.NewNopLogger(), internal.NewNopMarkerHandler())
			require.NoError(t, err)

			// Send all the input log entries
			for i, logEntry := range tt.inputEntries {
				c.enqueue(logEntry, 0)

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
			err = testutil.GatherAndCompare(reg, strings.NewReader(expectedMetrics), "loki_write_sent_entries_total", "loki_write_dropped_entries_total", "loki_write_mutated_entries_total", "loki_write_mutated_bytes_total")
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
			Client:    config.DefaultHTTPClientConfig,
			QueueConfig: QueueConfig{
				Capacity:        1,
				MinShards:       1,
				BlockOnOverflow: false,
			},
		}, log.NewNopLogger(), internal.NewNopMarkerHandler())
		require.NoError(t, err)
		defer e.stop()

		entry := loki.Entry{Entry: push.Entry{Line: "my entry"}}

		// NOTE: We have configured batch size to 1 so only one entry will fit in each batch.
		// To exceed the queue's capacity we need to pass 4 entries. We have one batch that we are actively trying
		// to send, one batch that is queued and one batch that we are currently working with filling up.
		require.NoError(t, e.enqueue(entry, 0))
		require.NoError(t, e.enqueue(entry, 0))
		// The third enqueue is a bit flaky because it will depend if a shard has grabbed a queued batch or not.
		// So we ignore this and the fourth one will for sure fail.
		e.enqueue(entry, 0)
		require.ErrorIs(t, e.enqueue(entry, 0), errQueueIsFull)
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
		}, log.NewNopLogger(), internal.NewNopMarkerHandler())
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
		require.NoError(t, e.enqueue(entry1, 0))
		require.NoError(t, e.enqueue(entry2, 0))
		require.NoError(t, e.enqueue(entry3, 0))
		require.NoError(t, e.enqueue(entry4, 0))
	})
}
