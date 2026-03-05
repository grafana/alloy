package prometheus_test

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/go-kit/log"
	promclient "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/grafana/alloy/internal/component/prometheus/remotewrite"
	"github.com/grafana/alloy/internal/service/labelstore"
)

func TestRollback(t *testing.T) {
	ls := labelstore.New(nil, promclient.DefaultRegisterer)
	fanout := prometheus.NewFanout([]storage.Appendable{prometheus.NewFanout(nil, "1", promclient.DefaultRegisterer, ls)}, "", promclient.DefaultRegisterer, ls)
	app := fanout.Appender(t.Context())
	err := app.Rollback()
	require.NoError(t, err)
}

func TestCommit(t *testing.T) {
	ls := labelstore.New(nil, promclient.DefaultRegisterer)
	fanout := prometheus.NewFanout([]storage.Appendable{prometheus.NewFanout(nil, "1", promclient.DefaultRegisterer, ls)}, "", promclient.DefaultRegisterer, ls)
	app := fanout.Appender(t.Context())
	err := app.Commit()
	require.NoError(t, err)
}

func TestNewFanoutIgnoresNilChildren(t *testing.T) {
	ls := labelstore.New(nil, promclient.DefaultRegisterer)
	fanout := prometheus.NewFanout([]storage.Appendable{nil, nil}, "", promclient.DefaultRegisterer, ls)
	app := fanout.Appender(t.Context())
	err := app.Commit()
	require.NoError(t, err)
}

func TestNewFanoutWithNilLabelStore(t *testing.T) {
	fanout := prometheus.NewFanout([]storage.Appendable{noopStore{}}, "", promclient.DefaultRegisterer, nil)
	app := fanout.Appender(t.Context())
	_, err := app.Append(0, labels.FromStrings("foo", "bar"), time.Now().UnixMilli(), 1.0)
	require.NoError(t, err)
	err = app.Commit()
	require.NoError(t, err)
}

type benchAppenderFlowsItem struct {
	series        []labels.Labels
	targetsCount  int
	useLabelStore bool
}

func (i benchAppenderFlowsItem) name() string {
	key := "seriesref"
	if i.useLabelStore {
		key = "labelstore"
	}

	return fmt.Sprintf("pipeline=%s/targets=%d/metrics=%d", key, i.targetsCount, len(i.series))
}

// go test -bench="BenchmarkAppenderFlows" . -run ^$ -benchmem -count 6 -benchtime 5s | tee benchmarks
// benchstat -row '.name /targets /metrics' -col '/pipeline' benchmarks
func BenchmarkAppenderFlows(b *testing.B) {
	labels := setupMetrics(2000)
	cases := []benchAppenderFlowsItem{
		{
			series:        labels,
			targetsCount:  1,
			useLabelStore: true,
		},
		{
			series:        labels,
			targetsCount:  2,
			useLabelStore: true,
		},
		{
			series:        labels,
			targetsCount:  1,
			useLabelStore: false,
		},
		{
			series:        labels,
			targetsCount:  2,
			useLabelStore: false,
		},
	}

	for _, c := range cases {
		now := time.Now().UnixMilli()
		ls := labelstore.New(log.NewNopLogger(), promclient.DefaultRegisterer, c.useLabelStore)

		children := make([]storage.Appendable, c.targetsCount)
		for i := range c.targetsCount {
			children[i] = remotewrite.NewInterceptor(strconv.Itoa(i), &atomic.Bool{}, noopDebugDataPublisher{}, ls, noopStore{})
		}
		fanout := prometheus.NewFanout(children, "fanout", promclient.DefaultRegisterer, ls)

		tname := c.name()
		b.Run(tname, func(b *testing.B) {
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				app := fanout.Appender(b.Context())
				for _, metric := range c.series {
					app.Append(0, metric, now, 1.0)
				}
				app.Commit()

				b.StopTimer()
				fanout.Clear()
				b.StartTimer()
			}
		})
	}
}

