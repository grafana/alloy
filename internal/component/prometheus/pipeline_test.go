package prometheus_test

import (
	"context"
	"fmt"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-kit/log"
	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus"
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
	pipeline, ls, destination := newDefaultPipeline(t, util.TestLogger(t))

	// We need to use a future timestamp since remote_write will ignore any
	// sample which is earlier than the time when it started. Adding a minute
	// ensures that our samples will never get ignored.
	sampleTimestamp := time.Now().Add(time.Minute).UnixMilli()

	// Send metrics to our component. These will be written to the WAL and
	// subsequently written to our HTTP server.
	lset1 := labels.FromStrings("foo", "bar")
	ref1 := sendMetric(t, pipeline.Appender(t.Context()), lset1, sampleTimestamp, 12)
	lset2 := labels.FromStrings("fizz", "buzz")
	ref2 := sendMetric(t, pipeline.Appender(t.Context()), lset2, sampleTimestamp, 34)

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

// This test simulates a scrape -> relabel -> remote_write pipeline, without actually scraping
func TestRelabelPipeline(t *testing.T) {
	pipeline, ls, destination := newRelabelPipeline(t, util.TestLogger(t))

	// We need to use a future timestamp since remote_write will ignore any
	// sample which is earlier than the time when it started. Adding a minute
	// ensures that our samples will never get ignored.
	sampleTimestamp := time.Now().Add(time.Minute).UnixMilli()

	// Send metrics to our component. These will be written to the WAL and
	// subsequently written to our HTTP server.
	lset1 := labels.FromStrings("foo", "bar")
	ref1 := sendMetric(t, pipeline.Appender(t.Context()), lset1, sampleTimestamp, 12)
	lset2 := labels.FromStrings("fizz", "buzz")
	ref2 := sendMetric(t, pipeline.Appender(t.Context()), lset2, sampleTimestamp, 34)

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
	}, 1*time.Minute, 100*time.Millisecond, "timed out waiting for metrics to be written to destination")

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

func BenchmarkPipelines(b *testing.B) {
	tests := []struct {
		name            string
		pipelineBuilder func(t testing.TB, logger log.Logger) (storage.Appendable, labelstore.LabelStore, testappender.CollectingAppender)
	}{
		{"default", newDefaultPipeline},
		{"relabel", newRelabelPipeline},
	}

	numberOfMetrics := []int{2, 10, 100, 1000}

	for _, n := range numberOfMetrics {
		for _, tt := range tests {
			// Don't need care about the labelstore and destination for benchmarks
			pipeline, _, _ := tt.pipelineBuilder(b, log.NewNopLogger())
			b.Run(fmt.Sprintf("%s/%d-metrics", tt.name, n), func(b *testing.B) {
				b.ReportAllocs()
				b.ResetTimer()

				for b.Loop() {
					for i := 0; i < n; i++ {
						sendMetric(
							b,
							pipeline.Appender(b.Context()),
							labels.FromStrings(fmt.Sprintf("metric-%d", i), fmt.Sprintf("metric-%d", i)),
							time.Now().Add(time.Minute).UnixMilli(),
							float64(i),
						)
					}
				}
			})
		}
	}
}

func newDefaultPipeline(t testing.TB, logger log.Logger) (storage.Appendable, labelstore.LabelStore, testappender.CollectingAppender) {
	ls := labelstore.New(logger, promclient.DefaultRegisterer)
	rwAppendable, rwDestination := newRemoteWriteComponent(t, logger, ls)
	pipelineAppendable := prometheus.NewFanout([]storage.Appendable{rwAppendable}, promclient.DefaultRegisterer, ls)
	scrapeInterceptor := scrape.NewInterceptor("prometheus.scrape.test", ls, livedebugging.NewLiveDebugging(), pipelineAppendable)

	return scrapeInterceptor, ls, rwDestination
}

func newRelabelPipeline(t testing.TB, logger log.Logger) (storage.Appendable, labelstore.LabelStore, testappender.CollectingAppender) {
	ls := labelstore.New(logger, promclient.DefaultRegisterer)
	rwAppendable, rwDestination := newRemoteWriteComponent(t, logger, ls)
	relabelAppendable := newRelabelComponent(t, logger, []storage.Appendable{rwAppendable}, ls)
	pipelineAppendable := prometheus.NewFanout([]storage.Appendable{relabelAppendable}, promclient.DefaultRegisterer, ls)
	scrapeInterceptor := scrape.NewInterceptor("prometheus.scrape.test", ls, livedebugging.NewLiveDebugging(), pipelineAppendable)

	return scrapeInterceptor, ls, rwDestination
}

func newRemoteWriteComponent(t testing.TB, logger log.Logger, ls *labelstore.Service) (storage.Appendable, testappender.CollectingAppender) {
	walDir := t.TempDir()

	walStorage, err := wal.NewStorage(logger, promclient.NewRegistry(), walDir)
	require.NoError(t, err)

	fanoutLogger := slog.New(
		logging.NewSlogGoKitHandler(
			log.With(logger, "subcomponent", "fanout"),
		),
	)

	inMemoryAppendable := testappender.ConstantAppendable{Inner: testappender.NewCollectingAppender()}
	store := storage.NewFanout(fanoutLogger, walStorage, testStorage{inMemoryAppendable: inMemoryAppendable})

	return remotewrite.NewInterceptor("prometheus.remote_write.test", &atomic.Bool{}, livedebugging.NewLiveDebugging(), ls, store), inMemoryAppendable.Inner
}

type testStorage struct {
	// Embed Queryable/ChunkQueryable for compatibility, but don't actually implement it.
	storage.Queryable
	storage.ChunkQueryable

	inMemoryAppendable storage.Appendable
}

func (t testStorage) Appender(ctx context.Context) storage.Appender {
	return t.inMemoryAppendable.Appender(ctx)
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
