package client

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/grafana/dskit/backoff"
	"github.com/grafana/dskit/flagext"
	"github.com/grafana/loki/pkg/push"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/config"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/tsdb/chunks"
	"github.com/prometheus/prometheus/tsdb/record"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"go.uber.org/goleak"

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal/savepoint"
	"github.com/grafana/alloy/internal/component/common/loki/wal"
	"github.com/grafana/alloy/internal/loki/util"
	"github.com/grafana/alloy/internal/runtime/logging"
	autil "github.com/grafana/alloy/internal/util"
)

func TestWALConsumer(t *testing.T) {
	var (
		dir       = t.TempDir()
		logger    = logging.NewSlogNop()
		reg       = prometheus.NewRegistry()
		walConfig = wal.Config{
			MaxSegmentAge: time.Second * 10,
			WatchConfig:   wal.DefaultWatchConfig,
		}
	)
	// start all necessary resources
	testEndpointConfig, rwReceivedReqs, closeServer := newServerAndEndpointConfig(t)

	wl, err := wal.New(logger, reg, dir)
	require.NoError(t, err)
	defer wl.Close()

	consumer, err := NewWALConsumer(logger, reg, wl, walConfig, testEndpointConfig)
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
		"wal_enabled": "true",
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
		require.Equal(t, `{wal_enabled="true"}`, req.Request.Streams[0].Labels)
		seenEntries[req.Request.Streams[0].Entries[0].Line] = struct{}{}
	}
	require.Len(t, seenEntries, totalLines)
}

func TestWALConsumer_MultipleConfigs(t *testing.T) {
	testEndpointConfig, rwReceivedReqs, closeServer := newServerAndEndpointConfig(t)
	testEndpointConfig2, rwReceivedReqs2, closeServer2 := newServerAndEndpointConfig(t)
	testEndpointConfig2.Name = "test-client-2"

	var (
		dir       = t.TempDir()
		logger    = logging.NewSlogNop()
		reg       = prometheus.NewRegistry()
		walConfig = wal.Config{
			WatchConfig:   wal.DefaultWatchConfig,
			MaxSegmentAge: time.Second * 10,
		}
	)

	wl, err := wal.New(logger, reg, dir)
	require.NoError(t, err)
	defer wl.Close()

	consumer, err := NewWALConsumer(logger, reg, wl, walConfig, testEndpointConfig, testEndpointConfig2)
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
			consumer.ConsumeEntry(t.Context(),
				loki.Entry{
					Labels: testLabels,
					Entry: push.Entry{
						Timestamp: time.Now(),
						Line:      fmt.Sprintf("line%d", i),
					},
				}),
		)
	}

	// times 2 due to endpoint being run
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

func TestWALConsumer_InvalidConfig(t *testing.T) {
	t.Run("no endpoints", func(t *testing.T) {
		var (
			dir       = t.TempDir()
			logger    = logging.NewSlogNop()
			reg       = prometheus.NewRegistry()
			walConfig = wal.Config{}
		)

		wl, err := wal.New(logger, reg, dir)
		require.NoError(t, err)
		defer wl.Close()

		_, err = NewWALConsumer(logger, reg, wl, walConfig)
		require.Error(t, err)
	})

	t.Run("repeated endpoints", func(t *testing.T) {
		host, _ := url.Parse("http://localhost:3100")

		var (
			dir       = t.TempDir()
			logger    = logging.NewSlogNop()
			reg       = prometheus.NewRegistry()
			walConfig = wal.Config{}
			config    = Config{URL: flagext.URLValue{URL: host}}
		)

		wl, err := wal.New(logger, reg, dir)
		require.NoError(t, err)
		defer wl.Close()

		_, err = NewWALConsumer(logger, reg, wl, walConfig, config, config)
		require.Error(t, err)
	})
}

type testCase struct {
	// numLines is the total number of lines sent through the endpoint in the benchmark.
	numLines int

	// numSeries is the different number of series to use in entries. Series are dynamically generated for each entry, but
	// would be numSeries in total, and evenly distributed.
	numSeries int

	// configs
	batchSize   int
	batchWait   time.Duration
	queueConfig QueueConfig

	// expects
	expectedRWReqsCount int64
}