type noopAppender struct {
	refCounter atomic.Uint64
}

func (n noopAppender) Append(storage.SeriesRef, labels.Labels, int64, float64) (storage.SeriesRef, error) {
	return storage.SeriesRef(n.refCounter.Inc()), nil
}

func (n noopAppender) Commit() error {
	return nil
}

func (n noopAppender) Rollback() error {
	return nil
}

func (n noopAppender) SetOptions(*storage.AppendOptions) {
}

func (n noopAppender) AppendExemplar(storage.SeriesRef, labels.Labels, exemplar.Exemplar) (storage.SeriesRef, error) {
	return storage.SeriesRef(n.refCounter.Inc()), nil
}

func (n noopAppender) AppendHistogram(storage.SeriesRef, labels.Labels, int64, *histogram.Histogram, *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return storage.SeriesRef(n.refCounter.Inc()), nil
}

func (n noopAppender) AppendHistogramSTZeroSample(storage.SeriesRef, labels.Labels, int64, int64, *histogram.Histogram, *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return storage.SeriesRef(n.refCounter.Inc()), nil
}

func (n noopAppender) UpdateMetadata(storage.SeriesRef, labels.Labels, metadata.Metadata) (storage.SeriesRef, error) {
	return storage.SeriesRef(n.refCounter.Inc()), nil
}

func (n noopAppender) AppendSTZeroSample(storage.SeriesRef, labels.Labels, int64, int64) (storage.SeriesRef, error) {
	return storage.SeriesRef(n.refCounter.Inc()), nil
}

type noopStore struct {
	refCounter atomic.Uint64
}

func (n noopStore) Querier(int64, int64) (storage.Querier, error) {
	return nil, nil
}

func (n noopStore) ChunkQuerier(int64, int64) (storage.ChunkQuerier, error) {
	return nil, nil
}

func (n noopStore) Appender(context.Context) storage.Appender {
	return noopAppender(n)
}

func (n noopStore) StartTime() (int64, error) {
	return 0, nil
}

func (n noopStore) Close() error {
	return nil
}

// recordingAppender records the ref passed to each Append call.
type recordingAppender struct {
	nextRef    storage.SeriesRef
	appendRefs []storage.SeriesRef
}

func (r *recordingAppender) Append(ref storage.SeriesRef, _ labels.Labels, _ int64, _ float64) (storage.SeriesRef, error) {
	r.appendRefs = append(r.appendRefs, ref)
	if ref == 0 {
		r.nextRef++
		return r.nextRef, nil
	}
	return ref, nil
}
func (r *recordingAppender) Commit() error {
	return nil
}

func (r *recordingAppender) Rollback() error {
	return nil
}

func (r *recordingAppender) SetOptions(*storage.AppendOptions) {}
func (r *recordingAppender) AppendExemplar(ref storage.SeriesRef, _ labels.Labels, _ exemplar.Exemplar) (storage.SeriesRef, error) {
	return ref, nil
}
func (r *recordingAppender) AppendHistogram(ref storage.SeriesRef, _ labels.Labels, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return ref, nil
}
func (r *recordingAppender) AppendHistogramSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return ref, nil
}
func (r *recordingAppender) UpdateMetadata(ref storage.SeriesRef, _ labels.Labels, _ metadata.Metadata) (storage.SeriesRef, error) {
	return ref, nil
}
func (r *recordingAppender) AppendSTZeroSample(ref storage.SeriesRef, _ labels.Labels, _, _ int64) (storage.SeriesRef, error) {
	return ref, nil
}

// recordingStore is a storage.Appendable backed by a recordingAppender.
type recordingStore struct{ appender *recordingAppender }

func newRecordingStore() *recordingStore {
	return &recordingStore{appender: &recordingAppender{nextRef: 5000}}
}
func (s *recordingStore) Appender(context.Context) storage.Appender { return s.appender }

