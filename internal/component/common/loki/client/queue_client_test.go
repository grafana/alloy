package client

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/alecthomas/units"
	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/loki/ingester/wal"
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

	"github.com/grafana/alloy/internal/component/common/loki"
	"github.com/grafana/alloy/internal/component/common/loki/client/internal"
	"github.com/grafana/alloy/internal/component/common/loki/utils"
)

type testCase struct {
	// numLines is the total number of lines sent through the client in the benchmark.
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

type nilMarkerHandler struct{}

func (n nilMarkerHandler) UpdateReceivedData(segmentId, dataCount int) {
}

func (n nilMarkerHandler) UpdateSentData(segmentId, dataCount int) {
}

func (n nilMarkerHandler) Stop() {
}

func TestQueueClient(t *testing.T) {
	for name, tc := range map[string]testCase{
		"small test": {
			numLines:  3,
			numSeries: 1,
			batchSize: 10,
			batchWait: time.Millisecond * 50,
			queueConfig: QueueConfig{
				Capacity:     100,
				DrainTimeout: time.Second,
			},
		},
		"many lines and series, immediate delivery": {
			numLines:  1000,
			numSeries: 10,
			batchSize: 10,
			batchWait: time.Millisecond * 50,
			queueConfig: QueueConfig{
				Capacity:     100,
				DrainTimeout: time.Second,
			},
		},
		"many lines and series, delivery because of batch age": {
			numLines:  100,
			numSeries: 10,
			batchSize: int(1 * units.MiB), // make batch size big enough so that all batches should be delivered because of batch age
			batchWait: time.Millisecond * 50,
			queueConfig: QueueConfig{
				Capacity:     int(100 * units.MiB), // keep buffered channel size on 100
				DrainTimeout: 10 * time.Second,
			},
			expectedRWReqsCount: 1, // expect all entries to be sent in a single batch (100 * < 10B per line) < 1MiB
		},
	} {
		t.Run(name, func(t *testing.T) {
			reg := prometheus.NewRegistry()

			// Create a buffer channel where we do enqueue received requests
			receivedReqsChan := make(chan utils.RemoteWriteRequest, 10)
			// count the number for remote-write requests received (which should correlated with the number of sent batches),
			// and the total number of entries.
			var receivedRWsCount atomic.Int64
			var receivedEntriesCount atomic.Int64

			receivedReqs := utils.NewSyncSlice[utils.RemoteWriteRequest]()
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
			server := utils.NewRemoteWriteServer(receivedReqsChan, 200)
			require.NotNil(t, server)
			defer server.Close()

			// Get the URL at which the local test server is listening to
			serverURL := flagext.URLValue{}
			err := serverURL.Set(server.URL)
			require.NoError(t, err)

			// Instance the client
			cfg := Config{
				URL:           serverURL,
				BatchWait:     tc.batchWait,
				BatchSize:     tc.batchSize,
				Client:        config.HTTPClientConfig{},
				BackoffConfig: backoff.Config{MinBackoff: 5 * time.Second, MaxBackoff: 10 * time.Second, MaxRetries: 1},
				Timeout:       1 * time.Second,
				TenantID:      "",
				Queue:         tc.queueConfig,
			}

			logger := log.NewLogfmtLogger(os.Stdout)

			qc, err := NewQueue(NewMetrics(reg), NewQueueClientMetrics(reg).CurryWithId("test"), cfg, 0, logger, nilMarkerHandler{})
			require.NoError(t, err)

			//labels := model.LabelSet{"app": "test"}
			lines := make([]string, 0, tc.numLines)
			for i := 0; i < tc.numLines; i++ {
				lines = append(lines, fmt.Sprintf("hola %d", i))
			}

			// Send all the input log entries
			for i, l := range lines {
				mod := i % tc.numSeries
				qc.StoreSeries([]record.RefSeries{
					{
						Labels: labels.New(
							labels.Label{Name: "app", Value: fmt.Sprintf("test-%d", mod)},
						),
						Ref: chunks.HeadSeriesRef(mod),
					},
				}, 0)

				_ = qc.AppendEntries(wal.RefEntries{
					Ref: chunks.HeadSeriesRef(mod),
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

			// Stop the client: it waits until the current batch is sent
			qc.Stop()
			close(receivedReqsChan)
		})
	}
}

func BenchmarkClientImplementations(b *testing.B) {
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
			b.Run("implementation=queue_nil_marker_handler", func(b *testing.B) {
				runQueueClientBenchCase(b, bc, func(t *testing.B) MarkerHandler {
					return &nilMarkerHandler{}
				})
			})

			b.Run("implementation=queue_marker_handler", func(b *testing.B) {
				runQueueClientBenchCase(b, bc, func(t *testing.B) MarkerHandler {
					dir := b.TempDir()
					nopLogger := log.NewNopLogger()

					markerFileHandler, err := internal.NewMarkerFileHandler(nopLogger, dir)
					require.NoError(b, err)

					markerHandler := internal.NewMarkerHandler(markerFileHandler, time.Minute, nopLogger, internal.NewMarkerMetrics(nil).WithCurriedId("test"))

					return markerHandler
				})
			})

			b.Run("implementation=regular", func(b *testing.B) {
				runRegularClientBenchCase(b, bc)
			})
		})
	}
}

func runQueueClientBenchCase(b *testing.B, bc testCase, mhFactory func(t *testing.B) MarkerHandler) {
	reg := prometheus.NewRegistry()

	// Create a buffer channel where we do enqueue received requests
	receivedReqsChan := make(chan utils.RemoteWriteRequest, 10)
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
	server := utils.NewRemoteWriteServer(receivedReqsChan, 200)
	require.NotNil(b, server)
	defer server.Close()

	// Get the URL at which the local test server is listening to
	serverURL := flagext.URLValue{}
	err := serverURL.Set(server.URL)
	require.NoError(b, err)

	// Instance the client
	cfg := Config{
		URL:           serverURL,
		BatchWait:     time.Millisecond * 50,
		BatchSize:     10,
		Client:        config.HTTPClientConfig{},
		BackoffConfig: backoff.Config{MinBackoff: 5 * time.Second, MaxBackoff: 10 * time.Second, MaxRetries: 1},
		Timeout:       1 * time.Second,
		TenantID:      "",
		Queue: QueueConfig{
			Capacity:     1000, // queue size of 100
			DrainTimeout: time.Second * 10,
		},
	}

	logger := log.NewLogfmtLogger(os.Stdout)

	qc, err := NewQueue(NewMetrics(reg), NewQueueClientMetrics(reg).CurryWithId("test"), cfg, 0, logger, mhFactory(b))
	require.NoError(b, err)

	//labels := model.LabelSet{"app": "test"}
	var lines []string
	for i := 0; i < bc.numLines; i++ {
		lines = append(lines, fmt.Sprintf("hola %d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Send all the input log entries
		for j, l := range lines {
			seriesId := j % bc.numSeries
			qc.StoreSeries([]record.RefSeries{
				{
					Labels: labels.New(
						// take j module bc.numSeries to evenly distribute those numSeries across all sent entries
						labels.Label{Name: "app", Value: fmt.Sprintf("series-%d", seriesId)},
					),
					Ref: chunks.HeadSeriesRef(seriesId),
				},
			}, 0)

			_ = qc.AppendEntries(wal.RefEntries{
				Ref: chunks.HeadSeriesRef(seriesId),
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

	// Stop the client: it waits until the current batch is sent
	qc.Stop()
	close(receivedReqsChan)
}

func runRegularClientBenchCase(b *testing.B, bc testCase) {
	reg := prometheus.NewRegistry()

	// Create a buffer channel where we do enqueue received requests
	receivedReqsChan := make(chan utils.RemoteWriteRequest, 10)
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
	server := utils.NewRemoteWriteServer(receivedReqsChan, 200)
	require.NotNil(b, server)
	defer server.Close()

	// Get the URL at which the local test server is listening to
	serverURL := flagext.URLValue{}
	err := serverURL.Set(server.URL)
	require.NoError(b, err)

	// Instance the client
	cfg := Config{
		URL:           serverURL,
		BatchWait:     time.Millisecond * 50,
		BatchSize:     10,
		Client:        config.HTTPClientConfig{},
		BackoffConfig: backoff.Config{MinBackoff: 5 * time.Second, MaxBackoff: 10 * time.Second, MaxRetries: 1},
		Timeout:       1 * time.Second,
		TenantID:      "",
		Queue: QueueConfig{
			Capacity:     1000, // queue size of 100
			DrainTimeout: time.Second * 10,
		},
	}

	logger := log.NewLogfmtLogger(os.Stdout)

	m := NewMetrics(reg)
	qc, err := New(m, cfg, 0, logger)
	require.NoError(b, err)

	//labels := model.LabelSet{"app": "test"}
	var lines []string
	for i := 0; i < bc.numLines; i++ {
		lines = append(lines, fmt.Sprintf("hola %d", i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Send all the input log entries
		for j, l := range lines {
			seriesId := j % bc.numSeries
			qc.Chan() <- loki.Entry{
				Labels: model.LabelSet{
					// take j module bc.numSeries to evenly distribute those numSeries across all sent entries
					"app": model.LabelValue(fmt.Sprintf("series-%d", seriesId)),
				},
				Entry: push.Entry{
					Timestamp: time.Now(),
					Line:      l,
				},
			}
		}

		require.Eventually(b, func() bool {
			return receivedEntriesCount.Load() == int64(len(lines))
		}, time.Second*10, time.Second, "timed out waiting for entries to arrive")

		// reset counters
		reset()
	}

	// Stop the client: it waits until the current batch is sent
	qc.Stop()
	close(receivedReqsChan)
}
