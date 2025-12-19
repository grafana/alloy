package prometheus_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/go-kit/log"
	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"golang.org/x/exp/maps"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/component/prometheus/appenders"
	"github.com/grafana/alloy/internal/component/prometheus/relabel"
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite"
	"github.com/grafana/alloy/internal/component/prometheus/scrape"
	"github.com/grafana/alloy/internal/runtime/componenttest"
	"github.com/grafana/alloy/internal/runtime/logging"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/grafana/alloy/internal/service/livedebugging"
	"github.com/grafana/alloy/internal/static/metrics/wal"
	"github.com/grafana/alloy/internal/util"
	"github.com/grafana/alloy/internal/util/testappender"
	"github.com/grafana/alloy/syntax"
)

// This test simulates a scrape -> remote_write pipeline, without actually scraping
func TestPipeline(t *testing.T) {
	tests := []struct {
		name          string
		useLabelStore bool
	}{
		{"without_labelstore", false},
		{"with_labelstore", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destination := testappender.NewCollectingAppender()
			cfg := refTrackingConfig{useLabelStore: tt.useLabelStore}
			pipeline, ls, _ := newRemoteWritePipeline(t, util.TestLogger(t), 1, destination, cfg)
			sampleTimestamp := time.Now().UnixMilli()

			// Send metrics to our component. These will be written to the WAL and to the collecting appender
			lset1 := labels.FromStrings("foo", "bar")
			ref1 := sendMetric(t, pipeline.Appender(t.Context()), lset1, sampleTimestamp, 12)
			require.NotZero(t, ref1)

			lset2 := labels.FromStrings("fizz", "buzz")
			ref2 := sendMetric(t, pipeline.Appender(t.Context()), lset2, sampleTimestamp, 34)
			require.NotZero(t, ref2)

			expect := []*testappender.MetricSample{{
				Labels:    lset1,
				Timestamp: sampleTimestamp,
				Value:     12,
			}, {
				Labels:    lset2,
				Timestamp: sampleTimestamp,
				Value:     34,
			}}

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				require.Len(t, destination.CollectedSamples(), 2)
				require.ElementsMatch(t, expect, maps.Values(destination.CollectedSamples()))
			}, 5*time.Second, 10*time.Millisecond, "timed out waiting for metrics to be written to destination")

			if tt.useLabelStore {
				ref := ls.GetOrAddGlobalRefID(lset1)
				require.NotZero(t, ref)
				// Append result ref should match the labelstore ref
				require.Equal(t, ref, ref1)
				localRef := ls.GetLocalRefID("prometheus.remote_write.test", ref)
				require.NotZero(t, localRef)

				ref = ls.GetOrAddGlobalRefID(lset2)
				require.NotZero(t, ref)
				// Append result ref should match the labelstore ref
				require.Equal(t, ref, ref2)
				localRef = ls.GetLocalRefID("prometheus.remote_write.test", ref)
				require.NotZero(t, localRef)
			}
		})
	}
}