func TestWALEndpoint(t *testing.T) {
	for name, tc := range map[string]testCase{
		"small test": {
			numLines:  3,
			numSeries: 1,
			batchSize: 10,
			batchWait: time.Millisecond * 50,
			queueConfig: QueueConfig{
				Capacity:        100,
				DrainTimeout:    time.Second,
				BlockOnOverflow: true,
			},
		},
		"many lines and series, immediate delivery": {
			numLines:  1000,
			numSeries: 10,
			batchSize: 10,
			batchWait: time.Millisecond * 50,
			queueConfig: QueueConfig{
				Capacity:        100,
				DrainTimeout:    time.Second,
				BlockOnOverflow: true,
			},
		},
		"many lines and series, delivery because of batch age": {
			numLines:  100,
			numSeries: 10,
			batchSize: int(1 * units.MiB), // make batch size big enough so that all batches should be delivered because of batch age
			batchWait: time.Millisecond * 50,
			queueConfig: QueueConfig{
				Capacity:        int(100 * units.MiB), // keep buffered channel size on 100
				DrainTimeout:    10 * time.Second,
				BlockOnOverflow: true,
			},
			expectedRWReqsCount: 1, // expect all entries to be sent in a single batch (100 * < 10B per line) < 1MiB
		},
	} {
		t.Run(name, func(t *testing.T) {
			reg := prometheus.NewRegistry()

			// Create a buffer channel where we do enqueue received requests
			receivedReqsChan := make(chan util.RemoteWriteRequest, 10)
			// count the number for remote-write requests received (which should correlated with the number of sent batches),
			// and the total number of entries.
			var receivedRWsCount atomic.Int64
			var receivedEntriesCount atomic.Int64

			receivedReqs := util.NewSyncSlice[util.RemoteWriteRequest]()
			go func() {
				for req := range receivedReqsChan {
					receivedReqs.Append(req)
					receivedRWsCount.Add(1)
					for _, s := range req.Request.Streams {
						receivedEntriesCount.Add(int64(len(s.Entries)))
					}
				}
			}()

			// Start a local HTTP server
			server := util.NewRemoteWriteServer(receivedReqsChan, 200)
			require.NotNil(t, server)
			defer server.Close()

			// Get the URL at which the local test server is listening to
			serverURL := flagext.URLValue{}
			err := serverURL.Set(server.URL)
			require.NoError(t, err)

			cfg := Config{
				URL:           serverURL,
				BatchWait:     tc.batchWait,
				BatchSize:     tc.batchSize,
				Client:        config.DefaultHTTPClientConfig,
				BackoffConfig: backoff.Config{MinBackoff: 5 * time.Second, MaxBackoff: 10 * time.Second, MaxRetries: 1},
				Timeout:       1 * time.Second,
				TenantID:      "",
				QueueConfig:   tc.queueConfig,
			}

			logger := autil.TestAlloyLogger(t).Slog()
			tracker := savepoint.NewNopTracker()

			endpoint, err := newEndpoint(newMetrics(reg), cfg, logger, tracker)
			require.NoError(t, err)
			adapter := newWalEndpointAdapter(endpoint, logger, newWALEndpointMetrics(reg).CurryWithId("test"), tracker)
			adapter.start()

			//labels := model.LabelSet{"app": "test"}
			lines := make([]string, 0, tc.numLines)
			for i := 0; i < tc.numLines; i++ {
				lines = append(lines, fmt.Sprintf("hola %d", i))
			}

			// Send all the input log entries
			for i, l := range lines {
				mod := i % tc.numSeries
				adapter.StoreSeries([]record.RefSeries{
					{
						Labels: labels.New(
							labels.Label{Name: "app", Value: fmt.Sprintf("test-%d", mod)},
						),
						Ref: chunks.HeadSeriesRef(mod),
					},
				}, 0)

				_ = adapter.AppendEntries(t.Context(), wal.RefEntries{
					Ref:     chunks.HeadSeriesRef(mod),
					Created: time.Now().UnixMicro(),
					Entries: []push.Entry{{
						Timestamp: time.Now(),
						Line:      l,
					}},
				}, 0)
			}

			require.Eventually(t, func() bool {
				return receivedEntriesCount.Load() == int64(len(lines))
			}, time.Second*10, time.Second, "timed out waiting for entries to arrive")

			if tc.expectedRWReqsCount != 0 {
				require.Equal(t, tc.expectedRWReqsCount, receivedRWsCount.Load(), "number for remote write request not expected")
			}

			// Stop the endpoint: it waits until the current batch is sent
			adapter.stop()
			close(receivedReqsChan)
		})
	}
}

