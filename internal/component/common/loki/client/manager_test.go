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
	"github.com/grafana/alloy/internal/component/common/loki/utils"
	"github.com/grafana/alloy/internal/component/common/loki/wal"
)

var nilMetrics = NewMetrics(nil)

// TestManager_NoDuplicateMetricsPanic ensures that creating two managers does
// not lead to duplicate metrics registration.
func TestManager_NoDuplicateMetricsPanic(t *testing.T) {
	var (
		host, _ = url.Parse("http://localhost:3100")

		reg     = prometheus.NewRegistry()
		metrics = NewMetrics(reg)
	)

	require.NotPanics(t, func() {
		for range 2 {
			_, err := NewManager(metrics, log.NewNopLogger(), reg, wal.Config{
				WatchConfig: wal.DefaultWatchConfig,
			}, Config{
				URL: flagext.URLValue{URL: host},
			})
			require.NoError(t, err)
		}
	})
}

func TestManager_ErrorCreatingWhenNoClientConfigsProvided(t *testing.T) {
	for _, walEnabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("wal-enabled = %t", walEnabled), func(t *testing.T) {
			walDir := t.TempDir()
			_, err := NewManager(nilMetrics, log.NewNopLogger(), prometheus.NewRegistry(), wal.Config{
				Dir:         walDir,
				Enabled:     walEnabled,
				WatchConfig: wal.DefaultWatchConfig,
			})
			require.Error(t, err)
		})
	}
}

func TestManager_ErrorCreatingWhenRepeatedConfigs(t *testing.T) {
	host1, _ := url.Parse("http://localhost:3100")
	config1 := Config{
		BatchSize: 20,
		BatchWait: 1 * time.Second,
		URL:       flagext.URLValue{URL: host1},
	}
	config1Copy := config1
	for _, walEnabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("wal-enabled = %t", walEnabled), func(t *testing.T) {
			walDir := t.TempDir()
			_, err := NewManager(nilMetrics, log.NewNopLogger(), prometheus.NewRegistry(), wal.Config{
				Dir:         walDir,
				Enabled:     walEnabled,
				WatchConfig: wal.DefaultWatchConfig,
			}, config1, config1Copy)
			require.Error(t, err)
		})
	}
}

type closer interface {
	Close()
}

type closerFunc func()

func (c closerFunc) Close() {
	c()
}

