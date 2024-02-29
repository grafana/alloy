package remotewriteexporter

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/agent/internal/static/metrics/instance"
	"github.com/grafana/agent/internal/static/traces/contextkeys"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/model/metadata"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

const (
	callsMetric  = "traces_spanmetrics_calls_total"
	sumMetric    = "traces_spanmetrics_latency_sum"
	countMetric  = "traces_spanmetrics_latency_count"
	bucketMetric = "traces_spanmetrics_latency_bucket"
)

func TestRemoteWriteExporter_ConsumeMetrics(t *testing.T) {
	var (
		countValue     uint64  = 20
		sumValue       float64 = 100
		bucketCounts           = []uint64{1, 2, 3, 4, 5, 6}
		explicitBounds         = []float64{1, 2.5, 5, 7.5, 10}
		ts                     = time.Date(2020, 1, 2, 3, 4, 5, 6, time.UTC)
	)

	cfg := Config{
		ConstLabels:  nil,
		Namespace:    "traces",
		PromInstance: "traces",
	}
	exp, err := newRemoteWriteExporter(&cfg)
	require.NoError(t, err)

	manager := &mockManager{}
	ctx := context.WithValue(context.Background(), contextkeys.Metrics, manager)
	require.NoError(t, exp.Start(ctx, nil))

	metrics := pmetric.NewMetrics()
	ilm := metrics.ResourceMetrics().AppendEmpty().ScopeMetrics().AppendEmpty()
	ilm.Scope().SetName("spanmetrics")

	// Append sum metric
	sm := ilm.Metrics().AppendEmpty()
	sm.SetEmptySum()
	sm.SetName("spanmetrics_calls_total")
	sm.Sum().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	sdp := sm.Sum().DataPoints().AppendEmpty()
	sdp.SetTimestamp(pcommon.NewTimestampFromTime(ts.UTC()))
	sdp.SetDoubleValue(sumValue)

	// Append histogram
	hm := ilm.Metrics().AppendEmpty()
	hm.SetEmptyHistogram()
	hm.SetName("spanmetrics_latency")
	hm.Histogram().SetAggregationTemporality(pmetric.AggregationTemporalityCumulative)

	hdp := hm.Histogram().DataPoints().AppendEmpty()
	hdp.SetTimestamp(pcommon.NewTimestampFromTime(ts.UTC()))
	hdp.BucketCounts().FromRaw(bucketCounts)
	hdp.ExplicitBounds().FromRaw(explicitBounds)
	hdp.SetCount(countValue)
	hdp.SetSum(sumValue)

	err = exp.ConsumeMetrics(context.TODO(), metrics)
	require.NoError(t, err)

	time.Sleep(5 * time.Second)

	require.NoError(t, exp.Shutdown(context.TODO()))

	// Verify calls
	calls := manager.instance.GetAppended(callsMetric)
	require.Equal(t, len(calls), 1)
	require.Equal(t, calls[0].v, sumValue)
	require.Equal(t, calls[0].l, labels.Labels{{Name: nameLabelKey, Value: "traces_spanmetrics_calls_total"}})

	// Verify _sum
	sum := manager.instance.GetAppended(sumMetric)
	require.Equal(t, len(sum), 1)
	require.Equal(t, sum[0].v, sumValue)
	require.Equal(t, sum[0].l, labels.Labels{{Name: nameLabelKey, Value: "traces_spanmetrics_latency_" + sumSuffix}})

	// Check _count
	count := manager.instance.GetAppended(countMetric)
	require.Equal(t, len(count), 1)
	require.Equal(t, count[0].v, float64(countValue))
	require.Equal(t, count[0].l, labels.Labels{{Name: nameLabelKey, Value: "traces_spanmetrics_latency_" + countSuffix}})

	// Check _bucket
	buckets := manager.instance.GetAppended(bucketMetric)
	require.Equal(t, len(buckets), len(bucketCounts))
}

type mockManager struct {
	instance *mockInstance
}

func (m *mockManager) GetInstance(string) (instance.ManagedInstance, error) {
	if m.instance == nil {
		m.instance = &mockInstance{}
	}
	return m.instance, nil
}

func (m *mockManager) ListInstances() map[string]instance.ManagedInstance { return nil }

func (m *mockManager) ListConfigs() map[string]instance.Config { return nil }

func (m *mockManager) ApplyConfig(_ instance.Config) error { return nil }

func (m *mockManager) DeleteConfig(_ string) error { return nil }

func (m *mockManager) Stop() {}

type mockInstance struct {
	instance.NoOpInstance
	appender *mockAppender
}

func (m *mockInstance) Appender(_ context.Context) storage.Appender {
	if m.appender == nil {
		m.appender = &mockAppender{}
	}
	return m.appender
}

func (m *mockInstance) GetAppended(n string) []metric {
	return m.appender.GetAppended(n)
}

type metric struct {
	l labels.Labels
	t int64
	v float64
}

type mockAppender struct {
	appendedMetrics []metric
}

var _ storage.Appender = (*mockAppender)(nil)

func (a *mockAppender) GetAppended(n string) []metric {
	var ms []metric
	for _, m := range a.appendedMetrics {
		if n == m.l.Get(nameLabelKey) {
			ms = append(ms, m)
		}
	}
	return ms
}

func (a *mockAppender) Append(_ storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	a.appendedMetrics = append(a.appendedMetrics, metric{l: l, t: t, v: v})
	return 0, nil
}

func (a *mockAppender) Commit() error { return nil }

func (a *mockAppender) Rollback() error { return nil }

func (a *mockAppender) AppendExemplar(_ storage.SeriesRef, _ labels.Labels, _ exemplar.Exemplar) (storage.SeriesRef, error) {
	return 0, nil
}

func (a *mockAppender) UpdateMetadata(_ storage.SeriesRef, _ labels.Labels, _ metadata.Metadata) (storage.SeriesRef, error) {
	return 0, nil
}

func (a *mockAppender) AppendHistogram(_ storage.SeriesRef, _ labels.Labels, _ int64, _ *histogram.Histogram, _ *histogram.FloatHistogram) (storage.SeriesRef, error) {
	return 0, nil
}