func BenchmarkEndpointImplementations(b *testing.B) {
	for name, bc := range map[string]testCase{
		"100 entries, single series, no batching": {
			numLines:  100,
			numSeries: 1,
			batchSize: 10,
			batchWait: time.Millisecond * 50,
			queueConfig: QueueConfig{
				Capacity:     1000, // buffer size 100
				DrainTimeout: time.Second,
			},
		},
		"100k entries, 100 series, default batching": {
			numLines:  100_000,
			numSeries: 100,
			batchSize: int(1 * units.MiB),
			batchWait: time.Second,
			queueConfig: QueueConfig{
				Capacity:     int(10 * units.MiB), // buffer size 100
				DrainTimeout: 5 * time.Second,
			},
		},
	} {
		b.Run(name, func(b *testing.B) {
			b.Run("implementation=wal_nop_tracker", func(b *testing.B) {
				runWALEndpointBenchCase(b, bc, func(t *testing.B) savepoint.Tracker {
					return savepoint.NewNopTracker()
				})
			})

			b.Run("implementation=wal_segment_tracker", func(b *testing.B) {
				runWALEndpointBenchCase(b, bc, func(t *testing.B) savepoint.Tracker {
					dir := b.TempDir()
					nopLogger := logging.NewSlogNop()

					f, err := savepoint.NewFile(nopLogger, dir)
					require.NoError(b, err)

					return savepoint.NewSegmentTracker(f, "test", time.Minute, nopLogger, savepoint.NewMetrics(nil))
				})
			})

			b.Run("implementation=regular", func(b *testing.B) {
				runEndpointBenchCase(b, bc)
			})
		})
	}
}

func runWALEndpointBenchCase(b *testing.B, bc testCase, trackerFactory func(t *testing.B) savepoint.Tracker) {
	reg := prometheus.NewRegistry()

	// Create a buffer channel where we do enqueue received requests
	receivedReqsChan := make(chan util.RemoteWriteRequest, 10)
	// count the number for remote-write requests received (which should correlated with the number of sent batches),
	// and the total number of entries.
	var receivedEntriesCount atomic.Int64
	reset := func() {
		receivedEntriesCount.Store(0)
	}

	go func() {
		for req := range receivedReqsChan {
			for _, s := range req.Request.Streams {
				receivedEntriesCount.Add(int64(len(s.Entries)))
			}
		}
	}()

	// Start a local HTTP server
	server := util.NewRemoteWriteServer(receivedReqsChan, 200)
	require.NotNil(b, server)
	defer server.Close()

	// Get the URL at which the local test server is listening to
	serverURL := flagext.URLValue{}
	err := serverURL.Set(server.URL)
	require.NoError(b, err)

	cfg := Config{
		URL:           serverURL,
		BatchWait:     time.Millisecond * 50,
		BatchSize:     10,
		Client:        config.DefaultHTTPClientConfig,
		BackoffConfig: backoff.Config{MinBackoff: 5 * time.Second, MaxBackoff: 10 * time.Second, MaxRetries: 1},
		Timeout:       1 * time.Second,
		TenantID:      "",
		QueueConfig: QueueConfig{
			Capacity:     1000, // queue size of 100
			DrainTimeout: time.Second * 10,
		},
	}

	logger := logging.NewSlogNop()
	tracker := trackerFactory(b)

	endpoint, err := newEndpoint(newMetrics(reg), cfg, logger, tracker)
	require.NoError(b, err)
	adapter := newWalEndpointAdapter(endpoint, logger, newWALEndpointMetrics(reg).CurryWithId("test"), tracker)
	adapter.start()

	//labels := model.LabelSet{"app": "test"}
	var lines []string
	for i := 0; i < bc.numLines; i++ {
		lines = append(lines, fmt.Sprintf("hola %d", i))
	}

	for b.Loop() {
		// Send all the input log entries
		for j, l := range lines {
			seriesId := j % bc.numSeries
			adapter.StoreSeries([]record.RefSeries{
				{
					Labels: labels.New(
						// take j module bc.numSeries to evenly distribute those numSeries across all sent entries
						labels.Label{Name: "app", Value: fmt.Sprintf("series-%d", seriesId)},
					),
					Ref: chunks.HeadSeriesRef(seriesId),
				},
			}, 0)

			_ = adapter.AppendEntries(b.Context(), wal.RefEntries{
				Ref:     chunks.HeadSeriesRef(seriesId),
				Created: time.Now().UnixMicro(),
				Entries: []push.Entry{{
					Timestamp: time.Now(),
					Line:      l,
				}},
			}, 0)
		}

		require.Eventually(b, func() bool {
			return receivedEntriesCount.Load() == int64(len(lines))
		}, time.Second*10, time.Second, "timed out waiting for entries to arrive")

		// reset counters
		reset()
	}

	// Stop the endpoint: it waits until the current batch is sent
	adapter.stop()
	close(receivedReqsChan)
}

