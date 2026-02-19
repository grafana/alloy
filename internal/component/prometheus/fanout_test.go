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

func (n noopAppender) AppendHistogramCTZeroSample(storage.SeriesRef, labels.Labels, int64, int64, *histogram.Histogram, *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return storage.SeriesRef(n.refCounter.Inc()), nil
}

func (n noopAppender) UpdateMetadata(storage.SeriesRef, labels.Labels, metadata.Metadata) (storage.SeriesRef, error) {
	return storage.SeriesRef(n.refCounter.Inc()), nil
}

func (n noopAppender) AppendCTZeroSample(storage.SeriesRef, labels.Labels, int64, int64) (storage.SeriesRef, error) {
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