func newServerAndClientConfig(t *testing.T) (Config, chan utils.RemoteWriteRequest, closer) {
	receivedReqsChan := make(chan utils.RemoteWriteRequest, 10)

	// Start a local HTTP server
	server := utils.NewRemoteWriteServer(receivedReqsChan, http.StatusOK)
	require.NotNil(t, server)

	testClientURL, _ := url.Parse(server.URL)
	testClientConfig := Config{
		Name:      "test-client",
		URL:       flagext.URLValue{URL: testClientURL},
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
	return testClientConfig, receivedReqsChan, closerFunc(func() {
		server.Close()
		close(receivedReqsChan)
	})
}

func TestManager_WALEnabled(t *testing.T) {
	walDir := t.TempDir()
	walConfig := wal.Config{
		Dir:           walDir,
		Enabled:       true,
		MaxSegmentAge: time.Second * 10,
		WatchConfig:   wal.DefaultWatchConfig,
	}
	// start all necessary resources
	reg := prometheus.NewRegistry()
	logger := log.NewNopLogger()
	testClientConfig, rwReceivedReqs, closeServer := newServerAndClientConfig(t)
	clientMetrics := NewMetrics(reg)

	manager, err := NewManager(clientMetrics, logger, prometheus.NewRegistry(), walConfig, testClientConfig)
	require.NoError(t, err)

	receivedRequests := utils.NewSyncSlice[utils.RemoteWriteRequest]()
	go func() {
		for req := range rwReceivedReqs {
			receivedRequests.Append(req)
		}
	}()

	defer func() {
		manager.Stop()
		closeServer.Close()
	}()

	var testLabels = model.LabelSet{
		"wal_enabled": "true",
	}
	var totalLines = 100
	for i := range totalLines {
		manager.Chan() <- loki.Entry{
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
	// assert over rw client received entries
	defer receivedRequests.DoneIterate()
	for _, req := range receivedRequests.StartIterate() {
		require.Len(t, req.Request.Streams, 1, "expected 1 stream requests to be received")
		require.Len(t, req.Request.Streams[0].Entries, 1, "expected 1 entry in the only stream received per request")
		require.Equal(t, `{wal_enabled="true"}`, req.Request.Streams[0].Labels)
		seenEntries[req.Request.Streams[0].Entries[0].Line] = struct{}{}
	}
	require.Len(t, seenEntries, totalLines)
}

func TestManager_WALDisabled(t *testing.T) {
	walConfig := wal.Config{}
	// start all necessary resources
	reg := prometheus.NewRegistry()
	logger := log.NewNopLogger()
	testClientConfig, rwReceivedReqs, closeServer := newServerAndClientConfig(t)
	clientMetrics := NewMetrics(reg)

	// start writer and manager
	manager, err := NewManager(clientMetrics, logger, prometheus.NewRegistry(), walConfig, testClientConfig)
	require.NoError(t, err)

	receivedRequests := utils.NewSyncSlice[utils.RemoteWriteRequest]()
	go func() {
		for req := range rwReceivedReqs {
			receivedRequests.Append(req)
		}
	}()

	defer func() {
		manager.Stop()
		closeServer.Close()
	}()

	var testLabels = model.LabelSet{
		"pizza-flavour": "fugazzeta",
	}
	var totalLines = 100
	for i := range totalLines {
		manager.Chan() <- loki.Entry{
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
	// assert over rw client received entries
	defer receivedRequests.DoneIterate()
	for _, req := range receivedRequests.StartIterate() {
		require.Len(t, req.Request.Streams, 1, "expected 1 stream requests to be received")
		require.Len(t, req.Request.Streams[0].Entries, 1, "expected 1 entry in the only stream received per request")
		require.Equal(t, `{pizza-flavour="fugazzeta"}`, req.Request.Streams[0].Labels)
		seenEntries[req.Request.Streams[0].Entries[0].Line] = struct{}{}
	}
	require.Len(t, seenEntries, totalLines)
}

func TestManager_WALDisabled_MultipleConfigs(t *testing.T) {
	walConfig := wal.Config{}
	// start all necessary resources
	reg := prometheus.NewRegistry()
	logger := log.NewNopLogger()
	testClientConfig, rwReceivedReqs, closeServer := newServerAndClientConfig(t)

	testClientConfig2, rwReceivedReqs2, closeServer2 := newServerAndClientConfig(t)
	testClientConfig2.Name = "test-client-2"

	clientMetrics := NewMetrics(reg)

	// start writer and manager
	manager, err := NewManager(clientMetrics, logger, prometheus.NewRegistry(), walConfig, testClientConfig, testClientConfig2)
	require.NoError(t, err)

	receivedRequests := utils.NewSyncSlice[utils.RemoteWriteRequest]()
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
		manager.Stop()
		closeServer.Close()
		closeServer2.Close()
		cancel()
	}()

	var testLabels = model.LabelSet{
		"pizza-flavour": "fugazzeta",
	}
	var totalLines = 100
	for i := range totalLines {
		manager.Chan() <- loki.Entry{
			Labels: testLabels,
			Entry: push.Entry{
				Timestamp: time.Now(),
				Line:      fmt.Sprintf("line%d", i),
			},
		}
	}

	// times 2 due to clients being run
	expectedTotalLines := totalLines * 2
	require.Eventually(t, func() bool {
		return receivedRequests.Length() == expectedTotalLines
	}, 5*time.Second, time.Second, "timed out waiting for requests to be received")

	var seenEntries int
	// assert over rw client received entries
	defer receivedRequests.DoneIterate()
	for _, req := range receivedRequests.StartIterate() {
		require.Len(t, req.Request.Streams, 1, "expected 1 stream requests to be received")
		require.Len(t, req.Request.Streams[0].Entries, 1, "expected 1 entry in the only stream received per request")
		seenEntries += 1
	}
	require.Equal(t, seenEntries, expectedTotalLines)
}
