package client

import (
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

			m := NewMetrics(reg)
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