// This test simulates a scrape -> relabel -> remote_write pipeline, without actually scraping
func TestRelabelPipeline(t *testing.T) {
	tests := []struct {
		name          string
		useLabelStore bool
	}{
		{"without_labelstore", false},
		{"with_labelstore", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destination := testappender.NewCollectingAppender()
			cfg := refTrackingConfig{useLabelStore: tt.useLabelStore}
			pipeline, ls, _ := newRelabelPipeline(t, util.TestLogger(t), destination, cfg)
			sampleTimestamp := time.Now().UnixMilli()

			// Send metrics to our component. These will be written to the WAL and to the collecting appender
			lset1 := labels.FromStrings("foo", "bar")
			ref1 := sendMetric(t, pipeline.Appender(t.Context()), lset1, sampleTimestamp, 12)
			require.NotZero(t, ref1)

			lset2 := labels.FromStrings("fizz", "buzz")
			ref2 := sendMetric(t, pipeline.Appender(t.Context()), lset2, sampleTimestamp, 34)
			require.NotZero(t, ref2)

			expect := []*testappender.MetricSample{{
				Labels:    labels.NewBuilder(lset1).Set("lbl", "foo").Labels(),
				Timestamp: sampleTimestamp,
				Value:     12,
			}, {
				Labels:    labels.NewBuilder(lset2).Set("lbl", "foo").Labels(),
				Timestamp: sampleTimestamp,
				Value:     34,
			}}

			require.EventuallyWithT(t, func(t *assert.CollectT) {
				require.Len(t, destination.CollectedSamples(), 2)
				require.ElementsMatch(t, expect, maps.Values(destination.CollectedSamples()))
			}, 5*time.Second, 100*time.Millisecond, "timed out waiting for metrics to be written to destination")

			if tt.useLabelStore {
				ref := ls.GetOrAddGlobalRefID(lset1)
				require.NotZero(t, ref)
				// Append result ref should match the labelstore ref
				require.Equal(t, ref, ref1)
				// This was relabeled, so we shouldn't have a local ref
				localRef := ls.GetLocalRefID("prometheus.remote_write.test", ref)
				require.Zero(t, localRef)

				ref = ls.GetOrAddGlobalRefID(lset2)
				require.NotZero(t, ref)
				// Append result ref should match the labelstore ref
				require.Equal(t, ref, ref2)

				// This was relabeled, so we shouldn't have a local ref
				localRef = ls.GetLocalRefID("prometheus.remote_write.test", ref)
				require.Zero(t, localRef)

				lset1Relabeled := labels.NewBuilder(lset1).Set("lbl", "foo").Labels()
				ref = ls.GetOrAddGlobalRefID(lset1Relabeled)
				require.NotZero(t, ref)
				localRef = ls.GetLocalRefID("prometheus.remote_write.test", ref)
				require.NotZero(t, localRef)

				lset2Relabeled := labels.NewBuilder(lset2).Set("lbl", "foo").Labels()
				ref = ls.GetOrAddGlobalRefID(lset2Relabeled)
				require.NotZero(t, ref)
				localRef = ls.GetLocalRefID("prometheus.remote_write.test", ref)
				require.NotZero(t, localRef)
			}
		})
	}
}

type refTrackingConfig struct {
	useLabelStore bool

	// forceUniqueRemoteWriteRefs will ensure when multiple remote write components are used that they will
	// generate different refs for the same labelset. This is only respected when not using labelstore and is meant
	// to simulate the "worst case" scenario when not using labelstore with multiple remote_write components.
	forceUniqueRemoteWriteRefs bool
}

func (ref refTrackingConfig) TestNameString() string {
	if ref.useLabelStore {
		return "labelstore"
	}

	if ref.forceUniqueRemoteWriteRefs {
		return "seriestracking-uniquerefs"
	}

	return "seriestracking-samerefs"
}

