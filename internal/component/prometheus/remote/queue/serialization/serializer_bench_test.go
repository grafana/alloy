package serialization

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/grafana/alloy/internal/component/prometheus/remote/queue/types"
	"github.com/prometheus/prometheus/model/labels"
)

var lbls = labels.FromStrings("one", "two", "three", "four")

func BenchmarkAppender(b *testing.B) {
	// This should be 0 allocs
	b.ReportAllocs()
	logger := log.NewNopLogger()
	for i := 0; i < b.N; i++ {
		app := NewAppender(context.Background(), 1*time.Hour, &fakeSerializer{}, logger)
		for j := 0; j < 10_000; j++ {
			_, _ = app.Append(0, lbls, time.Now().Unix(), 1.1)
		}
		_ = app.Commit()
	}
}

func BenchmarkSerializer(b *testing.B) {
	// This should be around 200-300 allocs 7m ns/op
	series := getTimeSeries(b)
	b.ResetTimer()
	b.ReportAllocs()
	logger := log.NewNopLogger()
	for i := 0; i < b.N; i++ {
		serial, _ := NewSerializer(types.SerializerConfig{
			MaxSignalsInBatch: 1_000,
			FlushFrequency:    1 * time.Second,
		}, &fakeFileQueue{}, func(stats types.SerializerStats) {

		}, logger)
		serial.Start()
		for _, s := range series {
			_ = serial.SendSeries(context.Background(), s)
		}
		serial.Stop()
	}
}

func getTimeSeries(b *testing.B) []*types.TimeSeriesBinary {
	b.Helper()
	series := make([]*types.TimeSeriesBinary, 0)
	for j := 0; j < 10_000; j++ {
		timeseries := types.GetTimeSeriesFromPool()
		timeseries.TS = time.Now().Unix()
		timeseries.Value = rand.Float64()
		timeseries.Labels = getLabels()
		series = append(series, timeseries)
	}
	return series
}

func getLabels() labels.Labels {
	retLbls := make(labels.Labels, 0)
	for i := 0; i < rand.Intn(20); i++ {
		l := labels.Label{
			Name:  fmt.Sprintf("label_%d", i),
			Value: fmt.Sprintf("value_%d", i),
		}
		retLbls = append(retLbls, l)
	}
	return retLbls
}

var _ types.Serializer = (*fakeSerializer)(nil)

type fakeSerializer struct{}

func (f *fakeSerializer) UpdateConfig(ctx context.Context, cfg types.SerializerConfig) error {
	return nil
}

func (f *fakeSerializer) Start() {}

func (f *fakeSerializer) Stop() {}

func (f *fakeSerializer) SendSeries(ctx context.Context, data *types.TimeSeriesBinary) error {
	types.PutTimeSeriesIntoPool(data)
	return nil
}

func (f *fakeSerializer) SendMetadata(ctx context.Context, data *types.TimeSeriesBinary) error {
	types.PutTimeSeriesIntoPool(data)
	return nil
}

var _ types.FileStorage = (*fakeFileQueue)(nil)

type fakeFileQueue struct{}

func (f fakeFileQueue) Start() {

}

func (f fakeFileQueue) Stop() {

}

func (f fakeFileQueue) Store(ctx context.Context, meta map[string]string, value []byte) error {
	return nil
}
