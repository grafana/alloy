package process

import (
	"context"
	"github.com/grafana/alloy/internal/service/labelstore"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/prometheus/model/exemplar"
	"github.com/prometheus/prometheus/model/metadata"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/grafana/alloy/internal/component"
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
		GetServiceData: func(name string) (interface{}, error) {
			return &fakestore{}, nil
		},
		Registerer: prometheus.NewRegistry(),
	}, Arguments{
		Wasm: bb,
		Config: map[string]string{
			"allowed_services": "cool,not_here",
		},
		PrometheusForwardTo: []storage.Appendable{ta},
	})
	require.NoError(t, err)
	ctx := context.Background()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	go c.Run(ctx)
	bulk := c.Appender(ctx)
	for i := 0; i < 1000; i++ {
		bulk.Append(0, labels.FromStrings("service", "cool"), time.Now().UnixMilli(), 1)

	}
	for i := 0; i < 10; i++ {
		bulk.Append(0, labels.FromStrings("service", "warm"), time.Now().UnixMilli(), 1)
	}

	err = bulk.Commit()
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

func (ta *testAppendable) Appender(ctx context.Context) storage.Appender {
	return ta
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

type fakestore struct{}

func (f fakestore) GetOrAddLink(componentID string, localRefID uint64, lbls labels.Labels) uint64 {
	return 0
}

func (f fakestore) GetOrAddGlobalRefID(l labels.Labels) uint64 {
	return 0
}

func (f fakestore) GetGlobalRefID(componentID string, localRefID uint64) uint64 {
	return 0
}

func (f fakestore) GetLocalRefID(componentID string, globalRefID uint64) uint64 {
	return 0
}

func (f fakestore) TrackStaleness(ids []labelstore.StalenessTracker) {
	return
}

func (f fakestore) CheckAndRemoveStaleMarkers() {
	return
}