// go test -bench="BenchmarkPipelines" ./internal/component/prometheus -run ^$ -benchmem -count 6 -benchtime 5s | tee benchmarks
// benchstat -row ".name /pipeline /remotewritecomponents /metrics" -filter "/metrics:(10 OR 1000)" -col /reftrackingconfig benchmarks
// benchstat -row ".name /pipeline /remotewritecomponents /concurrency" -filter "/concurrency:(10 OR 1000)" -col /reftrackingconfig benchmarks
func BenchmarkPipelines(b *testing.B) {
	pipelineTypes := []struct {
		name            string
		pipelineBuilder func(t testing.TB, logger log.Logger, rwComponents int, refTrackingConfig refTrackingConfig) (storage.Appendable, labelstore.LabelStore, clearCacheFunc)
	}{
		{
			name: "remote_write",
			pipelineBuilder: func(t testing.TB, logger log.Logger, rwComponents int, refTrackingConfig refTrackingConfig) (storage.Appendable, labelstore.LabelStore, clearCacheFunc) {
				return newRemoteWritePipeline(t, logger, rwComponents, appenders.Noop{}, refTrackingConfig)
			},
		},
		{
			name: "relabel-remote_write",
			pipelineBuilder: func(t testing.TB, logger log.Logger, _ int, refTrackingConfig refTrackingConfig) (storage.Appendable, labelstore.LabelStore, clearCacheFunc) {
				return newRelabelPipeline(t, logger, appenders.Noop{}, refTrackingConfig)
			},
		},
	}

	testConfigs := []struct {
		numberOfRWComponents int
		refTrackingConfig    refTrackingConfig
		skipFor              map[string]struct{}
	}{
		{
			numberOfRWComponents: 1,
			refTrackingConfig: refTrackingConfig{
				useLabelStore: true,
			},
		},
		{
			numberOfRWComponents: 1,
			refTrackingConfig: refTrackingConfig{
				useLabelStore:              false,
				forceUniqueRemoteWriteRefs: false,
			},
			skipFor: map[string]struct{}{"relabel-remote_write": {}},
		},
		{
			numberOfRWComponents: 1,
			refTrackingConfig: refTrackingConfig{
				useLabelStore:              false,
				forceUniqueRemoteWriteRefs: true,
			},
		},
		{
			numberOfRWComponents: 2,
			refTrackingConfig: refTrackingConfig{
				useLabelStore: true,
			},
			skipFor: map[string]struct{}{"relabel-remote_write": {}},
		},
		{
			numberOfRWComponents: 2,
			refTrackingConfig: refTrackingConfig{
				useLabelStore:              false,
				forceUniqueRemoteWriteRefs: false,
			},
			skipFor: map[string]struct{}{"relabel-remote_write": {}},
		},
		{
			numberOfRWComponents: 2,
			refTrackingConfig: refTrackingConfig{
				useLabelStore:              false,
				forceUniqueRemoteWriteRefs: true,
			},
			skipFor: map[string]struct{}{"relabel-remote_write": {}},
		},
	}

	// Simulates appending various numbers of new metrics sequentially
	numberOfMetrics := []int{1000}
	for _, n := range numberOfMetrics {
		for _, pipelineType := range pipelineTypes {
			for _, config := range testConfigs {
				if _, skip := config.skipFor[pipelineType.name]; skip {
					continue
				}

				testName := fmt.Sprintf("pipeline=%s/remotewritecomponents=%d/reftrackingconfig=%s/new-metrics=%d",
					pipelineType.name, config.numberOfRWComponents, config.refTrackingConfig.TestNameString(), n)

				b.Run(testName, func(b *testing.B) {
					pipeline, _, clearCache := pipelineType.pipelineBuilder(b, log.NewNopLogger(), config.numberOfRWComponents, config.refTrackingConfig)
					metrics := setupMetrics(n)
					b.ReportAllocs()
					b.ResetTimer()

					for b.Loop() {
						b.StopTimer()
						clearCache()
						b.StartTimer()

						for i := 0; i < n; i++ {
							a := pipeline.Appender(b.Context())
							for i, metric := range metrics {
								_, err := a.Append(0, metric, time.Now().UnixMilli(), float64(i))
								require.NoError(b, err)
							}
							require.NoError(b, a.Commit())
						}
					}
				})
			}
		}
	}

	// Simulate concurrently appending from multiple scrapers for known metrics
	concurrency := []int{1000}
	for _, c := range concurrency {
		for _, pipelineType := range pipelineTypes {
			for _, config := range testConfigs {
				if _, skip := config.skipFor[pipelineType.name]; skip {
					continue
				}
				testName := fmt.Sprintf("pipeline=%s/remotewritecomponents=%d/reftrackingconfig=%s/concurrent-existing-metrics=%d",
					pipelineType.name, config.numberOfRWComponents, config.refTrackingConfig.TestNameString(), c)

				b.Run(testName, func(b *testing.B) {
					pipeline, _, _ := pipelineType.pipelineBuilder(b, log.NewNopLogger(), config.numberOfRWComponents, config.refTrackingConfig)
					var metricsForAppenders [][]labels.Labels
					numMetrics := 1000
					for appenderIndex := range c {
						metrics := setupMetrics(numMetrics, fmt.Sprintf("concurrency-%d", appenderIndex))

						// Send them through once so further appends can use "known refs"
						a := pipeline.Appender(b.Context())
						for metricIndex, metric := range metrics {
							expectedRef := storage.SeriesRef(appenderIndex*numMetrics + metricIndex + 1)
							ref, err := a.Append(expectedRef, metric, time.Now().UnixMilli(), float64(metricIndex))
							require.NoError(b, err)
							require.Equal(b, expectedRef, ref)
						}
						require.NoError(b, a.Commit())

						metricsForAppenders = append(metricsForAppenders, metrics)
					}
					b.ReportAllocs()
					b.ResetTimer()

					for b.Loop() {
						var wg sync.WaitGroup

						for appenderIndex := 0; appenderIndex < c; appenderIndex++ {
							wg.Add(1)
							go func(appenderIndex int) {
								defer wg.Done()

								a := pipeline.Appender(b.Context())
								for metricIndex, metric := range metricsForAppenders[appenderIndex] {
									ref := storage.SeriesRef(appenderIndex*numMetrics + metricIndex + 1)
									_, err := a.Append(ref, metric, time.Now().UnixMilli(), float64(metricIndex))
									require.NoError(b, err)
								}
								require.NoError(b, a.Commit())
							}(appenderIndex)
						}

						wg.Wait()
					}
				})
			}
		}
	}
}