// TestFanout_SeriesRefMappingToPassthroughTransition verifies that when the fanout
// transitions from seriesRefMapping (2 children) to passthrough (1 child) via
// UpdateChildren, store-issued unique refs cached by callers are zeroed before
// forwarding so the child allocates a fresh ref.
func TestFanout_SeriesRefMappingToPassthroughTransition(t *testing.T) {
	child1 := newRecordingStore()
	child2 := newRecordingStore()
	fanout := prometheus.NewFanout([]storage.Appendable{child1, child2}, "test", promclient.NewRegistry(), nil)

	// Phase 1 (2 children → seriesRefMapping): store issues unique ref 1 for lblsA.
	app1 := fanout.Appender(t.Context())
	lblsA := labels.FromStrings("job", "seriesA")
	uniqueRef, err := app1.Append(0, lblsA, 1, 1.0)
	require.NoError(t, err)
	require.NoError(t, app1.Commit())
	require.Equal(t, storage.SeriesRef(1), uniqueRef)

	// Transition to 1 child.
	walChild := newRecordingStore()
	fanout.UpdateChildren([]storage.Appendable{walChild})

	// Phase 2 (1 child → passthrough): caller re-sends the store-issued unique ref.
	// The passthrough must zero it so the child allocates a fresh ref.
	app2 := fanout.Appender(t.Context())
	_, err = app2.Append(uniqueRef, lblsA, 2, 2.0)
	require.NoError(t, err)
	require.NoError(t, app2.Commit())

	// The child must have been called with ref=0, not the store-issued unique ref.
	require.Equal(t, storage.SeriesRef(0), walChild.appender.appendRefs[0],
		"child must be called with ref=0, not the store-issued unique ref")
}

// TestFanout_PassthroughToSeriesRefMappingTransition verifies that when the fanout
// transitions from passthrough (1 child) to seriesRefMapping (2 children) via
// UpdateChildren, a cached raw child ref that collides numerically with a new
// store-issued unique ref for a different series is handled correctly via label
// hash guards.
func TestFanout_PassthroughToSeriesRefMappingTransition(t *testing.T) {
	walChild := newRecordingStore()
	fanout := prometheus.NewFanout([]storage.Appendable{walChild}, "test", promclient.NewRegistry(), nil)

	// Phase 1 (1 child → passthrough): child returns raw ref 5001 for lblsB.
	app1 := fanout.Appender(t.Context())
	lblsB := labels.FromStrings("job", "seriesB")
	passthroughRef, err := app1.Append(0, lblsB, 1, 1.0)
	require.NoError(t, err)
	require.NoError(t, app1.Commit())
	// passthrough returns the raw child ref directly.
	require.Equal(t, storage.SeriesRef(5001), passthroughRef)

	// Transition to 2 children.
	child1 := newRecordingStore()
	child2 := newRecordingStore()
	fanout.UpdateChildren([]storage.Appendable{child1, child2})

	// Phase 2 (2 children → seriesRefMapping): store issues unique ref 1 for lblsA.
	app2 := fanout.Appender(t.Context())
	lblsA := labels.FromStrings("job", "seriesA")
	_, err = app2.Append(0, lblsA, 2, 2.0)
	require.NoError(t, err)

	// Caller re-sends passthroughRef for lblsB. The label hash guard must prevent
	// it from matching lblsA's mapping and force a fresh append for lblsB.
	refB, err := app2.Append(passthroughRef, lblsB, 3, 3.0)
	require.NoError(t, err)
	require.NoError(t, app2.Commit())

	// lblsB must have its own mapping distinct from lblsA's.
	require.NotEqual(t, storage.SeriesRef(1), refB,
		"seriesB must not reuse seriesA's store-issued unique ref")

	// Both children must have been called with passthroughRef for lblsB, not seriesA's child refs.
	require.Equal(t, passthroughRef, child1.appender.appendRefs[1],
		"child1 must be called with passthrough ref for seriesB")
	require.Equal(t, passthroughRef, child2.appender.appendRefs[1],
		"child2 must be called with passthrough ref for seriesB")
}
