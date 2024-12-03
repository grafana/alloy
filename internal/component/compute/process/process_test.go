package process

import (
	"context"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/metadata"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
	"github.com/grafana/alloy/internal/component/prometheus"
	"github.com/prometheus/prometheus/model/histogram"
	"github.com/prometheus/prometheus/model/labels"
	"github.com/prometheus/prometheus/storage"
	"github.com/stretchr/testify/require"
)

func TestProcess(t *testing.T) {
	bb, err := os.ReadFile(filepath.Join(".", "examples", "go", "restrict", "main.wasm"))
	require.NoError(t, err)
	ta := &testAppendable{ts: t}
	c, err := New(component.Options{
		OnStateChange: func(e component.Exports) {

		},
	}, Arguments{
		Wasm: bb,
		Config: map[string]string{
			"allowed_services": "cool,not_here",
		},
		PrometheusForwardTo: ta,
	})
	require.NoError(t, err)
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	go c.Run(ctx)
	bulk := c.Appender(ctx)
	metrics := make([]prometheus.PromMetric, 0)
	for i := 0; i < 1000; i++ {
		metrics = append(metrics, prometheus.PromMetric{
			Value:  1,
			TS:     1,
			Labels: labels.FromStrings("service", "cool"),
		})
	}
	for i := 0; i < 10; i++ {
		metrics = append(metrics, prometheus.PromMetric{
			Value:  1,
			TS:     1,
			Labels: labels.FromStrings("service", "warm"),
		})
	}

	err = bulk.Append(nil, metrics)
	require.NoError(t, err)
	// There should only be 1_000 since we dont want any warm services to make it through
	require.Eventually(t, func() bool {
		return ta.count.Load() == 1000
	}, 5*time.Second, 100*time.Millisecond)
}

type testAppendable struct {
	count atomic.Int32
	ts    *testing.T
}

func (ta *testAppendable) Commit() error {
	return nil
}

func (ta *testAppendable) Rollback() error {
	//TODO implement me
	panic("implement me")
}

func (ta *testAppendable) Append(ref storage.SeriesRef, l labels.Labels, t int64, v float64) (storage.SeriesRef, error) {
	require.True(ta.ts, l.Get("service") == "cool")
	ta.count.Add(1)
	return ref, nil
}

func (ta *testAppendable) AppendExemplar(ref storage.SeriesRef, l labels.Labels, e exemplar.Exemplar) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (ta *testAppendable) AppendHistogram(ref storage.SeriesRef, l labels.Labels, t int64, h *histogram.Histogram, fh *histogram.FloatHistogram) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (ta *testAppendable) UpdateMetadata(ref storage.SeriesRef, l labels.Labels, m metadata.Metadata) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (ta *testAppendable) AppendCTZeroSample(ref storage.SeriesRef, l labels.Labels, t, ct int64) (storage.SeriesRef, error) {
	//TODO implement me
	panic("implement me")
}

func (ta *testAppendable) Appender(_ context.Context) storage.Appender {
	return ta
}