func setupMetrics(numberOfMetrics int, extraLabels ...string) []labels.Labels {
	metrics := make([]labels.Labels, 0, numberOfMetrics)
	for i := 0; i < numberOfMetrics; i++ {
		key := fmt.Sprintf("metric-%d", i)
		value := fmt.Sprintf("%d", i)
		lbls := labels.FromStrings(key, value)
		for _, extraLabel := range extraLabels {
			lbls = labels.NewBuilder(lbls).Set(extraLabel, "value").Labels()
		}
		metrics = append(metrics, lbls)
	}
	return metrics
}

type clearCacheFunc = func()

func newRemoteWritePipeline(t testing.TB, logger log.Logger, numberOfRemoteWriteComponents int, destination storage.Appender, config refTrackingConfig) (storage.Appendable, labelstore.LabelStore, clearCacheFunc) {
	t.Setenv("ALLOY_USE_LABEL_STORE", fmt.Sprintf("%v", config.useLabelStore))
	ls := labelstore.New(logger, promclient.DefaultRegisterer)

	destAppendable := testappender.ConstantAppendable{Inner: destination}

	rwAppendables := make([]storage.Appendable, 0, numberOfRemoteWriteComponents)
	for i := 0; i < numberOfRemoteWriteComponents; i++ {
		rwAppendable := newRemoteWriteComponent(t, logger, ls, destAppendable)

		// We force uniqueRemoteWriteRefs by appending 0 - n dummy metrics to each remote write component. This
		// will ensure each component is not starting at the same ref number and will hand out different refs for the same labelset.
		if config.forceUniqueRemoteWriteRefs && !config.useLabelStore {
			app := rwAppendable.Appender(t.Context())
			for j := 0; j < i; j++ {
				_, err := app.Append(0, labels.FromStrings(fmt.Sprintf("%d", j+1), "ref"), time.Now().UnixMilli(), 0)
				require.NoError(t, err)
			}
			require.NoError(t, app.Commit())
		}

		rwAppendables = append(rwAppendables, rwAppendable)
	}
	pipelineAppendable := prometheus.NewFanout(rwAppendables, "", promclient.DefaultRegisterer, ls)
	scrapeInterceptor := scrape.NewInterceptor("prometheus.scrape.test", livedebugging.NewLiveDebugging(), pipelineAppendable)

	return scrapeInterceptor, ls, func() { pipelineAppendable.Clear() }
}

