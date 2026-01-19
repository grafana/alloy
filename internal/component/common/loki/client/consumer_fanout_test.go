package client

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/model"
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
		QueueConfig: QueueConfig{
			Capacity:     10, // buffered channel of size 10
			DrainTimeout: time.Second * 10,
		},
	}
	return endpointConfig, receivedReqsChan, func() {
		server.Close()
		close(receivedReqsChan)
	}
}
