package client

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/alloy/internal/runtime/logging"
)

func TestFanoutConsumer(t *testing.T) {
	testEndpointConfig, rwReceivedReqs, closeServer := newServerAndEndpointConfig(t)

	consumer, err := NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), testEndpointConfig)
	require.NoError(t, err)
	consumer.Start()

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
		require.NoError(
			t,
			consumer.ConsumeEntry(
				t.Context(),
				loki.Entry{
					Labels: testLabels,
					Entry: push.Entry{
						Timestamp: time.Now(),
						Line:      fmt.Sprintf("line%d", i),
					},
				},
			),
		)
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
	consumer, err := NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), testEndpointConfig, testEndpointConfig2)
	require.NoError(t, err)
	consumer.Start()

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
		require.NoError(
			t,
			consumer.ConsumeEntry(
				t.Context(),
				loki.Entry{
					Labels: testLabels,
					Entry: push.Entry{
						Timestamp: time.Now(),
						Line:      fmt.Sprintf("line%d", i),
					},
				},
			),
		)
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

func TestFanoutConsumer_UnixSocket(t *testing.T) {
	sockPath := filepath.Join(t.TempDir(), "loki.sock")

	var received atomic.Int64
	srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err == nil && len(body) > 0 {
			received.Add(1)
		}
		w.WriteHeader(http.StatusNoContent)
	})}
	lis, err := net.Listen("unix", sockPath)
	require.NoError(t, err)
	defer lis.Close()
	go func() { _ = srv.Serve(lis) }()
	defer srv.Close()

	// The request URL keeps a TCP host:port for path and Host header; only
	// the transport dials the socket.
	pushURL, err := url.Parse("http://127.0.0.1:3100/loki/api/v1/push")
	require.NoError(t, err)
	endpointConfig := Config{
		Name:           "test-client-unix",
		URL:            flagext.URLValue{URL: pushURL},
		UnixSocketPath: sockPath,
		Timeout:        time.Second * 2,
		BatchSize:      1,
		BackoffConfig: backoff.Config{
			MaxRetries: 0,
		},
		QueueConfig: QueueConfig{
			Capacity:        10,
			DrainTimeout:    time.Second * 10,
			BlockOnOverflow: true,
		},
	}

	consumer, err := NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), endpointConfig)
	require.NoError(t, err)
	consumer.Start()
	defer consumer.Stop()

	for i := range 10 {
		require.NoError(t, consumer.ConsumeEntry(
			t.Context(),
			loki.Entry{
				Labels: model.LabelSet{"test": "unix"},
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      fmt.Sprintf("unix-line%d", i),
				},
			},
		))
	}

	require.Eventually(t, func() bool {
		return received.Load() >= 1
	}, 10*time.Second, 200*time.Millisecond, "timed out waiting for batches over the unix socket")
}

func TestFanoutConsumer_InvalidConfig(t *testing.T) {
	t.Run("no endpoints", func(t *testing.T) {
		_, err := NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry())
		require.Error(t, err)
	})

	t.Run("repeated endpoint", func(t *testing.T) {
		host, _ := url.Parse("http://localhost:3100")
		config := Config{URL: flagext.URLValue{URL: host}}
		_, err := NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), config, config)
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
			_, err := NewFanoutConsumer(logging.NewSlogNop(), reg, Config{URL: flagext.URLValue{URL: host}})
			require.NoError(t, err)
		}
	})
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
			Capacity:        10,
			DrainTimeout:    time.Second * 10,
			BlockOnOverflow: true,
		},
	}
	return endpointConfig, receivedReqsChan, func() {
		server.Close()
		close(receivedReqsChan)
	}
}

func TestFanoutConsumer_StopWithFullSendQueue(t *testing.T) {
	const drainTimeout = time.Second

	server, blocked, release := newBlockedServer()
	defer server.Close()
	defer release()

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	endpointConfig := Config{
		Name: "test-client",
		URL:  flagext.URLValue{URL: serverURL},
		// Long enough that the in-flight request stays parked for the whole test.
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
			DrainTimeout:    drainTimeout,
			BlockOnOverflow: true,
		},
	}

	consumer, err := NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), endpointConfig)
	require.NoError(t, err)
	consumer.Start()

	feedUntilBlocked(t, blocked, consumer)

	done := make(chan struct{})
	go func() {
		consumer.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * drainTimeout):
		release()
		t.Fatal("Stop did not finish in time")
	}
}

func TestFanoutConsumer_NoLeakOnFailedEndpoint(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	host, err := url.Parse("http://localhost:3100")
	require.NoError(t, err)

	var (
		cfg = Config{URL: flagext.URLValue{URL: host}}
		// HTTPClientConfig.Validate allows at most one bearer token source, so
		// creating the endpoint for this config fails.
		invalidClientCfg = Config{
			URL: flagext.URLValue{URL: host},
			Client: config.HTTPClientConfig{
				BearerToken:     "my-token",
				BearerTokenFile: "my-token-file",
			},
		}
	)

	// Using same config twice.
	_, err = NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), cfg, cfg)
	require.Error(t, err)

	// Using two different configs but endpoint cannot be created by the second one.
	_, err = NewFanoutConsumer(logging.NewSlogNop(), prometheus.NewRegistry(), cfg, invalidClientCfg)
	require.Error(t, err)
}