func newRelabelPipeline(t testing.TB, logger log.Logger, destination storage.Appender, config refTrackingConfig) (storage.Appendable, labelstore.LabelStore, clearCacheFunc) {
	t.Setenv("ALLOY_USE_LABEL_STORE", fmt.Sprintf("%v", config.useLabelStore))
	ls := labelstore.New(logger, promclient.DefaultRegisterer)

	destAppendable := testappender.ConstantAppendable{Inner: destination}
	rwAppendable := newRemoteWriteComponent(t, logger, ls, destAppendable)
	relabelAppendable := newRelabelComponent(t, logger, []storage.Appendable{rwAppendable}, ls)
	pipelineAppendable := prometheus.NewFanout([]storage.Appendable{relabelAppendable}, "", promclient.DefaultRegisterer, ls)
	scrapeInterceptor := scrape.NewInterceptor("prometheus.scrape.test", livedebugging.NewLiveDebugging(), pipelineAppendable)

	return scrapeInterceptor, ls, func() { pipelineAppendable.Clear() }
}

func newRemoteWriteComponent(t testing.TB, logger log.Logger, ls *labelstore.Service, destination storage.Appendable) storage.Appendable {
	walDir := t.TempDir()

	walStorage, err := wal.NewStorage(logger, promclient.NewRegistry(), walDir)
	require.NoError(t, err)

	fanoutLogger := slog.New(
		logging.NewSlogGoKitHandler(
			log.With(logger, "subcomponent", "fanout"),
		),
	)

	store := storage.NewFanout(fanoutLogger, walStorage, testStorage{destination: destination})

	t.Cleanup(func() {
		store.Close()
		walStorage.Close()
	})

	return remotewrite.NewInterceptor("prometheus.remote_write.test", &atomic.Bool{}, livedebugging.NewLiveDebugging(), ls, store)
}

type testStorage struct {
	// Embed Queryable/ChunkQueryable for compatibility, but don't actually implement it.
	storage.Queryable
	storage.ChunkQueryable

	destination storage.Appendable
}

func (t testStorage) Appender(ctx context.Context) storage.Appender {
	return t.destination.Appender(ctx)
}

func (t testStorage) StartTime() (int64, error) {
	return 0, nil
}

func (t testStorage) Close() error {
	return nil
}

func newRelabelComponent(t testing.TB, logger log.Logger, forwardTo []storage.Appendable, ls *labelstore.Service) storage.Appendable {
	cfg := `forward_to = []
			rule {
				action       = "replace"
				target_label = "lbl"
				replacement  = "foo"
			}`
	var args relabel.Arguments
	require.NoError(t, syntax.Unmarshal([]byte(cfg), &args))
	args.ForwardTo = forwardTo

	tc, err := componenttest.NewControllerFromID(logger, "prometheus.relabel")
	require.NoError(t, err)
	go func() {
		err = tc.Run(componenttest.TestContext(t), args, func(opts component.Options) component.Options {
			inner := opts.GetServiceData
			opts.GetServiceData = func(name string) (interface{}, error) {
				if name == labelstore.ServiceName {
					return ls, nil
				}
				return inner(name)
			}
			return opts
		})
		require.NoError(t, err)
	}()
	require.NoError(t, tc.WaitRunning(5*time.Second))

	return tc.Exports().(relabel.Exports).Receiver
}

func sendMetric(t testing.TB, appender storage.Appender, labels labels.Labels, time int64, value float64) uint64 {
	ref, err := appender.Append(0, labels, time, value)
	require.NoError(t, err)
	require.NoError(t, appender.Commit())

	return uint64(ref)
}