func runEndpointBenchCase(b *testing.B, bc testCase) {
	reg := prometheus.NewRegistry()

	// Create a buffer channel where we do enqueue received requests
	receivedReqsChan := make(chan util.RemoteWriteRequest, 10)
	// count the number for remote-write requests received (which should correlated with the number of sent batches),
	// and the total number of entries.
	var receivedEntriesCount atomic.Int64
	reset := func() {
		receivedEntriesCount.Store(0)
	}

	go func() {
		for req := range receivedReqsChan {
			for _, s := range req.Request.Streams {
				receivedEntriesCount.Add(int64(len(s.Entries)))
			}
		}
	}()

	// Start a local HTTP server
	server := util.NewRemoteWriteServer(receivedReqsChan, 200)
	require.NotNil(b, server)
	defer server.Close()

	// Get the URL at which the local test server is listening to
	serverURL := flagext.URLValue{}
	err := serverURL.Set(server.URL)
	require.NoError(b, err)

	cfg := Config{
		URL:           serverURL,
		BatchWait:     time.Millisecond * 50,
		BatchSize:     10,
		Client:        config.DefaultHTTPClientConfig,
		BackoffConfig: backoff.Config{MinBackoff: 5 * time.Second, MaxBackoff: 10 * time.Second, MaxRetries: 1},
		Timeout:       1 * time.Second,
		TenantID:      "",
		QueueConfig: QueueConfig{
			Capacity:     1000, // queue size of 100
			DrainTimeout: time.Second * 10,
		},
	}

	m := newMetrics(reg)
	endpoint, err := newEndpoint(m, cfg, logging.NewSlogNop(), savepoint.NewNopTracker())
	require.NoError(b, err)
	endpoint.start()

	//labels := model.LabelSet{"app": "test"}
	var lines []string
	for i := 0; i < bc.numLines; i++ {
		lines = append(lines, fmt.Sprintf("hola %d", i))
	}

	for b.Loop() {
		// Send all the input log entries
		for j, l := range lines {
			seriesId := j % bc.numSeries
			endpoint.enqueue(b.Context(), loki.Entry{
				Labels: model.LabelSet{
					// take j module bc.numSeries to evenly distribute those numSeries across all sent entries
					"app": model.LabelValue(fmt.Sprintf("series-%d", seriesId)),
				},
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      l,
				},
			}, 0)
		}

		require.Eventually(b, func() bool {
			return receivedEntriesCount.Load() == int64(len(lines))
		}, time.Second*10, time.Second, "timed out waiting for entries to arrive")

		// reset counters
		reset()
	}

	// Stop the endpoint: it waits until the current batch is sent
	endpoint.stop()
	close(receivedReqsChan)
}

func TestWALConsumer_StopWithFullSendQueue(t *testing.T) {
	const (
		drainTimeout = time.Second
	)

	server, blocked, release := newBlockedServer()
	defer server.Close()
	defer release()

	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	dir := t.TempDir()
	walConfig := wal.Config{
		MaxSegmentAge: time.Minute,
		WatchConfig: wal.WatchConfig{
			MinReadFrequency: 10 * time.Millisecond,
			MaxReadFrequency: 50 * time.Millisecond,
			DrainTimeout:     drainTimeout,
		},
	}

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

	var (
		logger = logging.NewSlogNop()
		reg    = prometheus.NewRegistry()
	)

	wl, err := wal.New(logger, reg, dir)
	require.NoError(t, err)
	defer wl.Close()

	consumer, err := NewWALConsumer(logger, reg, wl, walConfig, endpointConfig)
	require.NoError(t, err)
	consumer.Start()

	feedUntilBlocked(t, blocked, consumer)

	stopped := make(chan struct{})
	go func() {
		consumer.StopAndDrain()
		close(stopped)
	}()

	// A healthy shutdown costs at most the WAL drain timeout plus the queue drain timeout.
	select {
	case <-stopped:
	case <-time.After(5 * (drainTimeout + drainTimeout)):
		release()
		t.Fatal("StopAndDrain did not finish in time")
	}
}

func TestWALConsumer_NoLeakOnFailedEndpoint(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	host, err := url.Parse("http://localhost:3100")
	require.NoError(t, err)

	var (
		dir       = t.TempDir()
		logger    = logging.NewSlogNop()
		reg       = prometheus.NewRegistry()
		walConfig = wal.Config{
			MaxSegmentAge: time.Second * 10,
			WatchConfig:   wal.DefaultWatchConfig,
		}
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

	wl, err := wal.New(logger, reg, dir)
	require.NoError(t, err)
	defer wl.Close()

	// Using same config twice.
	_, err = NewWALConsumer(logger, reg, wl, walConfig, cfg, cfg)
	require.Error(t, err)

	// Using two different configs but endpoint cannot be created by the second one.
	_, err = NewWALConsumer(logger, reg, wl, walConfig, cfg, invalidClientCfg)
	require.Error(t, err)
}
